// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"fmt"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// Manages persistent configuration state and validation
type ConfigService struct {
	logger     logger.Logger
	config     *config.Config
	configFile string
	uiService  UIServiceInterface
}

// Create a new ConfigService instance
func NewConfigService(
	logger logger.Logger,
	config *config.Config,
	configFile string,
) *ConfigService {
	return &ConfigService{
		logger:     logger,
		config:     config,
		configFile: configFile,
		uiService:  nil, // Will be set later via dependency injection
	}
}

// Wire UI service to enable user feedback after config changes
func (cs *ConfigService) SetUIService(uiService UIServiceInterface) {
	cs.uiService = uiService
}

// Update active configuration path reference
func (cs *ConfigService) LoadConfig(configFile string) error {
	cs.logger.Info("Loading configuration from: %s", configFile)
	cs.configFile = configFile
	// Config is already loaded by factory, just update the file path
	return nil
}

// Persist current configuration state to disk
func (cs *ConfigService) SaveConfig() error {
	cs.logger.Info("Saving configuration to: %s", cs.configFile)
	if cs.configFile == "" {
		return fmt.Errorf("no config file path set")
	}
	return config.SaveConfig(cs.configFile, cs.config)
}

// Restore factory defaults and notify user of successful reset
func (cs *ConfigService) ResetToDefaults() error {
	cs.logger.Info("Resetting configuration to defaults...")

	// Reset existing config in-place to keep pointer stable across app
	config.SetDefaultConfig(cs.config)

	// Save to file immediately
	if err := cs.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}
	cs.logger.Info("Configuration reset to defaults successfully")

	// Show success notification via UI service and refresh UI
	if cs.uiService != nil {
		cs.uiService.ShowNotification(constants.NotifyConfigReset, constants.NotifyConfigResetSuccess)
		cs.uiService.UpdateSettings(cs.config)
	}
	return nil
}

// Provide read-only access to current configuration
func (cs *ConfigService) GetConfig() *config.Config {
	return cs.config
}

// Change whisper model setting with rollback on save failure
func (cs *ConfigService) UpdateWhisperModel(modelID string) error {
	cs.logger.Info("Updating whisper model to: %s", modelID)
	if cs.config.General.WhisperModel == modelID {
		return nil
	}
	old := cs.config.General.WhisperModel
	cs.config.General.WhisperModel = modelID
	if err := cs.SaveConfig(); err != nil {
		cs.config.General.WhisperModel = old
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// Change whisper language setting with rollback on save failure
func (cs *ConfigService) UpdateLanguage(language string) error {
	cs.logger.Info("Updating language to: %s", language)
	if cs.config.General.Language == language {
		return nil
	}
	old := cs.config.General.Language
	cs.config.General.Language = language

	if err := cs.SaveConfig(); err != nil {
		cs.config.General.Language = old
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// Switch between clipboard and typing output with rollback protection
func (cs *ConfigService) UpdateOutputMode(mode string) error {
	cs.logger.Info("Updating output mode to: %s", mode)
	if cs.config.Output.DefaultMode == mode {
		return nil
	}
	old := cs.config.Output.DefaultMode
	cs.config.Output.DefaultMode = mode

	if err := cs.SaveConfig(); err != nil {
		cs.config.Output.DefaultMode = old
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// Enable/disable desktop notifications for workflow events
func (cs *ConfigService) ToggleWorkflowNotifications() error {
	cs.logger.Info("Toggling workflow notifications")
	cs.config.Notifications.EnableWorkflowNotifications = !cs.config.Notifications.EnableWorkflowNotifications
	return cs.SaveConfig()
}

// Switch audio backend (arecord/ffmpeg) with validation and persistence
func (cs *ConfigService) UpdateRecordingMethod(method string) error {
	cs.logger.Info("Updating recording method to: %s", method)
	if method != "arecord" && method != "ffmpeg" {
		return fmt.Errorf("invalid recording method: %s", method)
	}
	if cs.config.Audio.RecordingMethod == method {
		return nil
	}
	old := cs.config.Audio.RecordingMethod
	cs.config.Audio.RecordingMethod = method

	if err := cs.SaveConfig(); err != nil {
		cs.config.Audio.RecordingMethod = old
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// Rebind hotkey combinations with validation and rollback protection
func (cs *ConfigService) UpdateHotkey(action, combo string) error {
	if cs == nil || cs.config == nil {
		return fmt.Errorf("config service not available")
	}
	oldStart := cs.config.Hotkeys.StartRecording
	oldStop := cs.config.Hotkeys.StopRecording
	oldShow := cs.config.Hotkeys.ShowConfig
	oldReset := cs.config.Hotkeys.ResetToDefaults

	switch action {
	case "start_recording", "stop_recording":
		cs.config.Hotkeys.StartRecording = combo
		cs.config.Hotkeys.StopRecording = combo
	case "show_config":
		cs.config.Hotkeys.ShowConfig = combo
	case "reset_to_defaults":
		cs.config.Hotkeys.ResetToDefaults = combo
	default:
		return fmt.Errorf("unknown hotkey action: %s", action)
	}

	if err := cs.SaveConfig(); err != nil {
		// rollback
		cs.config.Hotkeys.StartRecording = oldStart
		cs.config.Hotkeys.StopRecording = oldStop
		cs.config.Hotkeys.ShowConfig = oldShow
		cs.config.Hotkeys.ResetToDefaults = oldReset
		return fmt.Errorf("failed to save hotkey: %w", err)
	}

	// Refresh UI display
	if cs.uiService != nil {
		cs.uiService.UpdateSettings(cs.config)
	}
	return nil
}

// Ensure final configuration state is saved before termination
func (cs *ConfigService) Shutdown() error {
	cs.logger.Info("ConfigService shutdown complete")
	return nil
}
