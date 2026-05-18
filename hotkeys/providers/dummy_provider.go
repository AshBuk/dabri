// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"fmt"
	"time"

	"github.com/AshBuk/dabri/v2/internal/logger"
)

// Implements a KeyboardEventProvider with no actual functionality
// Use as a fallback when no other providers are available
type DummyKeyboardProvider struct {
	callbacks   map[string]func() error
	isListening bool
	logger      logger.Logger
}

// Create a new DummyKeyboardProvider
func NewDummyKeyboardProvider(logger logger.Logger) *DummyKeyboardProvider {
	return &DummyKeyboardProvider{
		callbacks:   make(map[string]func() error),
		isListening: false,
		logger:      logger,
	}
}

// Return true as the dummy provider is always supported as a fallback
func (p *DummyKeyboardProvider) IsSupported() bool {
	return true
}

// Log helpful instructions and warnings instead of starting a listener
func (p *DummyKeyboardProvider) Start() error {
	if p.isListening {
		return fmt.Errorf("dummy keyboard provider already started")
	}

	p.isListening = true
	p.logger.Warning("Using dummy keyboard provider; hotkeys will not be functional")
	p.logger.Info("")
	p.logger.Info("To enable hotkeys, try one of these solutions:")
	p.logger.Info("")
	p.logger.Info("🔧 Modern Desktop Environments (GNOME/KDE):")
	p.logger.Info("   - Ensure D-Bus session is running")
	p.logger.Info("   - Check if 'dbus-daemon --session' is active")
	p.logger.Info("")
	p.logger.Info("🔧 Other Desktop Environments (XFCE/i3/sway):")
	p.logger.Info("   - Add your user to the 'input' group: sudo usermod -a -G input $USER")
	p.logger.Info("   - Then logout and login again")
	p.logger.Info("   - Or run the application with sudo (not recommended)")
	p.logger.Info("")
	p.logger.Info("🔧 Alternative Solutions:")
	p.logger.Info("   - Use system hotkey tools like 'sxhkd' or 'xbindkeys'")
	p.logger.Info("   - Configure DE-specific keyboard shortcuts")
	p.logger.Info("   - Use the WebSocket interface for remote control")
	p.logger.Info("")

	return nil
}

// Update the state to indicate the provider is stopped
func (p *DummyKeyboardProvider) Stop() {
	p.isListening = false
}

// Store the callback but never call it
func (p *DummyKeyboardProvider) RegisterHotkey(hotkey string, callback func() error) error {
	p.logger.Info("Registered hotkey: %s (but it will not function with dummy provider)", hotkey)
	p.callbacks[hotkey] = callback
	return nil
}

// SupportsCaptureOnce returns false as dummy provider has no real functionality
func (p *DummyKeyboardProvider) SupportsCaptureOnce() bool { return false }

// Return an error as this functionality is not supported
func (p *DummyKeyboardProvider) CaptureOnce(timeout time.Duration) (string, error) {
	return "", fmt.Errorf("captureOnce not supported in dummy provider")
}
