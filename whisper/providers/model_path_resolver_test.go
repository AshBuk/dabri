// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
)

const testModelFileName = "ggml-test-model.bin"

func TestGetBundledModelPath_AppImage(t *testing.T) {
	// Create temp directory structure for AppImage
	appDir := t.TempDir()
	modelDir := filepath.Join(appDir, "sources", "language-models")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("failed to create model dir: %v", err)
	}
	modelPath := filepath.Join(modelDir, testModelFileName)
	if err := os.WriteFile(modelPath, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create model file: %v", err)
	}

	// Set APPDIR environment variable
	t.Setenv("APPDIR", appDir)

	resolver := NewModelPathResolver(&config.Config{}, testModelFileName)
	result := resolver.GetBundledModelPath()
	expected := filepath.Join(appDir, "sources/language-models", testModelFileName)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetBundledModelPath_UserData(t *testing.T) {
	// Clear APPDIR to skip AppImage check
	t.Setenv("APPDIR", "")

	// Create temp XDG_DATA_HOME
	dataHome := t.TempDir()
	modelDir := filepath.Join(dataHome, AppDataDirName, ModelsDirName)
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("failed to create model dir: %v", err)
	}
	modelPath := filepath.Join(modelDir, testModelFileName)
	if err := os.WriteFile(modelPath, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create model file: %v", err)
	}

	t.Setenv("XDG_DATA_HOME", dataHome)

	resolver := NewModelPathResolver(&config.Config{}, testModelFileName)
	result := resolver.GetBundledModelPath()
	if result != modelPath {
		t.Errorf("expected %q, got %q", modelPath, result)
	}
}

func TestGetBundledModelPath_DevPath(t *testing.T) {
	// Clear environment variables
	t.Setenv("APPDIR", "")
	t.Setenv("XDG_DATA_HOME", "/nonexistent")

	// Create dev path in temp directory and change to it
	tempDir := t.TempDir()
	devModelDir := filepath.Join(tempDir, "sources", "language-models")
	if err := os.MkdirAll(devModelDir, 0o755); err != nil {
		t.Fatalf("failed to create dev model dir: %v", err)
	}
	devModelPath := filepath.Join(devModelDir, testModelFileName)
	if err := os.WriteFile(devModelPath, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create model file: %v", err)
	}

	// Change to temp directory so relative path works
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	resolver := NewModelPathResolver(&config.Config{}, testModelFileName)
	result := resolver.GetBundledModelPath()
	expected := filepath.Join("sources/language-models", testModelFileName)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetBundledModelPath_FallbackToUserData(t *testing.T) {
	// Clear environment - no model exists anywhere
	t.Setenv("APPDIR", "")
	dataHome := t.TempDir() // Empty directory
	t.Setenv("XDG_DATA_HOME", dataHome)

	resolver := NewModelPathResolver(&config.Config{}, testModelFileName)
	result := resolver.GetBundledModelPath()
	// Should return user data path as default download location
	expected := filepath.Join(dataHome, AppDataDirName, ModelsDirName, testModelFileName)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetUserDataModelPath(t *testing.T) {
	dataHome := "/custom/data/home"
	t.Setenv("XDG_DATA_HOME", dataHome)

	resolver := NewModelPathResolver(&config.Config{}, testModelFileName)
	result := resolver.GetUserDataModelPath()
	expected := filepath.Join(dataHome, AppDataDirName, ModelsDirName, testModelFileName)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetUserDataModelPath_DefaultHome(t *testing.T) {
	// Clear XDG_DATA_HOME to use default
	t.Setenv("XDG_DATA_HOME", "")

	resolver := NewModelPathResolver(&config.Config{}, testModelFileName)
	result := resolver.GetUserDataModelPath()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share", AppDataDirName, ModelsDirName, testModelFileName)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFileExists(t *testing.T) {
	// Create temp file
	tempFile, err := os.CreateTemp("", "test_model")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"existing file", tempFile.Name(), true},
		{"non-existing file", "/nonexistent/path/file.bin", false},
		{"directory", os.TempDir(), false},
		{"empty path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileExists(tt.path)
			if result != tt.expected {
				t.Errorf("fileExists(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestModelPathResolver_Priority(t *testing.T) {
	// Test that AppImage path takes priority over user data path
	appDir := t.TempDir()
	dataHome := t.TempDir()

	// Create model in both locations
	appImageModelDir := filepath.Join(appDir, "sources", "language-models")
	userModelDir := filepath.Join(dataHome, AppDataDirName, ModelsDirName)

	for _, dir := range []string{appImageModelDir, userModelDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		modelPath := filepath.Join(dir, testModelFileName)
		if err := os.WriteFile(modelPath, []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to create model file: %v", err)
		}
	}

	t.Setenv("APPDIR", appDir)
	t.Setenv("XDG_DATA_HOME", dataHome)

	resolver := NewModelPathResolver(&config.Config{}, testModelFileName)
	result := resolver.GetBundledModelPath()
	// AppImage should take priority
	expected := filepath.Join(appDir, "sources/language-models", testModelFileName)
	if result != expected {
		t.Errorf("AppImage path should take priority: expected %q, got %q", expected, result)
	}
}
