// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"errors"
	"testing"

	"github.com/AshBuk/dabri/v2/tests/mocks"
)

// TestServiceContainer tests the ServiceContainer basic functionality
func TestServiceContainer(t *testing.T) {
	t.Run("NewServiceContainer", func(t *testing.T) {
		container := NewServiceContainer()
		if container == nil {
			t.Fatal("NewServiceContainer returned nil")
		}

		if container.Audio != nil {
			t.Error("Audio service should be nil initially")
		}
		if container.UI != nil {
			t.Error("UI service should be nil initially")
		}
		if container.IO != nil {
			t.Error("IO service should be nil initially")
		}
		if container.Config != nil {
			t.Error("Config service should be nil initially")
		}
		if container.Hotkeys != nil {
			t.Error("Hotkeys service should be nil initially")
		}
	})

	t.Run("Shutdown_EmptyContainer", func(t *testing.T) {
		container := NewServiceContainer()
		err := container.Shutdown()
		if err != nil {
			t.Errorf("Shutdown of empty container should not error, got: %v", err)
		}
	})
}

func TestServiceContainer_Shutdown(t *testing.T) {
	t.Run("Shutdown_AllServicesSuccess", func(t *testing.T) {
		container := NewServiceContainer()
		// Create mock services
		audioMock := &mocks.MockAudioService{}
		uiMock := &mocks.MockUIService{}
		ioMock := &mocks.MockIOService{}
		configMock := &mocks.MockConfigService{}
		hotkeyMock := &mocks.MockHotkeyService{}
		// Assign mock services
		container.Audio = audioMock
		container.UI = uiMock
		container.IO = ioMock
		container.Config = configMock
		container.Hotkeys = hotkeyMock

		err := container.Shutdown()
		if err != nil {
			t.Errorf("Shutdown should succeed, got error: %v", err)
		}
		// Verify all shutdown methods were called
		if !audioMock.WasShutdownCalled() {
			t.Error("Audio service shutdown was not called")
		}
		if !uiMock.WasShutdownCalled() {
			t.Error("UI service shutdown was not called")
		}
		if !ioMock.WasShutdownCalled() {
			t.Error("IO service shutdown was not called")
		}
		if !configMock.WasShutdownCalled() {
			t.Error("Config service shutdown was not called")
		}
		if !hotkeyMock.WasShutdownCalled() {
			t.Error("Hotkey service shutdown was not called")
		}
	})

	t.Run("Shutdown_ServiceError", func(t *testing.T) {
		container := NewServiceContainer()
		expectedError := errors.New("shutdown failed")
		audioMock := &mocks.MockAudioService{}
		audioMock.SetShutdownError(expectedError)
		uiMock := &mocks.MockUIService{}

		container.Audio = audioMock
		container.UI = uiMock

		err := container.Shutdown()
		if err == nil {
			t.Error("Shutdown should return error when service fails")
		}
		if !errors.Is(err, expectedError) {
			t.Errorf("Expected error to contain %v, got %v", expectedError, err)
		}
		// Both services should still be called even if one fails
		if !audioMock.WasShutdownCalled() {
			t.Error("Audio service shutdown was not called")
		}
		if !uiMock.WasShutdownCalled() {
			t.Error("UI service shutdown was not called")
		}
	})
}
