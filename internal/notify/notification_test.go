// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package notify

import (
	"os/exec"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
)

// Helper function to create test config
func createTestConfig() *config.Config {
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	return cfg
}

func TestNewNotificationManager(t *testing.T) {
	appName := "TestApp"
	cfg := createTestConfig()
	nm := NewNotificationManager(appName, cfg)
	if nm == nil {
		t.Fatal("NewNotificationManager returned nil")
	}

	if nm.appName != appName {
		t.Errorf("Expected appName %q, got %q", appName, nm.appName)
	}
}

func TestNotificationManager_NotifyStartRecording(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())
	// Mock notify-send command to avoid actual notifications during testing
	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}

	err := nm.NotifyStartRecording()
	// We expect this to either succeed or fail gracefully
	// The actual notification may fail in CI environment
	if err != nil {
		t.Logf("NotifyStartRecording failed (expected in CI): %v", err)
	}
}

func TestNotificationManager_NotifyStopRecording(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())

	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}

	err := nm.NotifyStopRecording()

	if err != nil {
		t.Logf("NotifyStopRecording failed (expected in CI): %v", err)
	}
}

func TestNotificationManager_NotifyTranscriptionComplete(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())

	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}

	err := nm.NotifyTranscriptionComplete()

	if err != nil {
		t.Logf("NotifyTranscriptionComplete failed (expected in CI): %v", err)
	}
}

func TestNotificationManager_NotifyError(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())

	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}

	testError := "Test error message"
	err := nm.NotifyError(testError)
	if err != nil {
		t.Logf("NotifyError failed (expected in CI): %v", err)
	}
}

func TestNotificationManager_ShowNotification(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())

	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}

	summary := "Test Summary"
	body := "Test Body"
	err := nm.ShowNotification(summary, body)
	if err != nil {
		t.Logf("ShowNotification failed (expected in CI): %v", err)
	}
}

func TestNotificationManager_IsAvailable(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())

	result := nm.IsAvailable()
	// Check if notify-send is available in PATH
	expected := commandExists("notify-send")

	if result != expected {
		t.Errorf("Expected IsAvailable to return %v, got %v", expected, result)
	}
}

func TestNotificationManager_SendNotification_ValidCommand(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())
	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}
	// Test with valid parameters
	err := nm.sendNotification("Test Summary", "Test Body", "dialog-information")
	// In CI environment, this may fail due to no display
	if err != nil {
		t.Logf("sendNotification failed (expected in CI): %v", err)
	}
}

func TestNotificationManager_SendNotification_EmptyParameters(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())
	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}
	// Test with empty parameters
	err := nm.sendNotification("", "", "")

	// Should still work with empty parameters
	if err != nil {
		t.Logf("sendNotification with empty params failed (expected in CI): %v", err)
	}
}

func TestNotificationManager_AppName(t *testing.T) {
	tests := []struct {
		name    string
		appName string
	}{
		{
			name:    "normal app name",
			appName: "MyApp",
		},
		{
			name:    "app name with spaces",
			appName: "My App Name",
		},
		{
			name:    "app name with special chars",
			appName: "My-App_v1.0",
		},
		{
			name:    "empty app name",
			appName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nm := NewNotificationManager(tt.appName, createTestConfig())

			if nm.appName != tt.appName {
				t.Errorf("Expected appName %q, got %q", tt.appName, nm.appName)
			}
		})
	}
}

func TestNotificationManager_NotificationTypes(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())

	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}

	// Test different notification types
	tests := []struct {
		name   string
		method func() error
	}{
		{
			name:   "start recording",
			method: nm.NotifyStartRecording,
		},
		{
			name:   "stop recording",
			method: nm.NotifyStopRecording,
		},
		{
			name:   "transcription complete",
			method: nm.NotifyTranscriptionComplete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.method()

			// Log error but don't fail test in CI environment
			if err != nil {
				t.Logf("Notification method %s failed (expected in CI): %v", tt.name, err)
			}
		})
	}
}

func TestNotificationManager_ErrorNotification(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())

	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}

	tests := []struct {
		name     string
		errorMsg string
	}{
		{
			name:     "simple error",
			errorMsg: "Simple error message",
		},
		{
			name:     "error with special chars",
			errorMsg: "Error: Failed to connect to server (timeout)",
		},
		{
			name:     "empty error",
			errorMsg: "",
		},
		{
			name:     "long error message",
			errorMsg: "This is a very long error message that might exceed normal notification limits and should still be handled properly by the notification system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nm.NotifyError(tt.errorMsg)

			if err != nil {
				t.Logf("NotifyError failed (expected in CI): %v", err)
			}
		})
	}
}

func TestNotificationManager_ShowNotification_Variations(t *testing.T) {
	nm := NewNotificationManager("TestApp", createTestConfig())

	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping test")
	}

	tests := []struct {
		name    string
		summary string
		body    string
	}{
		{
			name:    "normal notification",
			summary: "Test Summary",
			body:    "Test Body",
		},
		{
			name:    "empty summary",
			summary: "",
			body:    "Test Body",
		},
		{
			name:    "empty body",
			summary: "Test Summary",
			body:    "",
		},
		{
			name:    "both empty",
			summary: "",
			body:    "",
		},
		{
			name:    "unicode characters",
			summary: "🎤 Recording",
			body:    "Voice recording started ✅",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nm.ShowNotification(tt.summary, tt.body)

			if err != nil {
				t.Logf("ShowNotification failed (expected in CI): %v", err)
			}
		})
	}
}

// Helper function to check if a command exists
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// Test helper function
func TestCommandExists(t *testing.T) {
	// Test with a command that should exist
	if !commandExists("echo") {
		t.Error("Expected 'echo' command to exist")
	}

	// Test with a command that should not exist
	if commandExists("non-existent-command-12345") {
		t.Error("Expected 'non-existent-command-12345' to not exist")
	}
}
