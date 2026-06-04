//go:build linux

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/AshBuk/dabri/v2/hotkeys/utils"
	"github.com/AshBuk/dabri/v2/internal/logger"
	dbus "github.com/godbus/dbus/v5"
)

// Implements KeyboardEventProvider using the D-Bus GlobalShortcuts portal
type DbusKeyboardProvider struct {
	callbacks     map[string]func() error
	conn          *dbus.Conn
	sessionHandle string
	isListening   bool
	mutex         sync.Mutex
	logger        logger.Logger
	wg            sync.WaitGroup // Tracks listener goroutine
}

// Create a new D-Bus keyboard provider
func NewDbusKeyboardProvider(logger logger.Logger) *DbusKeyboardProvider {
	return &DbusKeyboardProvider{
		callbacks:   make(map[string]func() error),
		isListening: false,
		logger:      logger,
	}
}

// Check if the D-Bus GlobalShortcuts portal is available
func (p *DbusKeyboardProvider) IsSupported() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		p.logger.Warning("D-Bus session bus not available: %v", err)
		return false
	}
	defer func() {
		if err := conn.Close(); err != nil {
			p.logger.Error("Failed to close D-Bus connection: %v", err)
		}
	}()
	// Probe with a short timeout — portal may be activatable but not yet running,
	// which causes the default D-Bus timeout (25s) to block app startup.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	call := obj.CallWithContext(ctx, "org.freedesktop.DBus.Introspectable.Introspect", 0)
	if call.Err != nil {
		p.logger.Warning("D-Bus portal not available: %v", call.Err)
		return false
	}

	var introspectData string
	if err := call.Store(&introspectData); err != nil {
		p.logger.Error("Failed to get introspection data: %v", err)
		return false
	}
	if len(introspectData) > 0 && containsGlobalShortcuts(introspectData) {
		p.logger.Info("D-Bus portal GlobalShortcuts detected")
		return true
	}
	p.logger.Info("D-Bus portal GlobalShortcuts not available")
	return false
}

// Check if the introspection data contains the GlobalShortcuts interface
func containsGlobalShortcuts(data string) bool {
	return strings.Contains(data, "GlobalShortcuts")
}

// Start listening for D-Bus hotkey events
func (p *DbusKeyboardProvider) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isListening {
		return fmt.Errorf("D-Bus keyboard provider already started")
	}

	var err error
	p.conn, err = dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus (D-Bus unavailable): %w", err)
	}
	// Register hotkeys via the GlobalShortcuts portal
	if err := p.registerHotkeys(); err != nil {
		if closeErr := p.conn.Close(); closeErr != nil {
			p.logger.Error("Failed to close D-Bus connection: %v", closeErr)
		}
		p.logger.Error("DBus GlobalShortcuts binding failed: %v", err)
		p.logger.Info("Hint: In AppImage/sandboxed environments, global shortcuts may require user consent")
		return fmt.Errorf("failed to register hotkeys (GlobalShortcuts portal unavailable): %w", err)
	}
	p.isListening = true
	p.logger.Info("D-Bus hotkey provider started successfully")
	return nil
}

// Stop the D-Bus hotkey listener
func (p *DbusKeyboardProvider) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.isListening {
		return
	}
	// Close D-Bus connection to unblock signal channel
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			p.logger.Error("Failed to close D-Bus connection: %v", err)
		}
		p.conn = nil
	}

	p.isListening = false
	// Wait for listener goroutine to exit
	p.wg.Wait()

	p.logger.Info("D-Bus hotkey provider stopped")
}

// Register a hotkey and its callback
func (p *DbusKeyboardProvider) RegisterHotkey(hotkey string, callback func() error) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, exists := p.callbacks[hotkey]; exists {
		return fmt.Errorf("hotkey %s already registered", hotkey)
	}

	p.callbacks[hotkey] = callback
	p.logger.Info("D-Bus hotkey registered: %s", hotkey)
	return nil
}

