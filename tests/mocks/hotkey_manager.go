// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package mocks

import (
	"time"

	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/hotkeys/manager"
)

// HotkeyManager defines the interface that MockHotkeyManager implements
type HotkeyManager interface {
	Start() error
	Stop()
	RegisterCallbacks(startRecording, stopRecording func() error)
	RegisterHotkeyAction(action string, callback manager.HotkeyAction)
	ReloadConfig(newConfig adapters.HotkeyConfig) error
	CaptureOnce(timeout time.Duration) (string, error)
	SupportsCaptureOnce() bool
}

// MockHotkeyManager implements HotkeyManager for testing
type MockHotkeyManager struct {
	startCalled                bool
	stopCalled                 bool
	registerCallbacksCalled    bool
	registerHotkeyActionCalled map[string]bool
	startError                 error
	callbacks                  struct {
		startRecording  func() error
		stopRecording   func() error
		showConfig      manager.HotkeyAction
		resetToDefaults manager.HotkeyAction
	}
}

func (m *MockHotkeyManager) Start() error {
	m.startCalled = true
	return m.startError
}

func (m *MockHotkeyManager) Stop() {
	m.stopCalled = true
}

func (m *MockHotkeyManager) RegisterCallbacks(startRecording, stopRecording func() error) {
	m.registerCallbacksCalled = true
	m.callbacks.startRecording = startRecording
	m.callbacks.stopRecording = stopRecording
}

func (m *MockHotkeyManager) RegisterHotkeyAction(action string, callback manager.HotkeyAction) {
	if m.registerHotkeyActionCalled == nil {
		m.registerHotkeyActionCalled = make(map[string]bool)
	}
	m.registerHotkeyActionCalled[action] = true

	switch action {
	case "show_config":
		m.callbacks.showConfig = callback
	case "reset_to_defaults":
		m.callbacks.resetToDefaults = callback
	}
}

func (m *MockHotkeyManager) ReloadConfig(_ adapters.HotkeyConfig) error { return nil }

func (m *MockHotkeyManager) CaptureOnce(_ time.Duration) (string, error) { return "alt+r", nil }

func (m *MockHotkeyManager) SupportsCaptureOnce() bool { return true }

// Test helper methods
func (m *MockHotkeyManager) WasStartCalled() bool { return m.startCalled }

func (m *MockHotkeyManager) WasStopCalled() bool { return m.stopCalled }

func (m *MockHotkeyManager) WereCallbacksRegistered() bool { return m.registerCallbacksCalled }

func (m *MockHotkeyManager) WasHotkeyActionRegistered(action string) bool {
	return m.registerHotkeyActionCalled[action]
}

func (m *MockHotkeyManager) TriggerCallback(callbackType string) error {
	switch callbackType {
	case "startRecording":
		if m.callbacks.startRecording != nil {
			return m.callbacks.startRecording()
		}
	case "showConfig":
		if m.callbacks.showConfig != nil {
			return m.callbacks.showConfig()
		}
	case "resetToDefaults":
		if m.callbacks.resetToDefaults != nil {
			return m.callbacks.resetToDefaults()
		}
	}
	return nil
}

// SetStartError sets the error that will be returned by Start method
func (m *MockHotkeyManager) SetStartError(err error) { m.startError = err }
