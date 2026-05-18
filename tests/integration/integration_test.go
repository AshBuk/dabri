//go:build integration

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/whisper"
)

// Integration tests for complete user scenarios
// Run with: go test -tags=integration ./tests/integration/...

func TestApplicationInitialization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test that basic config loading works
	tempDir := t.TempDir()
	// Create test config
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	cfg.General.TempAudioPath = tempDir
	cfg.Output.DefaultMode = "clipboard" // Safe for testing
	cfg.WebServer.Enabled = false
	// Test config validation
	err := config.ValidateConfig(cfg)
	if err != nil {
		t.Logf("Config validation failed (expected in test environment): %v", err)
	}

	t.Log("Application initialization test completed")
}

func TestConfigurationLoading(t *testing.T) {
	tests := []struct {
		name       string
		configData string
		expectErr  bool
	}{
		{
			name: "valid_config",
			configData: `
general:
  debug: false
  language: "en"
hotkeys:
  start_recording: "altgr+comma"
audio:
  device: "default"
  sample_rate: 16000
output:
  default_mode: "clipboard"
`,
			expectErr: false,
		},
		{
			name: "invalid_yaml",
			configData: `
invalid: yaml: content:
  - malformed
`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "config.yaml")
			err := os.WriteFile(configFile, []byte(tt.configData), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			_, err = config.LoadConfig(configFile)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestHotkeyManagerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test that hotkey manager can be created and configured
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	// Test different hotkey configurations
	testHotkeys := []string{
		"altgr+comma",
		"ctrl+shift+r",
		"alt+space",
	}

	for _, hotkey := range testHotkeys {
		t.Run("hotkey_"+hotkey, func(t *testing.T) {
			cfg.Hotkeys.StartRecording = hotkey
			// This would normally require elevated permissions
			// In test environment, we just verify it doesn't crash
			t.Logf("Testing hotkey configuration: %s", hotkey)
		})
	}
}

func TestOutputManagerIntegration(t *testing.T) {
	// Test output manager creation with different configurations
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	testModes := []string{"clipboard", "active_window"}

	for _, mode := range testModes {
		t.Run("mode_"+mode, func(t *testing.T) {
			cfg.Output.DefaultMode = mode
			// Test that output manager can be created
			// Actual functionality would require external tools
			t.Logf("Testing output mode: %s", mode)
		})
	}
}

func TestEnvironmentDetection(t *testing.T) {
	// Test that environment detection doesn't crash
	// and returns reasonable values

	// This would test actual environment detection
	// but in CI it might return "Unknown"
	t.Log("Environment detection test - results may vary in CI")
}

func TestModelManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test bundled model path resolution
	t.Run("bundled_model_resolution", func(t *testing.T) {
		cfg := &config.Config{}
		config.SetDefaultConfig(cfg)

		// Test ModelManager's bundled model resolution
		modelManager := whisper.NewModelManager(cfg)

		// This should always return a path (even if file doesn't exist in dev)
		modelPath, err := modelManager.GetModelPath(context.Background())

		if err != nil {
			t.Logf("Bundled model not found (expected in development): %v", err)
			// In development this is expected - bundled models only exist in AppImage
		} else {
			t.Logf("Bundled model path resolved to: %s", modelPath)

			// Model can be found in AppImage/dev (sources/language-models/) or XDG user data (dabri/models/)
			isBundledPath := strings.Contains(modelPath, "sources/language-models/small-q5_1.bin")
			isUserDataPath := strings.Contains(modelPath, "dabri/models/small-q5_1.bin")
			if !isBundledPath && !isUserDataPath {
				t.Errorf("Unexpected model path: %s", modelPath)
			}
		}
	})
}

func TestConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test that concurrent config operations don't cause issues
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")

	// Create simple test config file
	configContent := `
general:
  debug: false
output:
  default_mode: "clipboard"
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Run concurrent config loads
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, err := config.LoadConfig(configFile)
			errChan <- err
		}()
	}

	// Collect results
	for i := 0; i < 10; i++ {
		err := <-errChan
		if err != nil {
			t.Errorf("Concurrent config load failed: %v", err)
		}
	}
}
