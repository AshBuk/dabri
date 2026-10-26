// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package mocks

import (
	"context"
	"time"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
)

// CallLog records calls in order; share one log across mocks to assert
// cross-service sequencing. A nil log records nothing.
type CallLog struct{ Calls []string }

func (l *CallLog) add(call string) {
	if l != nil {
		l.Calls = append(l.Calls, call)
	}
}

// MockAudioService implements AudioServiceInterface for testing
type MockAudioService struct {
	Log            *CallLog
	Recording      bool
	SwitchModelErr map[string]error // keyed by model ID
	shutdownCalled bool
	shutdownError  error
}

func (m *MockAudioService) Shutdown() error {
	m.shutdownCalled = true
	return m.shutdownError
}

func (m *MockAudioService) HandleStartRecording() error { m.Log.add("Audio.Start"); return nil }
func (m *MockAudioService) HandleStopRecording() error  { m.Log.add("Audio.Stop"); return nil }
func (m *MockAudioService) IsRecording() bool           { return m.Recording }
func (m *MockAudioService) ClearSession()               { m.Log.add("Audio.ClearSession") }
func (m *MockAudioService) SwitchModel(_ context.Context, id string) error {
	m.Log.add("Audio.SwitchModel:" + id)
	return m.SwitchModelErr[id]
}
func (m *MockAudioService) DeleteModel(_ string) error { return nil }
func (m *MockAudioService) GetLastTranscript() string  { return "" }

// Test helper methods
func (m *MockAudioService) WasShutdownCalled() bool { return m.shutdownCalled }

func (m *MockAudioService) SetShutdownError(err error) { m.shutdownError = err }

// MockUIService implements UIServiceInterface for testing
type MockUIService struct {
	Log            *CallLog
	shutdownCalled bool
	shutdownError  error
}

func (m *MockUIService) Shutdown() error {
	m.shutdownCalled = true
	return m.shutdownError
}

func (m *MockUIService) SetRecordingState(isRecording bool)     {}
func (m *MockUIService) ShowNotification(title, message string) { m.Log.add("UI.Notify:" + title) }
func (m *MockUIService) SetError(message string)                {}
func (m *MockUIService) SetSuccess(message string)              {}
func (m *MockUIService) ShowConfigFile() error                  { m.Log.add("UI.ShowConfig"); return nil }
func (m *MockUIService) ShowAboutPage() error                   { return nil }
func (m *MockUIService) UpdateSettings(cfg *config.Config)      { m.Log.add("UI.UpdateSettings") }

// Test helper methods
func (m *MockUIService) WasShutdownCalled() bool { return m.shutdownCalled }

func (m *MockUIService) SetShutdownError(err error) { m.shutdownError = err }

// MockIOService implements IOServiceInterface for testing
type MockIOService struct {
	Log            *CallLog
	shutdownCalled bool
	shutdownError  error
}

func (m *MockIOService) Shutdown() error {
	m.shutdownCalled = true
	return m.shutdownError
}

func (m *MockIOService) OutputText(text string) error { return nil }
func (m *MockIOService) SetOutputMethod(method string) error {
	m.Log.add("IO.SetOutputMethod:" + method)
	return nil
}
func (m *MockIOService) BeginTranscription()                                        {}
func (m *MockIOService) CompleteTranscription(result string)                        {}
func (m *MockIOService) WaitForTranscription(timeout time.Duration) (string, error) { return "", nil }
func (m *MockIOService) GetOutputToolNames() (string, string)                       { return "mock-clipboard", "mock-typing" }
func (m *MockIOService) StartWebSocketServer() error                                { return nil }

// Test helper methods
func (m *MockIOService) WasShutdownCalled() bool { return m.shutdownCalled }

func (m *MockIOService) SetShutdownError(err error) { m.shutdownError = err }

// MockConfigService implements ConfigServiceInterface for testing
type MockConfigService struct {
	Log            *CallLog
	Cfg            *config.Config // returned by GetConfig; empty config when nil
	ResetErr       error
	UpdateModelErr error
	shutdownCalled bool
	shutdownError  error
}

func (m *MockConfigService) Shutdown() error {
	m.shutdownCalled = true
	return m.shutdownError
}

func (m *MockConfigService) ResetToDefaults() error { m.Log.add("Config.Reset"); return m.ResetErr }
func (m *MockConfigService) GetConfig() *config.Config {
	if m.Cfg == nil {
		m.Cfg = &config.Config{}
	}
	return m.Cfg
}

func (m *MockConfigService) UpdateLanguage(language string) error {
	m.Log.add("Config.Language:" + language)
	return nil
}
func (m *MockConfigService) UpdateModel(modelID string) error {
	m.Log.add("Config.Model:" + modelID)
	return m.UpdateModelErr
}
func (m *MockConfigService) ToggleWorkflowNotifications() error {
	m.Log.add("Config.ToggleNotifications")
	return nil
}
func (m *MockConfigService) UpdateRecordingMethod(method string) error {
	m.Log.add("Config.RecordingMethod:" + method)
	return nil
}
func (m *MockConfigService) UpdateOutputMode(mode string) error {
	m.Log.add("Config.OutputMode:" + mode)
	return nil
}
func (m *MockConfigService) UpdateHotkeyMode(mode string) error {
	m.Log.add("Config.HotkeyMode:" + mode)
	return nil
}
func (m *MockConfigService) UpdateHotkey(action, combo string) error {
	m.Log.add("Config.Hotkey:" + action + "=" + combo)
	return nil
}

// Test helper methods
func (m *MockConfigService) WasShutdownCalled() bool { return m.shutdownCalled }

func (m *MockConfigService) SetShutdownError(err error) { m.shutdownError = err }

// MockHotkeyService implements HotkeyServiceInterface for testing
type MockHotkeyService struct {
	Log            *CallLog
	ReloadErr      error  // returned by ReloadFromConfig
	Captured       string // returned by CaptureOnce
	PTT            bool   // returned by PushToTalk
	shutdownCalled bool
	shutdownError  error
}

func (m *MockHotkeyService) Shutdown() error {
	m.shutdownCalled = true
	return m.shutdownError
}

func (m *MockHotkeyService) SetupHotkeyCallbacks(
	recording func(pressed bool) error,
	showConfig func() error,
	resetToDefaults func() error,
) error {
	return nil
}
func (m *MockHotkeyService) RegisterHotkeys() error { return nil }
func (m *MockHotkeyService) ReloadFromConfig(recording func(pressed bool) error, cfg adapters.HotkeyConfig) error {
	m.Log.add("Hotkeys.Reload:" + cfg.GetStartRecordingHotkey())
	return m.ReloadErr
}

// CaptureOnce mock implementation
func (m *MockHotkeyService) CaptureOnce(timeoutMs int) (string, error) {
	m.Log.add("Hotkeys.Capture")
	return m.Captured, nil
}

// SupportsCaptureOnce mock implementation
func (m *MockHotkeyService) SupportsCaptureOnce() bool { return true }

// PushToTalk mock implementation
func (m *MockHotkeyService) PushToTalk() bool { return m.PTT }

// Test helper methods
func (m *MockHotkeyService) WasShutdownCalled() bool { return m.shutdownCalled }

func (m *MockHotkeyService) SetShutdownError(err error) { m.shutdownError = err }
