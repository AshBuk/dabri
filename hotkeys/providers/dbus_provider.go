//go:build linux

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/AshBuk/go-wlportal/shortcuts"

	"github.com/AshBuk/dabri/v2/hotkeys/utils"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// Implements KeyboardEventProvider using the GlobalShortcuts portal through the
// go-wlportal/shortcuts adapter.
type DbusKeyboardProvider struct {
	callbacks   map[string]func() error // keyed by stable action id
	triggers    map[string]string       // action id -> desired key combo
	session     *shortcuts.Session
	isListening bool
	mutex       sync.Mutex
	logger      logger.Logger
	wg          sync.WaitGroup // Tracks listener goroutine
}

// Create a new D-Bus keyboard provider
func NewDbusKeyboardProvider(logger logger.Logger) *DbusKeyboardProvider {
	return &DbusKeyboardProvider{
		callbacks: make(map[string]func() error),
		triggers:  make(map[string]string),
		logger:    logger,
	}
}

// actionDescriptions maps stable action ids to human-readable text shown in the
// compositor's shortcut UI. Unknown ids fall back to the id itself.
var actionDescriptions = map[string]string{
	"toggle_recording":  "Start/stop recording",
	"show_config":       "Open configuration",
	"reset_to_defaults": "Reset settings to defaults",
}

func describeAction(id string) string {
	if d, ok := actionDescriptions[id]; ok {
		return d
	}
	return id
}

// Check if the D-Bus GlobalShortcuts portal is available
func (p *DbusKeyboardProvider) IsSupported() bool {
	if shortcuts.Available() {
		p.logger.Info("D-Bus portal GlobalShortcuts detected")
		return true
	}
	p.logger.Info("D-Bus portal GlobalShortcuts not available")
	return false
}

// Register an action and its callback. Binding is deferred until Start.
func (p *DbusKeyboardProvider) RegisterHotkey(id, hotkey string, callback func() error) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, exists := p.callbacks[id]; exists {
		return fmt.Errorf("action %s already registered", id)
	}

	p.callbacks[id] = callback
	p.triggers[id] = hotkey
	p.logger.Info("D-Bus action registered: %s (%s)", id, hotkey)
	return nil
}

// Start binds all registered hotkeys via the GlobalShortcuts portal and listens
// for their activations.
func (p *DbusKeyboardProvider) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.isListening {
		return fmt.Errorf("D-Bus keyboard provider already started")
	}

	// The portal ID is the stable action id (echoed back on activation), so it
	// stays constant across rebinds; the key is only the preferred_trigger hint.
	list := make([]shortcuts.Shortcut, 0, len(p.triggers))
	for id, hotkey := range p.triggers {
		accel := convertHotkeyToAccelerator(hotkey)
		list = append(list, shortcuts.Shortcut{
			ID:               id,
			Description:      describeAction(id),
			PreferredTrigger: accel,
		})
	}

	session, err := shortcuts.New(list, shortcuts.WithAppID(constants.AppID))
	if err != nil {
		p.logger.Error("DBus GlobalShortcuts binding failed: %v", err)
		p.logger.Info("Hint: In AppImage/sandboxed environments, global shortcuts may require user consent")
		return fmt.Errorf("failed to register hotkeys (GlobalShortcuts portal unavailable): %w", err)
	}
	p.session = session
	p.isListening = true

	// Pass the channel explicitly so the goroutine never reads p.session, which
	// Stop clears under the lock.
	p.wg.Add(1)
	go p.listen(session.Events())

	p.logger.Info("D-Bus hotkey provider started successfully")
	return nil
}

// Stop ends the portal session and waits for the listener goroutine to exit.
func (p *DbusKeyboardProvider) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.isListening {
		return
	}
	// Closing the session closes its Events channel, which ends the listener.
	if p.session != nil {
		if err := p.session.Close(); err != nil {
			p.logger.Error("Failed to close GlobalShortcuts session: %v", err)
		}
		p.session = nil
	}

	p.isListening = false
	// Wait for listener goroutine to exit
	p.wg.Wait()

	p.logger.Info("D-Bus hotkey provider stopped")
}

// SupportsCaptureOnce returns false as D-Bus GlobalShortcuts portal doesn't support one-shot capture
func (p *DbusKeyboardProvider) SupportsCaptureOnce() bool { return false }

// Return an error as this provider does not support capture-once functionality
func (p *DbusKeyboardProvider) CaptureOnce(timeout time.Duration) (string, error) {
	return "", fmt.Errorf("captureOnce not supported in dbus provider")
}

// listen dispatches callbacks for activated shortcuts until the session closes.
// The callbacks map is read-only after Start, so no lock is taken here.
func (p *DbusKeyboardProvider) listen(events <-chan shortcuts.Event) {
	defer p.wg.Done()
	for e := range events {
		if !e.Pressed {
			continue
		}
		callback, exists := p.callbacks[e.ID]
		if !exists {
			continue
		}
		p.logger.Info("Hotkey activated: %s", e.ID)
		if err := callback(); err != nil {
			p.logger.Error("Error executing hotkey callback: %v", err)
		}
	}
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
