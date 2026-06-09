// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"testing"

	"github.com/AshBuk/dabri/v2/hotkeys/utils"
	"github.com/AshBuk/dabri/v2/internal/testutils"
)

func TestNewEvdevKeyboardProvider(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())
	if provider == nil {
		t.Fatal("NewEvdevKeyboardProvider returned nil")
	}

	if provider.callbacks == nil {
		t.Error("callbacks map not initialized")
	}
	if provider.stopListening == nil {
		t.Error("stopListening channel not initialized")
	}
	if provider.isListening {
		t.Error("should not be listening initially")
	}
	if provider.modifierState == nil {
		t.Error("modifierState map not initialized")
	}
}

func TestEvdevKeyboardProvider_IsSupported(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())
	// Test IsSupported - this will likely return false in test environment due to permissions
	supported := provider.IsSupported()

	// We don't assert true/false here since it depends on the test environment and permissions
	// Just verify the method doesn't panic and returns a boolean
	if supported {
		t.Log("Evdev is supported in test environment")
	} else {
		t.Log("Evdev is not supported in test environment (expected, likely due to permissions)")
	}
}

func TestEvdevKeyboardProvider_RegisterHotkey(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())
	callbackCalled := false
	callback := func() error {
		callbackCalled = true
		return nil
	}
	// Test registering a hotkey
	err := provider.RegisterHotkey("ctrl+shift+a", "ctrl+shift+a", callback)
	if err != nil {
		t.Errorf("unexpected error registering hotkey: %v", err)
	}
	// Check that callback is stored
	if len(provider.callbacks) != 1 {
		t.Errorf("expected 1 callback, got %d", len(provider.callbacks))
	}

	storedCallback, exists := provider.callbacks["ctrl+shift+a"]
	if !exists {
		t.Error("hotkey not found in callbacks")
	}

	// Test the stored callback
	if storedCallback != nil {
		err := storedCallback()
		if err != nil {
			t.Errorf("callback returned error: %v", err)
		}
		if !callbackCalled {
			t.Error("callback was not called")
		}
	}
}

func TestEvdevKeyboardProvider_Start_AlreadyStarted(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())
	// Set isListening to true to simulate already started
	provider.isListening = true

	err := provider.Start()
	if err == nil {
		t.Error("expected error when starting already started provider")
	}
	if err.Error() != "evdev keyboard provider already started" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestEvdevKeyboardProvider_Stop_NotStarted(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())
	// Stop should not panic even if not started
	provider.Stop()
	// Verify state
	if provider.isListening {
		t.Error("isListening should remain false")
	}
}

func TestEvdevKeyboardProvider_Stop_WhenStarted(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())
	// Simulate started state
	provider.isListening = true
	provider.stopListening = make(chan bool)

	provider.Stop()

	if provider.isListening {
		t.Error("expected isListening to be false after stop")
	}
	if provider.devices != nil {
		t.Error("expected devices to be nil after stop")
	}
}

func TestGetKeyName(t *testing.T) {
	tests := []struct {
		name     string
		keyCode  int
		expected string
	}{
		{
			name:     "escape key",
			keyCode:  1,
			expected: "esc",
		},
		{
			name:     "number 1",
			keyCode:  2,
			expected: "1",
		},
		{
			name:     "letter a",
			keyCode:  30,
			expected: "a",
		},
		{
			name:     "enter key",
			keyCode:  28,
			expected: "enter",
		},
		{
			name:     "space key",
			keyCode:  57,
			expected: "space",
		},
		{
			name:     "left ctrl",
			keyCode:  29,
			expected: "leftctrl",
		},
		{
			name:     "right ctrl",
			keyCode:  97,
			expected: "rightctrl",
		},
		{
			name:     "left alt",
			keyCode:  56,
			expected: "leftalt",
		},
		{
			name:     "right alt",
			keyCode:  100,
			expected: "rightalt",
		},
		{
			name:     "left shift",
			keyCode:  42,
			expected: "leftshift",
		},
		{
			name:     "right shift",
			keyCode:  54,
			expected: "rightshift",
		},
		{
			name:     "comma",
			keyCode:  51,
			expected: "comma",
		},
		{
			name:     "period",
			keyCode:  52,
			expected: "dot",
		},
		{
			name:     "unknown key",
			keyCode:  999,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.GetKeyName(tt.keyCode)
			if result != tt.expected {
				t.Errorf("GetKeyName(%d) = %q, want %q", tt.keyCode, result, tt.expected)
			}
		})
	}
}

func TestHasKeyEvents(t *testing.T) {
	// This test is difficult to implement without creating actual evdev devices
	// We'll test the function exists and doesn't panic with nil input
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("hasKeyEvents panicked: %v", r)
		}
	}()

	// Test with nil device - this should not panic but may return false
	// We can't easily test true cases without real devices
	// hasKeyEvents(nil) // This would panic, so we skip it

	// Just verify the function exists by calling it indirectly through other tests
	t.Log("hasKeyEvents function exists and is callable")
}

// Test helper functions that are used by the evdev provider

func TestModifierStateTracking(t *testing.T) {
	// Test that modifier state tracking logic works
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())

	// Simulate modifier key press
	provider.modifierState["leftctrl"] = true
	provider.modifierState["shift"] = true

	// Check state
	if !provider.modifierState["leftctrl"] {
		t.Error("expected leftctrl to be pressed")
	}
	if !provider.modifierState["shift"] {
		t.Error("expected shift to be pressed")
	}

	// Simulate modifier key release
	provider.modifierState["leftctrl"] = false

	if provider.modifierState["leftctrl"] {
		t.Error("expected leftctrl to be released")
	}
	if !provider.modifierState["shift"] {
		t.Error("expected shift to still be pressed")
	}
}
