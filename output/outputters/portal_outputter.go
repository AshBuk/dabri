// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/output/interfaces"
)

// RemoteDesktop portal — the sandbox-clean typing path. Input is injected by the
// compositor itself, so the Flatpak security-context that blocks wtype does not
// apply. Available on GNOME/KDE; the wlroots/Hyprland portal does not implement it.
const (
	portalDest      = "org.freedesktop.portal.Desktop"
	portalPath      = "/org/freedesktop/portal/desktop"
	portalRemote    = "org.freedesktop.portal.RemoteDesktop"
	portalRequest   = "org.freedesktop.portal.Request"
	rdKeyboard      = uint32(1) // RemoteDesktop DeviceType bitmask: KEYBOARD
	rdPersistToken  = uint32(2) // persist_mode: keep permission across restarts
	keyReleased     = uint32(0)
	keyPressed      = uint32(1)
	portalCallReply = 60 * time.Second
)

var portalTokenSeq uint64

// PortalRemoteDesktopAvailable reports whether the RemoteDesktop portal exposes
// keyboard injection on the current session.
func PortalRemoteDesktopAvailable() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var v dbus.Variant
	if err := conn.Object(portalDest, portalPath).
		Call("org.freedesktop.DBus.Properties.Get", 0, portalRemote, "AvailableDeviceTypes").
		Store(&v); err != nil {
		return false
	}
	types, ok := v.Value().(uint32)
	return ok && types&rdKeyboard != 0
}

// PortalOutputter types text through the RemoteDesktop portal
type PortalOutputter struct {
	config    *config.Config
	tokenPath string

	mu      sync.Mutex
	conn    *dbus.Conn
	signals chan *dbus.Signal
	session dbus.ObjectPath
}

// NewPortalOutputter creates a portal-based type outputter (session is opened lazily)
func NewPortalOutputter(cfg *config.Config) (interfaces.Outputter, error) {
	if !PortalRemoteDesktopAvailable() {
		return nil, fmt.Errorf("RemoteDesktop portal not available")
	}
	return &PortalOutputter{config: cfg, tokenPath: portalTokenPath()}, nil
}

// TypeToActiveWindow injects text as keyboard input into the focused window
func (p *PortalOutputter) TypeToActiveWindow(text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureSession(); err != nil {
		return fmt.Errorf("portal session: %w", err)
	}
	obj := p.conn.Object(portalDest, portalPath)
	opts := map[string]dbus.Variant{}
	for _, r := range text {
		keysym := int32(runeToKeysym(r))
		for _, state := range [2]uint32{keyPressed, keyReleased} {
			if call := obj.Call(portalRemote+".NotifyKeyboardKeysym", 0, p.session, opts, keysym, state); call.Err != nil {
				return fmt.Errorf("notify keysym: %w", call.Err)
			}
		}
	}
	return nil
}

// CopyToClipboard is not supported by this outputter
func (p *PortalOutputter) CopyToClipboard(text string) error {
	return fmt.Errorf("copying to clipboard not supported by portal outputter")
}

// GetToolNames reports the active typing backend
func (p *PortalOutputter) GetToolNames() (clipboardTool, typeTool string) {
	return "", "portal"
}

// ensureSession lazily creates and starts the RemoteDesktop keyboard session.
// A dedicated connection is kept open because the session lives with it.
func (p *PortalOutputter) ensureSession() error {
	if p.session != "" {
		return nil
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("connect session bus: %w", err)
	}
	p.conn = conn
	p.signals = make(chan *dbus.Signal, 8)
	conn.Signal(p.signals)

	created, err := p.request("CreateSession", func(token string) []interface{} {
		return []interface{}{map[string]dbus.Variant{
			"handle_token":         dbus.MakeVariant(token),
			"session_handle_token": dbus.MakeVariant(token),
		}}
	})
	if err != nil {
		return err
	}
	handle, _ := created["session_handle"].Value().(string)
	if handle == "" {
		return fmt.Errorf("empty session handle")
	}
	session := dbus.ObjectPath(handle)

	if _, err := p.request("SelectDevices", func(token string) []interface{} {
		opts := map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(token),
			"types":        dbus.MakeVariant(rdKeyboard),
			"persist_mode": dbus.MakeVariant(rdPersistToken),
		}
		if t := p.loadToken(); t != "" {
			opts["restore_token"] = dbus.MakeVariant(t)
		}
		return []interface{}{session, opts}
	}); err != nil {
		return err
	}

	started, err := p.request("Start", func(token string) []interface{} {
		return []interface{}{session, "", map[string]dbus.Variant{
			"handle_token": dbus.MakeVariant(token),
		}}
	})
	if err != nil {
		return err
	}
	if t, ok := started["restore_token"].Value().(string); ok {
		p.saveToken(t)
	}
	p.session = session
	return nil
}

// request invokes a portal method and waits for its asynchronous Response signal
func (p *PortalOutputter) request(method string, build func(token string) []interface{}) (map[string]dbus.Variant, error) {
	token := fmt.Sprintf("dabri%d", atomic.AddUint64(&portalTokenSeq, 1))
	sender := strings.ReplaceAll(strings.TrimPrefix(p.conn.Names()[0], ":"), ".", "_")
	reqPath := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)

	match := []dbus.MatchOption{
		dbus.WithMatchObjectPath(reqPath),
		dbus.WithMatchInterface(portalRequest),
		dbus.WithMatchMember("Response"),
	}
	if err := p.conn.AddMatchSignal(match...); err != nil {
		return nil, err
	}
	defer func() { _ = p.conn.RemoveMatchSignal(match...) }()

	var handle dbus.ObjectPath
	if err := p.conn.Object(portalDest, portalPath).
		Call(portalRemote+"."+method, 0, build(token)...).Store(&handle); err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}

	timeout := time.After(portalCallReply)
	for {
		select {
		case sig := <-p.signals:
			if sig == nil || sig.Path != reqPath || sig.Name != portalRequest+".Response" {
				continue
			}
			var code uint32
			var results map[string]dbus.Variant
			if err := dbus.Store(sig.Body, &code, &results); err != nil {
				return nil, fmt.Errorf("%s response: %w", method, err)
			}
			if code != 0 {
				return nil, fmt.Errorf("%s rejected (code %d)", method, code)
			}
			return results, nil
		case <-timeout:
			return nil, fmt.Errorf("%s timed out", method)
		}
	}
}

// runeToKeysym maps a rune to an X11 keysym. Latin-1 maps 1:1; other code points
// use the Unicode keysym range so non-ASCII transcripts still type.
func runeToKeysym(r rune) rune {
	switch r {
	case '\n':
		return 0xff0d // Return
	case '\t':
		return 0xff09 // Tab
	case '\b':
		return 0xff08 // BackSpace
	}
	if r <= 0x7e || (r >= 0xa0 && r <= 0xff) {
		return r
	}
	return 0x01000000 + r
}

func portalTokenPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "dabri", "portal-remotedesktop.token")
}

func (p *PortalOutputter) loadToken() string {
	if p.tokenPath == "" {
		return ""
	}
	b, _ := os.ReadFile(p.tokenPath)
	return strings.TrimSpace(string(b))
}

func (p *PortalOutputter) saveToken(token string) {
	if p.tokenPath == "" || token == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p.tokenPath), 0o700)
	_ = os.WriteFile(p.tokenPath, []byte(token), 0o600)
}
