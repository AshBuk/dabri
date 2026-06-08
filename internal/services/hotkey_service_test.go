// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"errors"
	"testing"

	"github.com/AshBuk/dabri/v2/internal/testutils"
	"github.com/AshBuk/dabri/v2/tests/mocks"
)

func TestHotkeyService_NewHotkeyService(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	mockManager := &mocks.MockHotkeyManager{}

	service := NewHotkeyService(mockLogger, mockManager)
	if service == nil {
		t.Fatal("NewHotkeyService returned nil")
	}
	if service.logger != mockLogger {
		t.Error("Logger not set correctly")
	}
	if service.hotkeyManager != mockManager {
		t.Error("Hotkey manager not set correctly")
	}
}

func TestHotkeyService_RegisterHotkeys_Success(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	mockManager := &mocks.MockHotkeyManager{}

	service := NewHotkeyService(mockLogger, mockManager)
	if err := service.RegisterHotkeys(); err != nil {
		t.Errorf("RegisterHotkeys failed: %v", err)
	}
	if !mockManager.WasStartCalled() {
		t.Error("Hotkey manager Start method was not called")
	}
}

func TestHotkeyService_RegisterHotkeys_Error(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	expectedError := errors.New("hotkey registration failed")
	mockManager := &mocks.MockHotkeyManager{}
	mockManager.SetStartError(expectedError)

	service := NewHotkeyService(mockLogger, mockManager)

	if err := service.RegisterHotkeys(); err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestHotkeyService_RegisterHotkeys_NoManager(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	service := NewHotkeyService(mockLogger, nil)

	if err := service.RegisterHotkeys(); err == nil {
		t.Error("RegisterHotkeys should fail when hotkey manager is nil")
	}
}

func TestHotkeyService_UnregisterHotkeys_Success(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	mockManager := &mocks.MockHotkeyManager{}

	service := NewHotkeyService(mockLogger, mockManager)
	if err := service.UnregisterHotkeys(); err != nil {
		t.Errorf("UnregisterHotkeys failed: %v", err)
	}
	if !mockManager.WasStopCalled() {
		t.Error("Hotkey manager Stop method was not called")
	}
}

func TestHotkeyService_UnregisterHotkeys_NoManager(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	service := NewHotkeyService(mockLogger, nil)

	if err := service.UnregisterHotkeys(); err != nil {
		t.Errorf("UnregisterHotkeys should not fail when hotkey manager is nil: %v", err)
	}
}

func TestHotkeyService_SetupHotkeyCallbacks_Success(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	mockManager := &mocks.MockHotkeyManager{}
	service := NewHotkeyService(mockLogger, mockManager)

	// Create test callback functions
	var callbacksCalled []string
	toggleRecording := func() error { callbacksCalled = append(callbacksCalled, "toggleRecording"); return nil }
	showConfig := func() error { callbacksCalled = append(callbacksCalled, "showConfig"); return nil }
	reloadConfig := func() error { callbacksCalled = append(callbacksCalled, "reloadConfig"); return nil }
	err := service.SetupHotkeyCallbacks(
		toggleRecording,
		showConfig, reloadConfig,
	)
	if err != nil {
		t.Errorf("SetupHotkeyCallbacks failed: %v", err)
	}

	// Verify the recording toggle was registered
	if !mockManager.WereCallbacksRegistered() {
		t.Error("RegisterToggle was not called")
	}
	// Verify all hotkey actions were registered
	expectedActions := []string{"show_config", "reset_to_defaults"}
	for _, action := range expectedActions {
		if !mockManager.WasHotkeyActionRegistered(action) {
			t.Errorf("RegisterHotkeyAction was not called for action: %s", action)
		}
	}
	// Test that callbacks work
	mockManager.TriggerCallback("toggleRecording")
	mockManager.TriggerCallback("showConfig")

	if len(callbacksCalled) != 2 {
		t.Errorf("Expected 2 callbacks to be called, got %d", len(callbacksCalled))
	}
	if callbacksCalled[0] != "toggleRecording" || callbacksCalled[1] != "showConfig" {
		t.Errorf("Unexpected callbacks called: %v", callbacksCalled)
	}
}

func TestHotkeyService_SetupHotkeyCallbacks_NoManager(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	service := NewHotkeyService(mockLogger, nil)

	err := service.SetupHotkeyCallbacks(
		func() error { return nil },
		func() error { return nil },
		func() error { return nil },
	)
	if err == nil {
		t.Error("SetupHotkeyCallbacks should fail when hotkey manager is nil")
	}
}

func TestHotkeyService_Shutdown_Success(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	mockManager := &mocks.MockHotkeyManager{}
	service := NewHotkeyService(mockLogger, mockManager)

	if err := service.Shutdown(); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	if !mockManager.WasStopCalled() {
		t.Error("Hotkey manager Stop method was not called during shutdown")
	}
}

func TestHotkeyService_Shutdown_NoManager(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	service := NewHotkeyService(mockLogger, nil)

	if err := service.Shutdown(); err != nil {
		t.Errorf("Shutdown should not fail when hotkey manager is nil: %v", err)
	}
}
