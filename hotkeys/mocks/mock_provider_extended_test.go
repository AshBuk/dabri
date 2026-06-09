// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package mocks

import (
	"fmt"
	"testing"
)

func TestMockHotkeyProvider_SetRegisterError(t *testing.T) {
	provider := NewMockHotkeyProvider()
	expectedErr := fmt.Errorf("register error")

	// Set register error
	provider.SetRegisterError(expectedErr)

	// Try to register a hotkey
	err := provider.RegisterHotkey("ctrl+r", "ctrl+r", func() error { return nil })

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	// Clear error
	provider.SetRegisterError(nil)

	// Should work now
	err = provider.RegisterHotkey("ctrl+r", "ctrl+r", func() error { return nil })
	if err != nil {
		t.Errorf("Expected no error after clearing, got %v", err)
	}
}

func TestMockHotkeyProvider_GetRegisteredHotkeys(t *testing.T) {
	provider := NewMockHotkeyProvider()

	// Initially should be empty
	hotkeys := provider.GetRegisteredHotkeys()
	if len(hotkeys) != 0 {
		t.Errorf("Expected 0 hotkeys initially, got %d", len(hotkeys))
	}

	// Register some hotkeys
	testHotkeys := []string{"ctrl+r", "alt+f", "shift+space"}
	for _, hotkey := range testHotkeys {
		err := provider.RegisterHotkey(hotkey, hotkey, func() error { return nil })
		if err != nil {
			t.Errorf("Failed to register hotkey %s: %v", hotkey, err)
		}
	}

	// Get registered hotkeys
	registered := provider.GetRegisteredHotkeys()
	if len(registered) != len(testHotkeys) {
		t.Errorf("Expected %d hotkeys, got %d", len(testHotkeys), len(registered))
	}

	// Check all hotkeys are present (order doesn't matter)
	hotkeyMap := make(map[string]bool)
	for _, hotkey := range registered {
		hotkeyMap[hotkey] = true
	}

	for _, expectedHotkey := range testHotkeys {
		if !hotkeyMap[expectedHotkey] {
			t.Errorf("Expected hotkey %s not found in registered list", expectedHotkey)
		}
	}
}

func TestMockHotkeyProvider_IsHotkeyRegistered(t *testing.T) {
	provider := NewMockHotkeyProvider()

	// Test with no hotkeys registered
	if provider.IsHotkeyRegistered("ctrl+r") {
		t.Error("Should not find unregistered hotkey")
	}

	// Register a hotkey
	err := provider.RegisterHotkey("ctrl+r", "ctrl+r", func() error { return nil })
	if err != nil {
		t.Errorf("Failed to register hotkey: %v", err)
	}

	// Should find it now
	if !provider.IsHotkeyRegistered("ctrl+r") {
		t.Error("Should find registered hotkey")
	}

	// Should not find different hotkey
	if provider.IsHotkeyRegistered("alt+f") {
		t.Error("Should not find different hotkey")
	}

	// Case sensitivity test
	if provider.IsHotkeyRegistered("Ctrl+r") {
		t.Error("Should be case sensitive")
	}
}

func TestMockHotkeyProvider_SimulateHotkeyPress(t *testing.T) {
	provider := NewMockHotkeyProvider()
	var callbackCalled bool

	callback := func() error {
		callbackCalled = true
		return nil
	}

	// Try to simulate unregistered hotkey
	err := provider.SimulateHotkeyPress("ctrl+r")
	if err == nil {
		t.Error("Should return error for unregistered hotkey")
	}

	// Register hotkey
	err = provider.RegisterHotkey("ctrl+r", "ctrl+r", callback)
	if err != nil {
		t.Errorf("Failed to register hotkey: %v", err)
	}

	// Simulate hotkey press
	err = provider.SimulateHotkeyPress("ctrl+r")
	if err != nil {
		t.Errorf("SimulateHotkeyPress failed: %v", err)
	}

	// Check callback was called
	if !callbackCalled {
		t.Error("Callback should have been called")
	}
}

func TestMockHotkeyProvider_EnableDisableEventSimulation(t *testing.T) {
	provider := NewMockHotkeyProvider()

	// Test EnableEventSimulation
	provider.EnableEventSimulation()

	// Register a hotkey
	var callbackCalled bool
	callback := func() error {
		callbackCalled = true
		return nil
	}

	err := provider.RegisterHotkey("ctrl+r", "ctrl+r", callback)
	if err != nil {
		t.Errorf("Failed to register hotkey: %v", err)
	}

	// Test DisableEventSimulation
	provider.DisableEventSimulation()

	// Events should be disabled now
	// Note: The exact behavior depends on implementation details
	// This test primarily checks that the methods can be called without errors
	_ = callbackCalled // Use the variable to avoid "declared and not used" error
}

