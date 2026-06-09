// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"testing"

	"github.com/AshBuk/dabri/v2/hotkeys/interfaces"
	"github.com/AshBuk/dabri/v2/internal/testutils"
)

func TestNewDummyKeyboardProvider(t *testing.T) {
	provider := NewDummyKeyboardProvider(testutils.NewMockLogger())

	if provider == nil {
		t.Fatal("NewDummyKeyboardProvider returned nil")
	}

	if provider.callbacks == nil {
		t.Error("callbacks map should be initialized")
	}

	if provider.isListening {
		t.Error("provider should not be listening initially")
	}

	// Test that it implements the interface
	var _ interfaces.KeyboardEventProvider = provider
}

func TestDummyKeyboardProvider_IsSupported(t *testing.T) {
	provider := NewDummyKeyboardProvider(testutils.NewMockLogger())

	// Dummy provider should always be supported
	if !provider.IsSupported() {
		t.Error("DummyKeyboardProvider should always be supported")
	}

	// Test multiple calls
	for i := 0; i < 5; i++ {
		if !provider.IsSupported() {
			t.Errorf("IsSupported should be consistent, failed on call %d", i+1)
		}
	}
}

func TestDummyKeyboardProvider_Start(t *testing.T) {
	provider := NewDummyKeyboardProvider(testutils.NewMockLogger())

	// Initial state
	if provider.isListening {
		t.Error("provider should not be listening initially")
	}

	// Start should succeed
	err := provider.Start()
	if err != nil {
		t.Errorf("Start should not return error, got: %v", err)
	}

	if !provider.isListening {
		t.Error("provider should be listening after Start()")
	}

	// Starting again should return error
	err = provider.Start()
	if err == nil {
		t.Error("Starting already started provider should return error")
	}

	expectedErrMsg := "dummy keyboard provider already started"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
	}
}

func TestDummyKeyboardProvider_Stop(t *testing.T) {
	provider := NewDummyKeyboardProvider(testutils.NewMockLogger())

	// Stop without starting should be safe
	provider.Stop()
	if provider.isListening {
		t.Error("provider should not be listening after Stop()")
	}

	// Start then stop
	provider.Start()
	if !provider.isListening {
		t.Error("provider should be listening after Start()")
	}

	provider.Stop()
	if provider.isListening {
		t.Error("provider should not be listening after Stop()")
	}

	// Multiple stops should be safe
	provider.Stop()
	provider.Stop()
	if provider.isListening {
		t.Error("provider should remain stopped after multiple Stop() calls")
	}
}

func TestDummyKeyboardProvider_RegisterHotkey(t *testing.T) {
	provider := NewDummyKeyboardProvider(testutils.NewMockLogger())
	callbackCalled := false

	callback := func() error {
		callbackCalled = true
		return nil
	}

	// Register a hotkey
	err := provider.RegisterHotkey("ctrl+r", "ctrl+r", callback)
	if err != nil {
		t.Errorf("RegisterHotkey should not return error, got: %v", err)
	}

	// Check callback is stored
	if len(provider.callbacks) != 1 {
		t.Errorf("Expected 1 callback, got %d", len(provider.callbacks))
	}

	storedCallback, exists := provider.callbacks["ctrl+r"]
	if !exists {
		t.Error("Callback should be stored with correct key")
	}

	// Test calling the stored callback
	err = storedCallback()
	if err != nil {
		t.Errorf("Stored callback returned error: %v", err)
	}

	if !callbackCalled {
		t.Error("Callback should have been called")
	}
}

