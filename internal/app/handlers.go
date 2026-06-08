// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package app

import (
	"fmt"

	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
)

// Hotkey Handlers - Adapter layer between HotkeyService and Business Services
// Event-driven hotkey callbacks (async, fire-and-forget)
//
// Architecture Flow:
//   User presses hotkey
//       ↓
//   Provider (evdev/dbus/dummy)
//       ↓
//   HotkeyManager (hotkeys/manager)
//       ↓
//   HotkeyService (internal/services/hotkey_service.go)
//       ↓
//   Handler methods (this file) ← adapter/facade layer
//       ↓
//   Business Services (AudioService/ConfigService/UIService)

// handleStartRecording Adapter - delegates start recording hotkey to AudioService
func (a *App) handleStartRecording() error {
	if a.Services == nil || a.Services.Audio == nil {
		return fmt.Errorf("audio service not available")
	}
	return a.Services.Audio.HandleStartRecording()
}

// handleStopRecordingAndTranscribe Adapter - delegates stop recording hotkey to AudioService
func (a *App) handleStopRecordingAndTranscribe() error {
	if a.Services == nil || a.Services.Audio == nil {
		return fmt.Errorf("audio service not available")
	}
	return a.Services.Audio.HandleStopRecording()
}

// handleToggleRecording starts or stops recording based on the recorder's real
// state, so every trigger (hotkey, window, tray) shares one source of truth and
// they never desync.
func (a *App) handleToggleRecording() error {
	if a.Services == nil || a.Services.Audio == nil {
		return fmt.Errorf("audio service not available")
	}
	if a.Services.Audio.IsRecording() {
		return a.handleStopRecordingAndTranscribe()
	}
	return a.handleStartRecording()
}

// handleShowConfig Adapter - delegates show config hotkey to UIService
func (a *App) handleShowConfig() error {
	if a.Services == nil || a.Services.UI == nil {
		return fmt.Errorf("UI service not available")
	}
	return a.Services.UI.ShowConfigFile()
}

// handleResetToDefaults coordinates config reset + hotkey reload
// Multi-service coordination: ConfigService.ResetToDefaults() → HotkeyService.ReloadFromConfig()
// Ensures new default hotkey bindings are applied immediately without restart
func (a *App) handleResetToDefaults() error {
	if a.Services == nil || a.Services.Config == nil {
		return fmt.Errorf("config service not available")
	}
	if err := a.Services.Config.ResetToDefaults(); err != nil {
		return err
	}
	if a.Services.Hotkeys != nil {
		cfgProvider := func() adapters.HotkeyConfig {
			if cfg := a.Services.Config.GetConfig(); cfg != nil {
				return adapters.NewConfigAdapter(cfg.Hotkeys.StartRecording, cfg.Hotkeys.Provider).
					WithAdditionalHotkeys(cfg.Hotkeys.ShowConfig, cfg.Hotkeys.ResetToDefaults)
			}
			return adapters.NewConfigAdapter("", "auto")
		}
		if err := a.Services.Hotkeys.ReloadFromConfig(
			a.handleToggleRecording,
			cfgProvider,
		); err != nil {
			return err
		}
	}
	return nil
}
