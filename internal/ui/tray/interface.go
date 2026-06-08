// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package tray

import (
	"context"

	"github.com/AshBuk/dabri/v2/config"
)

// Defines the interface for tray managers
type Manager interface {
	Start()
	SetRecordingState(isRecording bool)
	UpdateSettings(config *config.Config)
	// SetExitAction sets the callback invoked when Quit is clicked
	SetExitAction(onExit func())
	// SetCoreActions sets core menu callbacks (toggle, show config, show about, reset to defaults)
	SetCoreActions(onToggle func() error, onShowConfig func() error, onShowAbout func() error, onResetToDefaults func() error)
	// SetShowWindowAction sets the callback invoked when "Open Window" is clicked
	SetShowWindowAction(onShowWindow func())
	// SetAudioActions sets callbacks for audio-related actions
	SetAudioActions(onSelectRecorder func(method string) error)
	// SetModelAction sets callback for whisper model selection.
	// The callback receives a cancellable context and a progress reporter for download feedback.
	SetModelAction(onSelectModel func(ctx context.Context, modelID string) error)
	// SetHotkeyRebindAction sets callback to rebind hotkeys by action name
	SetHotkeyRebindAction(onRebind func(action string) error)
	// SetSettingsActions sets callbacks for general settings from tray (Language, Notifications)
	SetSettingsActions(
		onSelectLanguage func(language string) error,
		onToggleWorkflowNotifications func() error,
		onSelectOutputMode func(mode string) error,
	)
	// OutputToolsCallback sets the callback for getting actual output tool names
	OutputToolsCallback(callback func() (clipboardTool, typeTool string))
	// SetCaptureOnceSupport sets a callback indicating whether capture once is supported
	SetCaptureOnceSupport(callback func() bool)
	Stop()
}