func TestDummyKeyboardProvider_RegisterMultipleHotkeys(t *testing.T) {
	provider := NewDummyKeyboardProvider(testutils.NewMockLogger())

	hotkeys := []string{
		"ctrl+r",
		"alt+f",
		"shift+space",
		"F12",
		"ctrl+shift+r",
	}

	callbackResults := make(map[string]bool)

	// Register multiple hotkeys
	for _, hotkey := range hotkeys {
		localHotkey := hotkey // capture for closure
		callback := func() error {
			callbackResults[localHotkey] = true
			return nil
		}

		err := provider.RegisterHotkey(hotkey, hotkey, callback)
		if err != nil {
			t.Errorf("RegisterHotkey failed for '%s': %v", hotkey, err)
		}
	}

	// Check all callbacks are stored
	if len(provider.callbacks) != len(hotkeys) {
		t.Errorf("Expected %d callbacks, got %d", len(hotkeys), len(provider.callbacks))
	}

	// Test all callbacks
	for _, hotkey := range hotkeys {
		callback, exists := provider.callbacks[hotkey]
		if !exists {
			t.Errorf("Callback for '%s' not found", hotkey)
			continue
		}

		err := callback()
		if err != nil {
			t.Errorf("Callback for '%s' returned error: %v", hotkey, err)
		}
	}

	// Verify all callbacks were called
	for _, hotkey := range hotkeys {
		if !callbackResults[hotkey] {
			t.Errorf("Callback for '%s' was not called", hotkey)
		}
	}
}

func TestDummyKeyboardProvider_OverwriteHotkey(t *testing.T) {
	provider := NewDummyKeyboardProvider(testutils.NewMockLogger())

	firstCallbackCalled := false
	secondCallbackCalled := false

	firstCallback := func() error {
		firstCallbackCalled = true
		return nil
	}

	secondCallback := func() error {
		secondCallbackCalled = true
		return nil
	}

	// Register first callback
	err := provider.RegisterHotkey("ctrl+r", "ctrl+r", firstCallback)
	if err != nil {
		t.Errorf("RegisterHotkey failed: %v", err)
	}

	// Register second callback for same hotkey (should overwrite)
	err = provider.RegisterHotkey("ctrl+r", "ctrl+r", secondCallback)
	if err != nil {
		t.Errorf("RegisterHotkey failed: %v", err)
	}

	// Should only have one callback stored
	if len(provider.callbacks) != 1 {
		t.Errorf("Expected 1 callback, got %d", len(provider.callbacks))
	}

	// Call the stored callback
	callback := provider.callbacks["ctrl+r"]
	err = callback()
	if err != nil {
		t.Errorf("Callback returned error: %v", err)
	}

	// Only second callback should have been called
	if firstCallbackCalled {
		t.Error("First callback should not have been called")
	}
	if !secondCallbackCalled {
		t.Error("Second callback should have been called")
	}
}

func TestDummyKeyboardProvider_StartStopCycle(t *testing.T) {
	provider := NewDummyKeyboardProvider(testutils.NewMockLogger())

	// Start -> Stop -> Start cycle
	err := provider.Start()
	if err != nil {
		t.Errorf("First start failed: %v", err)
	}

	provider.Stop()

	err = provider.Start()
	if err != nil {
		t.Errorf("Second start failed: %v", err)
	}

	if !provider.isListening {
		t.Error("Provider should be listening after restart")
	}

	provider.Stop()
	if provider.isListening {
		t.Error("Provider should not be listening after final stop")
	}
}

func TestDummyKeyboardProvider_StateConsistency(t *testing.T) {
	provider := NewDummyKeyboardProvider(testutils.NewMockLogger())

	// Test various state combinations
	scenarios := []struct {
		name          string
		action        func() error
		expectedState bool
		expectError   bool
	}{
		{"initial state", func() error { return nil }, false, false},
		{"first start", provider.Start, true, false},
		{"start again", provider.Start, true, true},
		{"stop", func() error { provider.Stop(); return nil }, false, false},
		{"stop again", func() error { provider.Stop(); return nil }, false, false},
		{"restart", provider.Start, true, false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			err := scenario.action()

			if scenario.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !scenario.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if provider.isListening != scenario.expectedState {
				t.Errorf("Expected isListening=%v, got %v", scenario.expectedState, provider.isListening)
			}
		})
	}
}
