// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// ConfigService manages persistent configuration state and validation.
// Every change publishes a new copy, so a *config.Config returned by
// GetConfig is never mutated and is safe to read from any goroutine.
type ConfigService struct {
	logger     logger.Logger
	current    atomic.Pointer[config.Config]
	mu         sync.Mutex // Serializes updates: copy, save, publish
	configFile string
	uiService  UIServiceInterface
}

// NewConfigService creates a new ConfigService instance
func NewConfigService(
	logger logger.Logger,
	config *config.Config,
	configFile string,
) *ConfigService {
	cs := &ConfigService{
		logger:     logger,
		configFile: configFile,
		uiService:  nil, // Will be set later via dependency injection
	}
	cs.current.Store(config)
	return cs
}

// SetUIService wires UI service to enable user feedback after config changes
func (cs *ConfigService) SetUIService(uiService UIServiceInterface) {
	cs.uiService = uiService
}

// SaveConfig persists current configuration state to disk
func (cs *ConfigService) SaveConfig() error {
	return cs.update(func(*config.Config) {})
}

// update applies fn to a copy of the current config, saves the copy and
// publishes it only when the save succeeds.
func (cs *ConfigService) update(fn func(*config.Config)) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	next := *cs.current.Load()
	fn(&next)
	cs.logger.Info("Saving configuration to: %s", cs.configFile)
	if cs.configFile == "" {
		return fmt.Errorf("no config file path set")
	}
	if err := config.SaveConfig(cs.configFile, &next); err != nil {
		return err
	}
	cs.current.Store(&next)
	return nil
}

// ResetToDefaults restores factory defaults and notifies the user of successful reset
func (cs *ConfigService) ResetToDefaults() error {
	cs.logger.Info("Resetting configuration to defaults...")
	if err := cs.update(config.SetDefaultConfig); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}
	cs.logger.Info("Configuration reset to defaults successfully")

	// Show success notification via UI service and refresh UI
	if cs.uiService != nil {
		cs.uiService.ShowNotification(constants.NotifyConfigReset, constants.NotifyConfigResetSuccess)
		cs.uiService.UpdateSettings(cs.GetConfig())
	}
	return nil
}

// GetConfig provides read-only access to current configuration
func (cs *ConfigService) GetConfig() *config.Config {
	return cs.current.Load()
}

// UpdateModel changes the model setting; a failed save keeps the old value
func (cs *ConfigService) UpdateModel(modelID string) error {
	cs.logger.Info("Updating whisper model to: %s", modelID)
	if cs.GetConfig().General.Model == modelID {
		return nil
	}
	if err := cs.update(func(c *config.Config) { c.General.Model = modelID }); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// UpdateLanguage changes whisper language setting; a failed save keeps the old value
func (cs *ConfigService) UpdateLanguage(language string) error {
	cs.logger.Info("Updating language to: %s", language)
	if cs.GetConfig().General.Language == language {
		return nil
	}
	if err := cs.update(func(c *config.Config) { c.General.Language = language }); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// UpdateOutputMode switches between clipboard and typing output; a failed save keeps the old value
func (cs *ConfigService) UpdateOutputMode(mode string) error {
	cs.logger.Info("Updating output mode to: %s", mode)
	if cs.GetConfig().Output.DefaultMode == mode {
		return nil
	}
	if err := cs.update(func(c *config.Config) { c.Output.DefaultMode = mode }); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// UpdateHotkeyMode switches the recording hotkey between toggle and push-to-talk
func (cs *ConfigService) UpdateHotkeyMode(mode string) error {
	cs.logger.Info("Updating hotkey mode to: %s", mode)
	if mode != config.HotkeyModeToggle && mode != config.HotkeyModePTT {
		return fmt.Errorf("unknown hotkey mode: %s", mode)
	}
	if cs.GetConfig().Hotkeys.Mode == mode {
		return nil
	}
	if err := cs.update(func(c *config.Config) { c.Hotkeys.Mode = mode }); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// ToggleWorkflowNotifications enables or disables desktop notifications for workflow events
func (cs *ConfigService) ToggleWorkflowNotifications() error {
	cs.logger.Info("Toggling workflow notifications")
	return cs.update(func(c *config.Config) {
		c.Notifications.EnableWorkflowNotifications = !c.Notifications.EnableWorkflowNotifications
	})
}

// UpdateRecordingMethod switches audio backend (arecord/ffmpeg) with validation and persistence
func (cs *ConfigService) UpdateRecordingMethod(method string) error {
	cs.logger.Info("Updating recording method to: %s", method)
	if method != "arecord" && method != "ffmpeg" {
		return fmt.Errorf("invalid recording method: %s", method)
	}
	if cs.GetConfig().Audio.RecordingMethod == method {
		return nil
	}
	if err := cs.update(func(c *config.Config) { c.Audio.RecordingMethod = method }); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// UpdateHotkey rebinds hotkey combinations; a failed save keeps the old bindings
func (cs *ConfigService) UpdateHotkey(action, combo string) error {
	var set func(c *config.Config)
	switch action {
	case "start_recording", "stop_recording":
		set = func(c *config.Config) { c.Hotkeys.StartRecording, c.Hotkeys.StopRecording = combo, combo }
	case "show_config":
		set = func(c *config.Config) { c.Hotkeys.ShowConfig = combo }
	case "reset_to_defaults":
		set = func(c *config.Config) { c.Hotkeys.ResetToDefaults = combo }
	default:
		return fmt.Errorf("unknown hotkey action: %s", action)
	}
	if err := cs.update(set); err != nil {
		return fmt.Errorf("failed to save hotkey: %w", err)
	}

	// Refresh UI display
	if cs.uiService != nil {
		cs.uiService.UpdateSettings(cs.GetConfig())
	}
	return nil
}

// Shutdown ensures final configuration state is saved before termination
func (cs *ConfigService) Shutdown() error {
	cs.logger.Info("ConfigService shutdown complete")
	return nil
}
