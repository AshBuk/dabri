// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AshBuk/go-wlportal/typing"

	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/output/interfaces"
)

var portalPasteClipboardRestoreDelay = 750 * time.Millisecond

// PortalRemoteDesktopAvailable reports whether the RemoteDesktop portal exposes
// keyboard injection. This is the sandbox-clean typing path on GNOME/KDE; the
// wlroots/Hyprland portal does not implement it.
func PortalRemoteDesktopAvailable() bool {
	return typing.Available()
}

type portalKeyboard interface {
	Type(text string) error
	KeyCombo(keycodes ...typing.Keycode) error
}

type clipboardReader interface {
	ReadClipboard() (string, bool)
}

// types text through the RemoteDesktop portal via the go-wlportal/typing adapter
type PortalOutputter struct {
	kbd       portalKeyboard
	clipboard interfaces.Outputter
}

// NewPortalOutputter creates a portal-based type outputter (session is opened lazily).
func NewPortalOutputter(clipboard interfaces.Outputter) (interfaces.Outputter, error) {
	kbd, err := typing.NewKeyboard(
		typing.WithRestoreTokenPath(portalTokenPath()),
		typing.WithAppID(constants.AppID),
	)
	if err != nil {
		return nil, err
	}
	return &PortalOutputter{kbd: kbd, clipboard: clipboard}, nil
}

// TypeToActiveWindow injects text as keyboard input into the focused window.
// The portal can only type characters in the active keyboard layout. For
// non-ASCII text, keep active-window mode by copying the text and sending the
// paste shortcut through the same RemoteDesktop portal session.
func (o *PortalOutputter) TypeToActiveWindow(text string) error {
	if isNonASCII(text) {
		if o.clipboard == nil {
			return fmt.Errorf("portal paste requires a clipboard outputter")
		}
		previousClipboard, restoreClipboard := o.readClipboard()
		if err := o.clipboard.CopyToClipboard(text); err != nil {
			return err
		}
		if err := o.kbd.KeyCombo(typing.KeycodeLeftShift, typing.KeycodeInsert); err != nil {
			return err
		}
		if restoreClipboard {
			o.restoreClipboardLater(previousClipboard)
		}
		return nil
	}
	return o.kbd.Type(text)
}

// CopyToClipboard delegates to the configured clipboard outputter.
func (o *PortalOutputter) CopyToClipboard(text string) error {
	if o.clipboard == nil {
		return fmt.Errorf("copying to clipboard not supported by portal outputter")
	}
	return o.clipboard.CopyToClipboard(text)
}

// GetToolNames reports the active typing backend
func (o *PortalOutputter) GetToolNames() (clipboardTool, typeTool string) {
	if o.clipboard != nil {
		clipboardTool, _ = o.clipboard.GetToolNames()
	}
	return clipboardTool, "portal"
}

func (o *PortalOutputter) readClipboard() (string, bool) {
	reader, ok := o.clipboard.(clipboardReader)
	if !ok {
		return "", false
	}
	return reader.ReadClipboard()
}

func (o *PortalOutputter) restoreClipboardLater(text string) {
	time.AfterFunc(portalPasteClipboardRestoreDelay, func() {
		_ = o.clipboard.CopyToClipboard(text)
	})
}

// portalTokenPath returns where the portal permission token is persisted so the
// consent dialog appears only once across restarts.
func portalTokenPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "dabri", "portal-remotedesktop.token")
}
