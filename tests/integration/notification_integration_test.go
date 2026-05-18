// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

//go:build integration

package integration

import (
	"os"
	"os/exec"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/notify"
)

// Helper function to create test config
func createTestConfig() *config.Config {
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	return cfg
}

// Helper function to check if a command exists
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func TestNotificationManager_Integration(t *testing.T) {
	// Skip this test in CI environment
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI environment")
	}

	// Skip this test in Docker environment (no GUI available)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		t.Skip("Skipping integration test in Docker environment")
	}

	// Skip this test if no DISPLAY is available (headless environment)
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("Skipping integration test: no GUI display available")
	}

	if !commandExists("notify-send") {
		t.Skip("notify-send not available, skipping integration test")
	}

	nm := notify.NewNotificationManager("Integration Test", createTestConfig())
	// Test a sequence of notifications
	err := nm.NotifyStartRecording()
	if err != nil {
		t.Errorf("Failed to notify start recording: %v", err)
	}

	err = nm.NotifyStopRecording()
	if err != nil {
		t.Errorf("Failed to notify stop recording: %v", err)
	}

	err = nm.NotifyTranscriptionComplete()
	if err != nil {
		t.Errorf("Failed to notify transcription complete: %v", err)
	}

	err = nm.NotifyError("Test error for integration")
	if err != nil {
		t.Errorf("Failed to notify error: %v", err)
	}

	err = nm.ShowNotification("Integration Test", "All notifications completed")
	if err != nil {
		t.Errorf("Failed to show notification: %v", err)
	}
}
