// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"context"
	"errors"
	"time"

	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/ui/window"
	"github.com/AshBuk/dabri/v2/output/outputters"
)

// AudioServiceInterface defines the contract for audio-related operations
type AudioServiceInterface interface {
	// HandleStartRecording begins audio capture
	HandleStartRecording() error
	// HandleStopRecording stops audio capture and triggers transcription
	HandleStopRecording() error
	// IsRecording returns whether audio capture is active
	IsRecording() bool

	// ClearSession resets the current recording session
	ClearSession()

	// SwitchModel loads a different model
	SwitchModel(ctx context.Context, modelID string) error
	// DeleteModel removes a downloaded model
	DeleteModel(modelID string) error

	// GetLastTranscript returns the most recent transcription result
	GetLastTranscript() string

	// Shutdown releases audio resources
	Shutdown() error
}

// UIServiceInterface defines the contract for user interface operations
type UIServiceInterface interface {
	// SetRecordingState updates the tray icon to reflect recording status
	SetRecordingState(isRecording bool)
	// ShowNotification displays a desktop notification
	ShowNotification(title, message string)
	// UpdateSettings refreshes tray menu to reflect config changes
	UpdateSettings(config *config.Config)

	// SetError displays an error notification
	SetError(message string)
	// SetSuccess displays a success notification
	SetSuccess(message string)

	// ShowConfigFile opens the config file in the system editor
	ShowConfigFile() error
	// ShowAboutPage opens the about page in the browser
	ShowAboutPage() error

	// Shutdown releases UI resources
	Shutdown() error
}

// IOServiceInterface defines the contract for input/output operations
type IOServiceInterface interface {
	// OutputText sends transcribed text to the configured output target
	OutputText(text string) error
	// SetOutputMethod switches the output method (clipboard/typing)
	SetOutputMethod(method string) error

	// BeginTranscription signals that a transcription is in progress
	BeginTranscription()
	// CompleteTranscription delivers the transcription result
	CompleteTranscription(result string)
	// WaitForTranscription blocks until a transcription completes or times out
	WaitForTranscription(timeout time.Duration) (string, error)

	// GetOutputToolNames returns the names of the clipboard and typing tools
	GetOutputToolNames() (clipboardTool, typeTool string)

	// StartWebSocketServer starts the WebSocket server for remote control
	StartWebSocketServer() error

	// Shutdown releases IO resources
	Shutdown() error
}

// ConfigServiceInterface defines the contract for configuration operations
type ConfigServiceInterface interface {
	// ResetToDefaults restores all settings to their default values
	ResetToDefaults() error
	// GetConfig returns the current configuration
	GetConfig() *config.Config

	// UpdateLanguage changes the transcription language
	UpdateLanguage(language string) error
	// UpdateModel changes the active model
	UpdateModel(modelID string) error
	// ToggleWorkflowNotifications toggles workflow notification setting
	ToggleWorkflowNotifications() error
	// UpdateRecordingMethod changes the audio recording method
	UpdateRecordingMethod(method string) error
	// UpdateOutputMode changes the text output mode
	UpdateOutputMode(mode string) error
	// UpdateHotkeyMode switches the recording hotkey between toggle and push-to-talk
	UpdateHotkeyMode(mode string) error
	// UpdateHotkey changes the key binding for the given action
	UpdateHotkey(action, combo string) error

	// Shutdown releases config resources
	Shutdown() error
}

// HotkeyServiceInterface defines the contract for hotkey operations
type HotkeyServiceInterface interface {
	// SetupHotkeyCallbacks connects application handlers to hotkey events
	SetupHotkeyCallbacks(
		recording func(pressed bool) error,
		showConfig func() error,
		reloadConfig func() error,
	) error

	// RegisterHotkeys activates hotkey capture for the current session
	RegisterHotkeys() error

	// ReloadFromConfig applies new hotkey bindings without restarting
	ReloadFromConfig(recording func(pressed bool) error, cfg adapters.HotkeyConfig) error

	// CaptureOnce captures a single keypress for hotkey rebinding
	CaptureOnce(timeoutMs int) (string, error)

	// SupportsCaptureOnce reports whether interactive hotkey binding is available
	SupportsCaptureOnce() bool

	// PushToTalk reports whether the recording hotkey records while held
	PushToTalk() bool

	// Shutdown releases hotkey resources
	Shutdown() error
}

// ServiceContainer holds all service interfaces
type ServiceContainer struct {
	Logger          logger.Logger
	Audio           AudioServiceInterface
	UI              UIServiceInterface
	IO              IOServiceInterface
	Config          ConfigServiceInterface
	Hotkeys         HotkeyServiceInterface
	Window          window.Manager
	TempFileManager *processing.TempFileManager
	InputDaemon     *outputters.YdotoolDaemon
}

// Shutdown gracefully shuts down all services
func (sc *ServiceContainer) Shutdown() error {
	var errs []error
	if sc.Audio != nil {
		if err := sc.Audio.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}
	if sc.UI != nil {
		if err := sc.UI.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}
	if sc.IO != nil {
		if err := sc.IO.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}
	if sc.Config != nil {
		if err := sc.Config.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}
	if sc.Hotkeys != nil {
		if err := sc.Hotkeys.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}
	if sc.TempFileManager != nil {
		sc.TempFileManager.Stop()
		sc.TempFileManager.CleanupAll()
	}
	if sc.InputDaemon != nil {
		sc.InputDaemon.Stop()
	}

	return errors.Join(errs...)
}
