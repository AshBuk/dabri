// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/config/models"
	"github.com/AshBuk/dabri/v2/internal/testutils"
)

func createTestConfig() *models.Config {
	cfg := &models.Config{}
	cfg.General.Language = "en"
	cfg.General.WhisperModel = "small-q5_1"
	cfg.Notifications.EnableWorkflowNotifications = true
	return cfg
}

func TestConfigService_NewConfigService(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	testConfig := createTestConfig()

	service := NewConfigService(mockLogger, testConfig, "/test/config.yaml")
	if service == nil {
		t.Fatal("NewConfigService returned nil")
	}
	if service.logger != mockLogger {
		t.Error("Logger not set correctly")
	}
	if service.config != testConfig {
		t.Error("Config not set correctly")
	}
	if service.configFile != "/test/config.yaml" {
		t.Error("Config file path not set correctly")
	}
}

func TestConfigService_GetConfig(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	testConfig := createTestConfig()

	service := NewConfigService(mockLogger, testConfig, "/test/config.yaml")

	result := service.GetConfig()
	if result != testConfig {
		t.Error("GetConfig did not return the correct config")
	}
}

func TestConfigService_UpdateLanguage(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	testConfig := createTestConfig()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	if err := config.SaveConfig(configPath, testConfig); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	service := NewConfigService(mockLogger, testConfig, configPath)
	if err := service.UpdateLanguage("ru"); err != nil {
		t.Errorf("UpdateLanguage failed: %v", err)
	}
	if service.config.General.Language != "ru" {
		t.Error("Language not updated correctly")
	}

	// Test same language (should not error)
	if err := service.UpdateLanguage("ru"); err != nil {
		t.Errorf("UpdateLanguage with same language should not error: %v", err)
	}
}

func TestConfigService_ToggleWorkflowNotifications(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	testConfig := createTestConfig()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	if err := config.SaveConfig(configPath, testConfig); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	service := NewConfigService(mockLogger, testConfig, configPath)
	// Initial state should be true
	if !service.config.Notifications.EnableWorkflowNotifications {
		t.Error("Initial workflow notifications state should be true")
	}

	// Toggle to false
	if err := service.ToggleWorkflowNotifications(); err != nil {
		t.Errorf("ToggleWorkflowNotifications failed: %v", err)
	}
	if service.config.Notifications.EnableWorkflowNotifications {
		t.Error("Workflow notifications should be toggled to false")
	}

	// Toggle back to true
	if err := service.ToggleWorkflowNotifications(); err != nil {
		t.Errorf("ToggleWorkflowNotifications failed: %v", err)
	}
	if !service.config.Notifications.EnableWorkflowNotifications {
		t.Error("Workflow notifications should be toggled to true")
	}
}

func TestConfigService_LoadConfig(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	testConfig := createTestConfig()

	service := NewConfigService(mockLogger, testConfig, "")

	if err := service.LoadConfig("/new/path/config.yaml"); err != nil {
		t.Errorf("LoadConfig failed: %v", err)
	}
	if service.configFile != "/new/path/config.yaml" {
		t.Error("Config file path not updated correctly")
	}
}

func TestConfigService_SaveConfig_NoPath(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	testConfig := createTestConfig()

	service := NewConfigService(mockLogger, testConfig, "")

	if err := service.SaveConfig(); err == nil {
		t.Error("SaveConfig should fail when no config file path is set")
	}
}

func TestConfigService_ResetToDefaults(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	testConfig := createTestConfig()

	// Create temp config file
	tempFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	service := NewConfigService(mockLogger, testConfig, tempFile.Name())
	// Modify some settings first
	service.config.General.Language = "fr"

	if err := service.ResetToDefaults(); err != nil {
		t.Errorf("ResetToDefaults failed: %v", err)
	}
	// Verify settings were reset to defaults
	if service.config.General.Language != "en" {
		t.Error("Language should be reset to default 'en'")
	}
}

func TestConfigService_Shutdown(t *testing.T) {
	mockLogger := testutils.NewMockLogger()
	testConfig := createTestConfig()

	service := NewConfigService(mockLogger, testConfig, "/tmp/test_config.yaml")

	if err := service.Shutdown(); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Shutdown should complete successfully without saving
	// (config changes are saved immediately by their respective methods)
}