// SupportsCaptureOnce returns false as D-Bus GlobalShortcuts portal doesn't support one-shot capture
func (p *DbusKeyboardProvider) SupportsCaptureOnce() bool { return false }

// Return an error as this provider does not support capture-once functionality
func (p *DbusKeyboardProvider) CaptureOnce(timeout time.Duration) (string, error) {
	return "", fmt.Errorf("captureOnce not supported in dbus provider")
}

// Convert a hotkey string to a desktop-portal accelerator string
// e.g., "ctrl+shift+a" -> "<Ctrl><Shift>a"
func convertHotkeyToAccelerator(hotkey string) string {
	combo := utils.ParseHotkey(hotkey)
	var prefix strings.Builder
	for _, m := range combo.Modifiers {
		switch strings.ToLower(m) {
		case "ctrl", "leftctrl", "rightctrl":
			prefix.WriteString("<Ctrl>")
		case "alt", "leftalt":
			prefix.WriteString("<Alt>")
		case "rightalt", "altgr":
			prefix.WriteString("<AltGr>")
		case "shift", "leftshift", "rightshift":
			prefix.WriteString("<Shift>")
		case "super", "meta", "win", "leftmeta", "rightmeta":
			prefix.WriteString("<Super>")
		}
	}

	// Map special key names to the standard accelerator format
	key := combo.Key
	switch strings.ToLower(key) {
	case "comma":
		key = "comma"
	case "period":
		key = "period"
	case "space":
		key = "space"
	case "enter", "return":
		key = "Return"
	case "tab":
		key = "Tab"
	case "escape", "esc":
		key = "Escape"
	case "backspace":
		key = "BackSpace"
	case "delete", "del":
		key = "Delete"
	}
	return prefix.String() + key
}

// Register all hotkeys using the GlobalShortcuts portal
func (p *DbusKeyboardProvider) registerHotkeys() error {
	obj := p.conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")

	// Generate unique token for this request
	handleToken := fmt.Sprintf("dabri_%d", time.Now().UnixNano())
	sessionHandleToken := fmt.Sprintf("dabri_session_%d", time.Now().UnixNano())

	// Build expected request path from sender name and token
	senderName := strings.ReplaceAll(p.conn.Names()[0], ".", "_")
	senderName = strings.TrimPrefix(senderName, ":")
	expectedRequestPath := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/portal/desktop/request/%s/%s", senderName, handleToken))

	p.logger.Info("DBus: Expected request path: %s", expectedRequestPath)

	// Subscribe to Response signal BEFORE calling CreateSession to avoid race condition
	signalChan := make(chan *dbus.Signal, 1)
	p.conn.Signal(signalChan)
	defer p.conn.RemoveSignal(signalChan)

	rule := fmt.Sprintf("type='signal',interface='org.freedesktop.portal.Request',member='Response',path='%s'", expectedRequestPath)
	if err := p.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule).Err; err != nil {
		return fmt.Errorf("failed to add match rule: %w", err)
	}

	// Step 1: Create a session using Request/Response pattern
	sessionOptions := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(handleToken),
		"session_handle_token": dbus.MakeVariant(sessionHandleToken),
	}
	call := obj.Call("org.freedesktop.portal.GlobalShortcuts.CreateSession", 0, sessionOptions)
	if call.Err != nil {
		return fmt.Errorf("failed to create GlobalShortcuts session: %w", call.Err)
	}

	// Wait for the Response signal to get the session handle
	sessionHandle, err := p.waitForSessionResponseOnChannel(signalChan, expectedRequestPath)
	if err != nil {
		return fmt.Errorf("failed to get session handle: %w", err)
	}
	p.sessionHandle = sessionHandle
	// Step 2: Prepare shortcuts for binding (include accelerator)
	// Format according to GlobalShortcuts portal spec: a(sa{sv})
	shortcuts := make([]struct {
		ID   string
		Data map[string]dbus.Variant
	}, 0, len(p.callbacks))
	for hotkey := range p.callbacks {
		accel := convertHotkeyToAccelerator(hotkey)
		p.logger.Info("DBus: Converting hotkey '%s' to accelerator '%s'", hotkey, accel)

		shortcutData := map[string]dbus.Variant{
			"description":       dbus.MakeVariant(fmt.Sprintf("Dabri hotkey: %s", hotkey)),
			"preferred_trigger": dbus.MakeVariant(accel),
		}
		shortcuts = append(shortcuts, struct {
			ID   string
			Data map[string]dbus.Variant
		}{
			ID:   hotkey,
			Data: shortcutData,
		})
	}
	// Step 3: Bind shortcuts to the session
	bindOptions := map[string]dbus.Variant{}
	p.logger.Info("DBus: Binding %d shortcuts to session %s", len(shortcuts), sessionHandle)

	call = obj.Call("org.freedesktop.portal.GlobalShortcuts.BindShortcuts", 0,
		dbus.ObjectPath(sessionHandle), shortcuts, "", bindOptions)
	if call.Err != nil {
		return fmt.Errorf("failed to bind shortcuts: %w", call.Err)
	}

	p.logger.Info("DBus: Successfully bound shortcuts")
	// Step 4: Start listening for shortcut activations in tracked goroutine
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.listenForShortcuts()
	}()
	return nil
}

