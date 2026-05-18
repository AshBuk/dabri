//go:build linux

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package manager

import (
	"fmt"
	"testing"

	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/hotkeys/interfaces"
	"github.com/AshBuk/dabri/v2/hotkeys/mocks"
	"github.com/AshBuk/dabri/v2/hotkeys/utils"
	"github.com/AshBuk/dabri/v2/internal/testutils"
)

func TestParseHotkey(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedKey       string
		expectedModifiers []string
	}{
		{
			name:              "single key",
			input:             "r",
			expectedKey:       "r",
			expectedModifiers: nil,
		},
		{
			name:              "single key with whitespace",
			input:             "  Space  ",
			expectedKey:       "Space",
			expectedModifiers: nil,
		},
		{
			name:              "ctrl+key",
			input:             "ctrl+r",
			expectedKey:       "r",
			expectedModifiers: []string{"ctrl"},
		},
		{
			name:              "multiple modifiers",
			input:             "ctrl+shift+r",
			expectedKey:       "r",
			expectedModifiers: []string{"ctrl", "shift"},
		},
		{
			name:              "complex combination",
			input:             "ctrl+alt+shift+F12",
			expectedKey:       "F12",
			expectedModifiers: []string{"ctrl", "alt", "shift"},
		},
		{
			name:              "with whitespace",
			input:             " ctrl + shift + r ",
			expectedKey:       "r",
			expectedModifiers: []string{"ctrl", "shift"},
		},
		{
			name:              "mixed case modifiers",
			input:             "Ctrl+Shift+Alt+r",
			expectedKey:       "r",
			expectedModifiers: []string{"ctrl", "shift", "alt"},
		},
		{
			name:              "altgr modifier",
			input:             "altgr+comma",
			expectedKey:       "comma",
			expectedModifiers: []string{"altgr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.ParseHotkey(tt.input)
			if result.Key != tt.expectedKey {
				t.Errorf("Expected key '%s', got '%s'", tt.expectedKey, result.Key)
			}

			if len(result.Modifiers) != len(tt.expectedModifiers) {
				t.Errorf("Expected %d modifiers, got %d", len(tt.expectedModifiers), len(result.Modifiers))
			}
			for i, expectedModifier := range tt.expectedModifiers {
				if i >= len(result.Modifiers) {
					t.Errorf("Missing modifier at index %d", i)
					continue
				}
				if result.Modifiers[i] != expectedModifier {
					t.Errorf("Expected modifier '%s' at index %d, got '%s'",
						expectedModifier, i, result.Modifiers[i])
				}
			}
		})
	}
}

func TestIsModifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"ctrl", "ctrl", true},
		{"shift", "shift", true},
		{"alt", "alt", true},
		{"altgr", "altgr", true},
		{"super", "super", true},
		{"meta", "meta", true},
		{"hyper", "hyper", true},
		{"regular key", "r", false},
		{"function key", "F1", false},
		{"number", "1", false},
		{"special char", "/", false},
		{"empty string", "", false},
		{"mixed case ctrl", "Ctrl", true}, // IsModifier should be case insensitive
		{"mixed case shift", "SHIFT", true},
		{"unknown modifier", "unknown", false},
		{"space", "space", false},
		{"tab", "tab", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.IsModifier(tt.input)
			if result != tt.expected {
				t.Errorf("utils.IsModifier('%s') = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertModifierToEvdev(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"ctrl", "ctrl", "leftctrl"},
		{"shift", "shift", "leftshift"},
		{"alt", "alt", "leftalt"},
		{"super", "super", "leftmeta"},
		{"meta", "meta", "leftmeta"},
		{"win", "win", "leftmeta"},
		{"altgr", "altgr", "rightalt"},
		{"AltGr", "AltGr", "rightalt"}, // Test case insensitive
		{"unknown modifier", "unknown", "unknown"},
		{"empty string", "", ""},
		{"regular key", "r", "r"},
		{"mixed case", "Ctrl", "leftctrl"}, // should convert to lowercase
		{"SHIFT", "SHIFT", "leftshift"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.ConvertModifierToEvdev(tt.input)
			if result != tt.expected {
				t.Errorf("utils.ConvertModifierToEvdev('%s') = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHotkeyManager_StartWithProviderFailure(t *testing.T) {
	// Create a simple config adapter for testing
	config := adapters.NewConfigAdapter("ctrl+r", "auto")

	// Test when provider fails to start
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())

	// Replace with a mock that always fails
	mockProvider := mocks.NewMockHotkeyProvider()
	mockProvider.SetStartError(fmt.Errorf("provider start failed"))
	manager.provider = mockProvider

	err := manager.Start()
	if err == nil {
		t.Error("Expected error when provider fails to start")
	}

	if manager.isListening {
		t.Error("Manager should not be listening when provider fails to start")
	}
}

func TestHotkeyManager_StartWithNoProvider(t *testing.T) {
	// Create a simple config adapter for testing
	config := adapters.NewConfigAdapter("ctrl+r", "auto")

	// Test when no provider is available
	manager := &HotkeyManager{
		config:      config,
		environment: interfaces.EnvironmentUnknown,
		provider:    nil,
		isListening: false,
	}

	err := manager.Start()
	if err == nil {
		t.Error("Expected error when no provider is available")
	}
}

func TestSelectKeyboardProvider(t *testing.T) {
	// This tests the selectKeyboardProvider function indirectly through NewHotkeyManager
	tests := []struct {
		name        string
		environment interfaces.EnvironmentType
	}{
		{"X11 environment", interfaces.EnvironmentX11},
		{"Wayland environment", interfaces.EnvironmentWayland},
		{"Unknown environment", interfaces.EnvironmentUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := adapters.NewConfigAdapter("ctrl+r", "auto")
			manager := NewHotkeyManager(config, tt.environment, testutils.NewMockLogger())

			if manager == nil {
				t.Fatal("NewHotkeyManager returned nil")
			}

			if manager.environment != tt.environment {
				t.Errorf("Expected environment %v, got %v", tt.environment, manager.environment)
			}

			if manager.provider == nil {
				t.Error("Provider should not be nil")
			}
		})
	}
}

func TestHotkeyManager_StopWithoutStart(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())

	// Stop without start should be safe
	manager.Stop()

	if manager.isListening {
		t.Error("Manager should not be listening after Stop without Start")
	}
}

func TestHotkeyManager_MultipleStartStop(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider

	// Start
	err := manager.Start()
	if err != nil {
		t.Fatalf("First start failed: %v", err)
	}

	// Start again (should fail)
	err = manager.Start()
	if err == nil {
		t.Error("Second start should fail")
	}

	// Stop
	manager.Stop()

	// Start again (should work)
	err = manager.Start()
	if err != nil {
		t.Errorf("Restart failed: %v", err)
	}

	// Stop again
	manager.Stop()

	if manager.isListening {
		t.Error("Manager should not be listening after final stop")
	}
}

func TestHotkeyManager_ConcurrentAccess_Extended(t *testing.T) {
	config := adapters.NewConfigAdapter("ctrl+r", "auto")
	manager := NewHotkeyManager(config, interfaces.EnvironmentX11, testutils.NewMockLogger())
	mockProvider := mocks.NewMockHotkeyProvider()
	manager.provider = mockProvider

	// Test concurrent access to IsRecording
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				manager.IsRecording()
				manager.SimulateHotkeyPress("start_recording")
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should complete successfully
}
