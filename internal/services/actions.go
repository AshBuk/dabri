// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
)

// User actions shared by every surface (hotkeys, tray, window, CLI over IPC),
// so each scenario has exactly one implementation.
//
//   User presses hotkey / clicks tray or window / runs a CLI command
//       ↓
//   Provider (evdev/dbus/dummy) → HotkeyManager → HotkeyService,
//   tray/window callbacks (wireUI), or IPC handlers (internal/app/ipc.go)
//       ↓
//   ServiceContainer actions (this file)
//       ↓
//   Business Services (AudioService/ConfigService/UIService/IOService)

// ToggleRecording starts or stops recording based on the recorder's real
// state, so every trigger shares one source of truth and they never desync.
func (c *ServiceContainer) ToggleRecording() error {
	if c.Audio.IsRecording() {
		return c.Audio.HandleStopRecording()
	}
	return c.Audio.HandleStartRecording()
}

// RecordingHotkey maps the recording hotkey's press and release to an action
// for the active mode: toggle on press, or record while held (push-to-talk).
func (c *ServiceContainer) RecordingHotkey(pressed bool) error {
	if !c.Hotkeys.PushToTalk() {
		if !pressed {
			return nil
		}
		return c.ToggleRecording()
	}
	if pressed {
		return c.Audio.HandleStartRecording()
	}
	// The recording may already be stopped by the tray, window or time limit
	if err := c.Audio.HandleStopRecording(); !errors.Is(err, ErrNoRecordingInProgress) {
		return err
	}
	return nil
}

// ResetToDefaults restores default settings and applies the default hotkeys
// immediately, without a restart.
func (c *ServiceContainer) ResetToDefaults() error {
	if err := c.Config.ResetToDefaults(); err != nil {
		return err
	}
	return c.reloadHotkeys()
}

// SwitchModel hot-swaps the whisper engine and persists the choice. If saving
// fails, the engine rolls back so runtime state matches the config.
func (c *ServiceContainer) SwitchModel(ctx context.Context, modelID string) error {
	previous := c.Config.GetConfig().General.Model
	if err := c.Audio.SwitchModel(ctx, modelID); err != nil {
		return err
	}
	if err := c.Config.UpdateModel(modelID); err != nil {
		if previous != "" {
			if rbErr := c.Audio.SwitchModel(ctx, previous); rbErr != nil {
				c.Logger.Error("Failed to rollback model after config save error: %v", rbErr)
			}
		}
		return fmt.Errorf("failed to persist model setting: %w", err)
	}
	c.updateUISettings()
	return nil
}

// selectModel is SwitchModel for the tray and window: it reports the outcome
// as a desktop notification, staying silent on user-initiated cancellation.
func (c *ServiceContainer) selectModel(ctx context.Context, modelID string) error {
	if err := c.SwitchModel(ctx, modelID); err != nil {
		if ctx.Err() == nil {
			c.UI.ShowNotification("Model Switch Failed", err.Error())
		}
		return err
	}
	c.UI.ShowNotification("Model Switch", fmt.Sprintf("Switched to %s", modelID))
	return nil
}

func (c *ServiceContainer) setLanguage(language string) error {
	if err := c.Config.UpdateLanguage(language); err != nil {
		return err
	}
	c.updateUISettings()
	return nil
}

func (c *ServiceContainer) setOutputMode(mode string) error {
	if err := c.Config.UpdateOutputMode(mode); err != nil {
		return err
	}
	if err := c.IO.SetOutputMethod(mode); err != nil {
		return err
	}
	c.updateUISettings()
	return nil
}

// setHotkeyMode switches the recording hotkey between toggle and push-to-talk.
// The mode picks the hotkey provider, so hotkeys reload.
func (c *ServiceContainer) setHotkeyMode(mode string) error {
	if err := c.Config.UpdateHotkeyMode(mode); err != nil {
		return err
	}
	return c.reloadHotkeys()
}

// StartHotkeys registers the hotkeys at startup and reports the outcome.
func (c *ServiceContainer) StartHotkeys() error {
	err := c.Hotkeys.RegisterHotkeys()
	c.hotkeysApplied(err)
	return err
}

// hotkeysApplied refreshes the UI for the provider that actually runs, since a
// fallback may switch it, and tells the user when hotkeys are unavailable or
// push-to-talk toggles because evdev input access is missing.
func (c *ServiceContainer) hotkeysApplied(err error) {
	c.updateUISettings()
	switch {
	case err != nil:
		c.UI.ShowNotification("Hotkeys unavailable", "Record from the tray instead. See docs/Desktop_Environment_Support.md")
	case c.Config.GetConfig().Hotkeys.Mode == config.HotkeyModePTT && !c.Hotkeys.PushToTalk():
		c.UI.ShowNotification("Push-to-talk unavailable", "It needs input device access, so the hotkey toggles recording. See docs/Desktop_Environment_Support.md")
	}
}

func (c *ServiceContainer) setRecordingMethod(method string) error {
	if err := c.Config.UpdateRecordingMethod(method); err != nil {
		return err
	}
	c.Audio.ClearSession()
	c.updateUISettings()
	return nil
}

func (c *ServiceContainer) toggleNotifications() error {
	if err := c.Config.ToggleWorkflowNotifications(); err != nil {
		return err
	}
	status := "disabled"
	if c.Config.GetConfig().Notifications.EnableWorkflowNotifications {
		status = "enabled"
	}
	c.UI.ShowNotification("Workflow Notifications", "Now "+status)
	return nil
}

// rebindHotkey captures the next key combo and binds it to the action.
func (c *ServiceContainer) rebindHotkey(action string) error {
	c.UI.ShowNotification("Rebind Hotkey", "Press new hotkey… (Esc to cancel)")
	combo, err := c.Hotkeys.CaptureOnce(3000)
	if err != nil || strings.TrimSpace(combo) == "" {
		c.UI.ShowNotification("Rebind Hotkey", "Cancelled or timeout")
		return nil
	}
	if err := c.Config.UpdateHotkey(action, combo); err != nil {
		return err
	}
	if err := c.reloadHotkeys(); err != nil {
		return err
	}
	c.UI.ShowNotification("Hotkey Updated", fmt.Sprintf("%s -> %s", action, combo))
	return nil
}

func (c *ServiceContainer) reloadHotkeys() error {
	err := c.Hotkeys.ReloadFromConfig(c.RecordingHotkey, hotkeyConfig(c.Config.GetConfig()))
	c.hotkeysApplied(err)
	return err
}

func (c *ServiceContainer) updateUISettings() {
	c.UI.UpdateSettings(c.Config.GetConfig())
}

// hotkeyConfig adapts the app config to the hotkey manager's view of it.
func hotkeyConfig(cfg *config.Config) adapters.HotkeyConfig {
	return adapters.NewConfigAdapter(cfg.Hotkeys.StartRecording, cfg.Hotkeys.Provider).
		WithAdditionalHotkeys(cfg.Hotkeys.ShowConfig, cfg.Hotkeys.ResetToDefaults).
		WithMode(cfg.Hotkeys.Mode)
}
