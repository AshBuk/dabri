// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/config/models"
	"github.com/AshBuk/dabri/v2/internal/testutils"
)

func createTestConfig() *models.Config {
	cfg := &models.Config{}
	cfg.General.Language = "en"
	cfg.General.Model = "small-q5_1"
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
	if service.GetConfig() != testConfig {
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
	if service.GetConfig().General.Language != "ru" {
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
	if !service.GetConfig().Notifications.EnableWorkflowNotifications {
		t.Error("Initial workflow notifications state should be true")
	}

	// Toggle to false
	if err := service.ToggleWorkflowNotifications(); err != nil {
		t.Errorf("ToggleWorkflowNotifications failed: %v", err)
	}
	if service.GetConfig().Notifications.EnableWorkflowNotifications {
		t.Error("Workflow notifications should be toggled to false")
	}

	// Toggle back to true
	if err := service.ToggleWorkflowNotifications(); err != nil {
		t.Errorf("ToggleWorkflowNotifications failed: %v", err)
	}
	if !service.GetConfig().Notifications.EnableWorkflowNotifications {
		t.Error("Workflow notifications should be toggled to true")
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
	testConfig.General.Language = "fr"

	if err := service.ResetToDefaults(); err != nil {
		t.Errorf("ResetToDefaults failed: %v", err)
	}
	// Verify settings were reset to defaults
	if service.GetConfig().General.Language != "en" {
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

func TestConfigService_UpdateKeepsPublishedSnapshot(t *testing.T) {
	service := NewConfigService(testutils.NewMockLogger(), createTestConfig(), filepath.Join(t.TempDir(), "config.yaml"))
	before := service.GetConfig()
	lang := before.General.Language

	if err := service.UpdateLanguage("de"); err != nil {
		t.Fatalf("UpdateLanguage failed: %v", err)
	}
	if before.General.Language != lang {
		t.Error("update mutated a published snapshot")
	}
	if service.GetConfig().General.Language != "de" {
		t.Error("update was not published")
	}
}

func TestConfigService_FailedSaveKeepsConfig(t *testing.T) {
	service := NewConfigService(testutils.NewMockLogger(), createTestConfig(), "")
	before := service.GetConfig()

	if err := service.UpdateLanguage("de"); err == nil {
		t.Fatal("UpdateLanguage should fail without a config path")
	}
	if service.GetConfig() != before {
		t.Error("failed save published a new config")
	}
}

func TestConfigService_ConcurrentUpdatesAndReads(t *testing.T) {
	service := NewConfigService(testutils.NewMockLogger(), createTestConfig(), filepath.Join(t.TempDir(), "config.yaml"))
	initial := service.GetConfig().Notifications.EnableWorkflowNotifications

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { _ = service.ToggleWorkflowNotifications() })
		wg.Go(func() { _ = service.GetConfig().Notifications.EnableWorkflowNotifications })
	}
	wg.Wait()
	// An even number of serialized toggles restores the initial value
	if service.GetConfig().Notifications.EnableWorkflowNotifications != initial {
		t.Error("concurrent toggles lost an update")
	}
}
