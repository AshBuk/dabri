// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"fmt"
	"time"

	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/hotkeys/manager"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// HotkeyManager defines the interface for hotkey managers
type HotkeyManager interface {
	Start() error
	Stop()
	RegisterToggle(toggle func() error)
	RegisterHotkeyAction(action string, callback manager.HotkeyAction)
	ReloadConfig(newConfig adapters.HotkeyConfig) error
	CaptureOnce(timeout time.Duration) (string, error)
	SupportsCaptureOnce() bool
}

// Bridges hotkey events to application handlers
type HotkeyService struct {
	logger        logger.Logger
	hotkeyManager HotkeyManager
}

// Create a new service instance
func NewHotkeyService(
	logger logger.Logger,
	hotkeyManager HotkeyManager,
) *HotkeyService {
	return &HotkeyService{
		logger:        logger,
		hotkeyManager: hotkeyManager,
	}
}

// Connect application handlers to low-level hotkey events
func (hs *HotkeyService) SetupHotkeyCallbacks(
	toggleRecording func() error,
	showConfig func() error,
	resetToDefaults func() error,
) error {
	if hs.hotkeyManager == nil {
		return fmt.Errorf("hotkey manager not available")
	}
	hs.logger.Info("Setting up hotkey callbacks...")
	// Register the recording toggle (owns the start/stop decision)
	hs.hotkeyManager.RegisterToggle(toggleRecording)
	// Register additional hotkey actions
	hs.hotkeyManager.RegisterHotkeyAction("show_config", showConfig)
	hs.hotkeyManager.RegisterHotkeyAction("reset_to_defaults", resetToDefaults)
	hs.logger.Info("Hotkey callbacks configured successfully")
	return nil
}

// Activate hotkey capture for the current session
func (hs *HotkeyService) RegisterHotkeys() error {
	if hs.hotkeyManager == nil {
		return fmt.Errorf("hotkey manager not available")
	}
	hs.logger.Info("Registering hotkeys...")
	if err := hs.hotkeyManager.Start(); err != nil {
		// Provide helpful guidance for AppImage users when evdev/dbus fails
		hs.logger.Warning("Failed to register hotkeys: %v", err)
		if hs.logger != nil {
			hs.logger.Info("TIP: On Wayland/AppImage, D-Bus GlobalShortcuts is preferred. Ensure the portal is available.")
			hs.logger.Info("If using evdev, ensure user is in 'input' group and re-login.")
		}
		return err
	}
	return nil
}

// Release hotkey capture to prevent conflicts
func (hs *HotkeyService) UnregisterHotkeys() error {
	if hs.hotkeyManager == nil {
		return nil
	}
	hs.logger.Info("Unregistering hotkeys...")
	hs.hotkeyManager.Stop()
	return nil
}

// Clean shutdown to prevent hanging hotkey listeners
func (hs *HotkeyService) Shutdown() error {
	// Unregister hotkeys
	if err := hs.UnregisterHotkeys(); err != nil {
		hs.logger.Error("Error unregistering hotkeys: %v", err)
		return err
	}
	hs.logger.Info("HotkeyService shutdown complete")
	return nil
}

// Apply new hotkey bindings without restarting the service
func (hs *HotkeyService) ReloadFromConfig(toggleRecording func() error, configProvider func() adapters.HotkeyConfig) error {
	if hs.hotkeyManager == nil {
		return fmt.Errorf("hotkey manager not available")
	}
	cfg := configProvider()
	// ensure the toggle is set (in case of provider swap)
	hs.hotkeyManager.RegisterToggle(toggleRecording)
	return hs.hotkeyManager.ReloadConfig(cfg)
}

// Capture single keypress for hotkey rebinding workflow
func (hs *HotkeyService) CaptureOnce(timeoutMs int) (string, error) {
	if hs.hotkeyManager == nil {
		return "", fmt.Errorf("hotkey manager not available")
	}
	if timeoutMs <= 0 {
		timeoutMs = 3000
	}
	return hs.hotkeyManager.CaptureOnce(time.Duration(timeoutMs) * time.Millisecond)
}

// Check if interactive hotkey binding is available
func (hs *HotkeyService) SupportsCaptureOnce() bool {
	if hs.hotkeyManager == nil {
		return false
	}
	return hs.hotkeyManager.SupportsCaptureOnce()
}