// Wait for the Response signal on a pre-subscribed channel
func (p *DbusKeyboardProvider) waitForSessionResponseOnChannel(signalChan chan *dbus.Signal, expectedPath dbus.ObjectPath) (string, error) {
	timeout := time.After(5 * time.Second)
	for {
		select {
		case sig := <-signalChan:
			p.logger.Info("DBus: Received signal %s on path %s", sig.Name, sig.Path)
			if sig.Name == "org.freedesktop.portal.Request.Response" && sig.Path == expectedPath {
				if len(sig.Body) >= 2 {
					// Body[0] is response code, Body[1] is results
					// Response codes: 0=success, 1=user cancelled, 2=other error/rejected
					responseCode, ok := sig.Body[0].(uint32)
					if !ok || responseCode != 0 {
						if responseCode == 1 {
							return "", fmt.Errorf("user cancelled the permission dialog")
						}
						if responseCode == 2 {
							return "", fmt.Errorf("permission denied - GlobalShortcuts requires app to be installed or user approval")
						}
						return "", fmt.Errorf("CreateSession request failed with code %v", responseCode)
					}

					results, ok := sig.Body[1].(map[string]dbus.Variant)
					if !ok {
						return "", fmt.Errorf("invalid results format in Response signal")
					}

					sessionHandleVariant, exists := results["session_handle"]
					if !exists {
						return "", fmt.Errorf("session_handle not found in Response results")
					}

					sessionHandle, ok := sessionHandleVariant.Value().(string)
					if !ok {
						return "", fmt.Errorf("invalid session_handle type in Response results")
					}

					return sessionHandle, nil
				}
			}
			// Continue waiting for the expected signal
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for CreateSession response")
		}
	}
}

// Listen for shortcut activation signals from the GlobalShortcuts portal
func (p *DbusKeyboardProvider) listenForShortcuts() {
	rule := "type='signal',interface='org.freedesktop.portal.GlobalShortcuts',member='Activated',path='/org/freedesktop/portal/desktop'"
	p.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)
	// Listen for signals
	c := make(chan *dbus.Signal, 10)
	p.conn.Signal(c)

	for sig := range c {
		if sig.Name == "org.freedesktop.portal.GlobalShortcuts.Activated" {
			if len(sig.Body) >= 2 {
				// Body[0] is session handle, Body[1] is shortcut_id
				if sessionHandle, ok := sig.Body[0].(dbus.ObjectPath); ok && string(sessionHandle) == p.sessionHandle {
					if shortcutId, ok := sig.Body[1].(string); ok {
						if callback, exists := p.callbacks[shortcutId]; exists {
							p.logger.Info("Hotkey activated: %s", shortcutId)
							if err := callback(); err != nil {
								p.logger.Error("Error executing hotkey callback: %v", err)
							}
						}
					}
				}
			}
		}
	}
}
