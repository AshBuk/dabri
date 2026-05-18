// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package manager

import (
	"errors"
	"testing"

	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/hotkeys/interfaces"
	"github.com/AshBuk/dabri/v2/hotkeys/mocks"
	"github.com/AshBuk/dabri/v2/internal/testutils"
)

func TestNewHotkeyManager(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	if manager == nil {
		t.Fatal("NewHotkeyManager returned nil")
	}

	if manager.config != config {
		t.Error("Config not set correctly")
	}

	if manager.environment != interfaces.EnvironmentX11 {
		t.Error("Environment not set correctly")
	}

	if manager.isRecording {
		t.Error("Initial recording state should be false")
	}

	if manager.provider == nil {
		t.Error("Provider should be initialized")
	}
}

func TestHotkeyManager_Start_Success(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider immediately after creation
	mockProvider := mocks.NewMockHotkeyProvider()
	mockProvider.SetSupported(true) // Ensure it supports registration
	manager.provider = mockProvider
	err := manager.Start()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !mockProvider.IsStarted() {
		t.Error("Expected provider Start to be called")
	}
}

func TestHotkeyManager_Start_ProviderError(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider that returns error
	mockProvider := mocks.NewMockHotkeyProvider()
	mockProvider.SetSupported(true) // Ensure it supports registration
	mockProvider.SetStartError(errors.New("provider start failed"))
	manager.provider = mockProvider
	err := manager.Start()
	if err == nil {
		t.Error("Expected error when provider fails to start")
	}
	// Do not check IsStarted() here, as mock should not be started on start error
}

func TestHotkeyManager_Stop(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider immediately after creation
	mockProvider := mocks.NewMockHotkeyProvider()
	mockProvider.SetSupported(true) // Ensure it supports registration
	manager.provider = mockProvider

	_ = manager.Start() // Start first, then stop
	manager.Stop()

	if !mockProvider.WasStopCalled() {
		t.Error("Expected provider Stop to be called")
	}
}

func TestHotkeyManager_RegisterCallbacks(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider

	startCalled := false
	stopCalled := false

	startCallback := func() error {
		startCalled = true
		return nil
	}

	stopCallback := func() error {
		stopCalled = true
		return nil
	}

	manager.RegisterCallbacks(startCallback, stopCallback)
	// Check callback invocation via public API
	err := manager.SimulateHotkeyPress("start_recording")
	if err != nil {
		t.Errorf("Start callback failed: %v", err)
	}
	if !startCalled {
		t.Error("Start callback not called")
	}

	err = manager.SimulateHotkeyPress("stop_recording")
	if err != nil {
		t.Errorf("Stop callback failed: %v", err)
	}
	if !stopCalled {
		t.Error("Stop callback not called")
	}
}

func TestHotkeyManager_IsRecording(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider
	// Initially not recording
	if manager.IsRecording() {
		t.Error("Expected IsRecording to be false initially")
	}

	// Simulate start recording
	manager.isRecording = true
	if !manager.IsRecording() {
		t.Error("Expected IsRecording to be true after setting")
	}

	// Simulate stop recording
	manager.isRecording = false
	if manager.IsRecording() {
		t.Error("Expected IsRecording to be false after resetting")
	}
}

func TestHotkeyManager_SimulateHotkeyPress_StartRecording(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider

	startCalled := false
	stopCalled := false

	manager.RegisterCallbacks(
		func() error {
			startCalled = true
			return nil
		},
		func() error {
			stopCalled = true
			return nil
		},
	)

	err := manager.SimulateHotkeyPress("start_recording")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !startCalled {
		t.Error("Expected start recording callback to be called")
	}

	if stopCalled {
		t.Error("Expected stop recording callback not to be called")
	}

	if !manager.IsRecording() {
		t.Error("Expected recording state to be true after start")
	}
}

func TestHotkeyManager_SimulateHotkeyPress_StopRecording(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider

	startCalled := false
	stopCalled := false

	manager.RegisterCallbacks(
		func() error {
			startCalled = true
			return nil
		},
		func() error {
			stopCalled = true
			return nil
		},
	)

	// Set recording state to true
	manager.isRecording = true
	err := manager.SimulateHotkeyPress("stop_recording")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if startCalled {
		t.Error("Expected start recording callback not to be called")
	}

	if !stopCalled {
		t.Error("Expected stop recording callback to be called")
	}

	if manager.IsRecording() {
		t.Error("Expected recording state to be false after stop")
	}
}

func TestHotkeyManager_SimulateHotkeyPress_InvalidAction(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider

	err := manager.SimulateHotkeyPress("invalid_action")

	if err == nil {
		t.Error("Expected error for invalid action")
	}
}

func TestHotkeyManager_SimulateHotkeyPress_CallbackError(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider

	testError := errors.New("callback error")

	manager.RegisterCallbacks(
		func() error {
			return testError
		},
		func() error {
			return nil
		},
	)

	err := manager.SimulateHotkeyPress("start_recording")

	if err != testError {
		t.Errorf("Expected callback error, got %v", err)
	}

	// Recording state should not change on error
	if manager.IsRecording() {
		t.Error("Expected recording state to remain false on error")
	}
}

func TestHotkeyManager_EnvironmentTypes(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")

	tests := []struct {
		name        string
		environment interfaces.EnvironmentType
	}{
		{
			name:        "X11 environment",
			environment: interfaces.EnvironmentX11,
		},
		{
			name:        "Wayland environment",
			environment: interfaces.EnvironmentWayland,
		},
		{
			name:        "Unknown environment",
			environment: interfaces.EnvironmentUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewHotkeyManager(config, tt.environment, testutils.NewMockLogger())

			// Replace with mock provider
			mockProvider := mocks.NewMockHotkeyProvider()
			manager.provider = mockProvider

			if manager.environment != tt.environment {
				t.Errorf("Expected environment %v, got %v", tt.environment, manager.environment)
			}
		})
	}
}

func TestHotkeyManager_ConfigAdapter(t *testing.T) {
	tests := []struct {
		name           string
		startRecording string
		expected       string
	}{
		{
			name:           "simple hotkey",
			startRecording: "ctrl+r",
			expected:       "ctrl+r",
		},
		{
			name:           "complex hotkey",
			startRecording: "ctrl+shift+alt+f1",
			expected:       "ctrl+shift+alt+f1",
		},
		{
			name:           "empty hotkey",
			startRecording: "",
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := adapters.NewConfigAdapter(tt.startRecording, "auto")

			if config.GetStartRecordingHotkey() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, config.GetStartRecordingHotkey())
			}
		})
	}
}

func TestHotkeyManager_ConcurrentAccess(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	// Replace with mock provider
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider

	// Test concurrent access to IsRecording
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 100; i++ {
			manager.IsRecording()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			manager.hotkeysMutex.Lock()
			manager.isRecording = i%2 == 0
			manager.hotkeysMutex.Unlock()
		}
		done <- true
	}()

	<-done
	<-done
}
