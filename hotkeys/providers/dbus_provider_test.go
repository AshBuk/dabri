// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"testing"

	"github.com/AshBuk/dabri/v2/internal/testutils"
)

func TestNewDbusKeyboardProvider(t *testing.T) {
	provider := NewDbusKeyboardProvider(testutils.NewMockLogger())
	if provider == nil {
		t.Fatal("NewDbusKeyboardProvider returned nil")
	}

	if provider.callbacks == nil {
		t.Error("callbacks map not initialized")
	}
	if provider.isListening {
		t.Error("should not be listening initially")
	}
}

func TestDbusKeyboardProvider_IsSupported_NewTest(t *testing.T) {
	provider := NewDbusKeyboardProvider(testutils.NewMockLogger())
	// Test IsSupported - this will likely return false in test environment
	supported := provider.IsSupported()

	// We don't assert true/false here since it depends on the test environment
	// Just verify the method doesn't panic and returns a boolean
	if supported {
		t.Log("D-Bus portal GlobalShortcuts is supported in test environment")
	} else {
		t.Log("D-Bus portal GlobalShortcuts is not supported in test environment (expected)")
	}
}

func TestDbusKeyboardProvider_RegisterHotkey(t *testing.T) {
	provider := NewDbusKeyboardProvider(testutils.NewMockLogger())
	callbackCalled := false
	callback := func() error {
		callbackCalled = true
		return nil
	}
	// Test registering a hotkey
	err := provider.RegisterHotkey("ctrl+shift+a", callback)
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

func TestDbusKeyboardProvider_RegisterHotkey_Duplicate(t *testing.T) {
	provider := NewDbusKeyboardProvider(testutils.NewMockLogger())
	callback := func() error { return nil }
	// Register first hotkey
	err := provider.RegisterHotkey("ctrl+a", callback)
	if err != nil {
		t.Errorf("unexpected error registering first hotkey: %v", err)
	}
	// Try to register the same hotkey again
	err = provider.RegisterHotkey("ctrl+a", callback)
	if err == nil {
		t.Error("expected error when registering duplicate hotkey")
	}
	if err.Error() != "hotkey ctrl+a already registered" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDbusKeyboardProvider_Start_AlreadyStarted(t *testing.T) {
	provider := NewDbusKeyboardProvider(testutils.NewMockLogger())
	// Set isListening to true to simulate already started
	provider.isListening = true

	err := provider.Start()
	if err == nil {
		t.Error("expected error when starting already started provider")
	}
	if err.Error() != "D-Bus keyboard provider already started" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDbusKeyboardProvider_Stop_NotStarted(t *testing.T) {
	provider := NewDbusKeyboardProvider(testutils.NewMockLogger())
	// Stop should not panic even if not started
	provider.Stop()
	// Verify state
	if provider.isListening {
		t.Error("isListening should remain false")
	}
}

func TestDbusKeyboardProvider_Stop_WhenStarted(t *testing.T) {
	provider := NewDbusKeyboardProvider(testutils.NewMockLogger())
	// Simulate started state
	provider.isListening = true

	provider.Stop()

	if provider.isListening {
		t.Error("expected isListening to be false after stop")
	}
	if provider.session != nil {
		t.Error("expected session to be nil after stop")
	}
}