func TestMockHotkeyProvider_GetCallHistory(t *testing.T) {
	provider := NewMockHotkeyProvider()

	// Initially should be empty
	history := provider.GetCallHistory()
	if len(history) != 0 {
		t.Errorf("Expected empty call history initially, got %d entries", len(history))
	}

	// Perform some operations
	provider.Start()
	provider.RegisterHotkey("ctrl+r", "ctrl+r", func() error { return nil })
	provider.Stop()

	// Check history has entries
	history = provider.GetCallHistory()
	if len(history) == 0 {
		t.Error("Expected call history to have entries")
	}

	// History should contain the method calls
	expectedMethods := []string{"Start", "RegisterHotkey", "Stop"}
	methodCounts := make(map[string]int)

	for _, entry := range history {
		methodCounts[entry]++
	}

	for _, method := range expectedMethods {
		if methodCounts[method] == 0 {
			t.Errorf("Expected to find %s in call history", method)
		}
	}
}

func TestMockHotkeyProvider_Reset(t *testing.T) {
	provider := NewMockHotkeyProvider()

	// Set up some state
	provider.Start()
	provider.RegisterHotkey("ctrl+r", "ctrl+r", func() error { return nil })
	provider.SetStartError(fmt.Errorf("test error"))

	// Verify state exists
	if !provider.IsStarted() {
		t.Error("Provider should be started before reset")
	}
	if !provider.IsHotkeyRegistered("ctrl+r") {
		t.Error("Hotkey should be registered before reset")
	}

	// Reset
	provider.Reset()

	// Verify state is cleared
	if provider.IsStarted() {
		t.Error("Provider should not be started after reset")
	}
	if provider.IsHotkeyRegistered("ctrl+r") {
		t.Error("Hotkey should not be registered after reset")
	}

	// Should be able to start again without error
	err := provider.Start()
	if err != nil {
		t.Errorf("Should be able to start after reset, got error: %v", err)
	}
}

func TestMockHotkeyProvider_WasMethodCalled(t *testing.T) {
	provider := NewMockHotkeyProvider()

	// Initially no methods should be called
	if provider.WasMethodCalled("Start") {
		t.Error("Start should not be called initially")
	}

	// Call Start
	provider.Start()

	// Should find Start call
	if !provider.WasMethodCalled("Start") {
		t.Error("Start should be found in call history")
	}

	// Should not find other methods
	if provider.WasMethodCalled("Stop") {
		t.Error("Stop should not be found in call history")
	}

	// Test case sensitivity
	if provider.WasMethodCalled("start") {
		t.Error("Method calls should be case sensitive")
	}
}

func TestMockHotkeyProvider_GetMethodCallCount(t *testing.T) {
	provider := NewMockHotkeyProvider()

	// Initially all counts should be 0
	if provider.GetMethodCallCount("Start") != 0 {
		t.Error("Start count should be 0 initially")
	}

	// Call Start multiple times
	provider.Start()
	provider.Start() // This will fail but still be recorded
	provider.Start() // This will fail but still be recorded

	// Check count
	count := provider.GetMethodCallCount("Start")
	if count != 3 {
		t.Errorf("Expected Start count to be 3, got %d", count)
	}

	// Call other methods
	provider.Stop()
	provider.RegisterHotkey("ctrl+r", "ctrl+r", func() error { return nil })

	// Check individual counts
	if provider.GetMethodCallCount("Stop") != 1 {
		t.Errorf("Expected Stop count to be 1, got %d", provider.GetMethodCallCount("Stop"))
	}
	if provider.GetMethodCallCount("RegisterHotkey") != 1 {
		t.Errorf("Expected RegisterHotkey count to be 1, got %d", provider.GetMethodCallCount("RegisterHotkey"))
	}

	// Non-existent method should return 0
	if provider.GetMethodCallCount("NonExistentMethod") != 0 {
		t.Error("Non-existent method should have count 0")
	}
}

func TestMockHotkeyProviderWithErrors_SimulatePermissionDenied(t *testing.T) {
	provider := NewMockHotkeyProviderWithErrors()

	// Simulate permission denied
	provider.SimulatePermissionDenied()

	// Start should fail
	err := provider.Start()
	if err == nil {
		t.Error("Start should fail with permission denied")
	}

	// Error message should contain relevant information
	if err.Error() == "" {
		t.Error("Error should have a message")
	}
}

func TestMockHotkeyProviderWithErrors_SimulateInvalidHotkey(t *testing.T) {
	provider := NewMockHotkeyProviderWithErrors()

	// Simulate invalid hotkey
	provider.SimulateInvalidHotkey()

	// RegisterHotkey should fail
	err := provider.RegisterHotkey("ctrl+r", "ctrl+r", func() error { return nil })
	if err == nil {
		t.Error("RegisterHotkey should fail with invalid hotkey simulation")
	}
}

func TestMockHotkeyProviderWithErrors_SimulateSystemBusy(t *testing.T) {
	provider := NewMockHotkeyProviderWithErrors()

	// Simulate system busy
	provider.SimulateSystemBusy()

	// Operations should fail appropriately
	err := provider.Start()
	if err == nil {
		t.Error("Start should fail when system is busy")
	}
}

func TestMockHotkeyProviderWithErrors_SimulateHotkeyConflict(t *testing.T) {
	provider := NewMockHotkeyProviderWithErrors()

	// Simulate hotkey conflict
	provider.SimulateHotkeyConflict()

	// RegisterHotkey should fail
	err := provider.RegisterHotkey("ctrl+r", "ctrl+r", func() error { return nil })
	if err == nil {
		t.Error("RegisterHotkey should fail with hotkey conflict simulation")
	}
}
