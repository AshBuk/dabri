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

func TestHotkeyManager_RegisterToggle(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+shift+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider

	toggleCount := 0
	manager.RegisterToggle(func() error {
		toggleCount++
		return nil
	})

	// Each press fires the toggle; the callback owns the start/stop decision.
	for i := 1; i <= 2; i++ {
		if err := manager.SimulateHotkeyPress("toggle_recording"); err != nil {
			t.Errorf("Toggle press %d failed: %v", i, err)
		}
		if toggleCount != i {
			t.Errorf("Expected toggle called %d time(s), got %d", i, toggleCount)
		}
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
	manager.RegisterToggle(func() error {
		return testError
	})

	err := manager.SimulateHotkeyPress("toggle_recording")
	if err != testError {
		t.Errorf("Expected callback error, got %v", err)
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

	manager.RegisterToggle(func() error { return nil })

	// Test concurrent toggle presses (exercises the hotkey mutex)
	done := make(chan bool, 2)

	for g := 0; g < 2; g++ {
		go func() {
			for i := 0; i < 100; i++ {
				_ = manager.SimulateHotkeyPress("toggle_recording")
			}
			done <- true
		}()
	}

	<-done
	<-done
}
