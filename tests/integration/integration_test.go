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
	"time"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/testutils"
	"github.com/AshBuk/dabri/v2/stt"
)

// Integration tests for complete user scenarios
// Run with: go test -tags=integration ./tests/integration/...

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

func TestModelManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test bundled model path resolution
	t.Run("bundled_model_resolution", func(t *testing.T) {
		cfg := &config.Config{}
		config.SetDefaultConfig(cfg)

		// Test ModelManager's bundled model resolution
		modelManager := stt.NewModelManager(testutils.StaticConfig(cfg))

		// This should always return a path (even if file doesn't exist in dev)
		// A short deadline keeps a missing model from starting a full download
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		modelPath, err := modelManager.GetModelPath(ctx)

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
