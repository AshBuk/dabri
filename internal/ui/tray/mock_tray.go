// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package tray

import (
	"context"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// MockTrayManager implements a mock version of TrayManager without external dependencies
type MockTrayManager struct {
	isRecording            bool
	logger                 logger.Logger
	onExit                 func()
	onToggle               func() error
	onShowConfig           func() error
	onShowAbout            func() error
	onResetToDefaults      func() error
	onSelectRecorder       func(method string) error
	onSelectLang           func(language string) error
	onToggleWorkflowNotify func() error
	onGetOutputTools       func() (clipboardTool, typeTool string)
	onSelectOutputMode     func(mode string) error
}

// CreateMockTrayManager creates a mock tray manager that doesn't use systray.
// Callbacks are wired later via setter methods.
func CreateMockTrayManager(logger logger.Logger) Manager {
	return &MockTrayManager{
		logger: logger,
	}
}

// Start initializes and starts the mock system tray (no-op)
func (tm *MockTrayManager) Start() {
	tm.logger.Info("Mock tray started (no actual system tray is shown)")
}

func (tm *MockTrayManager) SetRecordingState(isRecording bool) {
	tm.isRecording = isRecording
	if isRecording {
		tm.logger.Info("Mock tray: Recording ON")
	} else {
		tm.logger.Info("Mock tray: Recording OFF")
	}
}

func (tm *MockTrayManager) UpdateSettings(config *config.Config) {
	tm.logger.Info("Mock tray: Settings updated")
}

func (tm *MockTrayManager) Stop() {
	tm.logger.Info("Mock tray stopped")
}

func (tm *MockTrayManager) SetExitAction(onExit func()) {
	tm.onExit = onExit
}

func (tm *MockTrayManager) SetCoreActions(onToggle func() error, onShowConfig func() error, onShowAbout func() error, onResetToDefaults func() error) {
	tm.onToggle = onToggle
	tm.onShowConfig = onShowConfig
	tm.onShowAbout = onShowAbout
	tm.onResetToDefaults = onResetToDefaults
}

func (tm *MockTrayManager) SetAudioActions(onSelectRecorder func(method string) error) {
	tm.onSelectRecorder = onSelectRecorder
}

func (tm *MockTrayManager) SetSettingsActions(
	onSelectLanguage func(language string) error,
	onToggleWorkflowNotifications func() error,
	onSelectOutputMode func(mode string) error,
) {
	tm.onSelectLang = onSelectLanguage
	tm.onToggleWorkflowNotify = onToggleWorkflowNotifications
	tm.onSelectOutputMode = onSelectOutputMode
}

func (tm *MockTrayManager) OutputToolsCallback(callback func() (clipboardTool, typeTool string)) {
	tm.onGetOutputTools = callback
}

func (tm *MockTrayManager) SetCaptureOnceSupport(_ func() bool) {}

func (tm *MockTrayManager) SetModelAction(_ func(ctx context.Context, modelID string) error) {}

func (tm *MockTrayManager) SetHotkeyRebindAction(_ func(action string) error) {}
