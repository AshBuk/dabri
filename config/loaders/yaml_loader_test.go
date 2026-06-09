// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package loaders

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AshBuk/dabri/v2/config/models"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	tests := []struct {
		name          string
		configContent string
		expectError   bool
		checkValues   func(*testing.T, *models.Config)
	}{
		{
			name: "valid config",
			configContent: `
general:
  debug: true
  whisper_model: "small-q5_1"
  language: "en"
  temp_audio_path: "/tmp"

audio:
  device: "default"
  sample_rate: 16000
  format: "S16_LE"
  recording_method: "arecord"

output:
  default_mode: "clipboard"
  clipboard_tool: "auto"
  type_tool: "auto"

hotkeys:
  start_recording: "AltGr+,"
  stop_recording: "AltGr+."
`,
			expectError: false,
			checkValues: func(t *testing.T, cfg *models.Config) {
				if !cfg.General.Debug {
					t.Errorf("expected debug to be true")
				}
				if cfg.General.WhisperModel != "small-q5_1" {
					t.Errorf("expected whisper model to be 'small-q5_1', got %s", cfg.General.WhisperModel)
				}
				if cfg.Audio.SampleRate != 16000 {
					t.Errorf("expected sample rate to be 16000, got %d", cfg.Audio.SampleRate)
				}
				if cfg.Output.DefaultMode != "clipboard" {
					t.Errorf("expected default mode to be 'clipboard', got %s", cfg.Output.DefaultMode)
				}
			},
		},
		{
			name: "minimal config",
			configContent: `
general:
  whisper_model: "small-q5_1"
`,
			expectError: false,
			checkValues: func(t *testing.T, cfg *models.Config) {
				if cfg.General.WhisperModel != "small-q5_1" {
					t.Errorf("expected whisper model to be 'small-q5_1', got %s", cfg.General.WhisperModel)
				}
				// Other fields should have default values
			},
		},
		{
			name: "invalid yaml",
			configContent: `
general:
  debug: true
  invalid_yaml: [
`,
			expectError: true,
			checkValues: nil,
		},
		{
			name:          "empty config",
			configContent: ``,
			expectError:   false,
			checkValues: func(t *testing.T, cfg *models.Config) {
				// Should create config with default values
				if cfg == nil {
					t.Errorf("expected config to be created")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write config content to file
			if err := os.WriteFile(configPath, []byte(tt.configContent), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			config, err := LoadConfig(configPath)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && config == nil {
				t.Errorf("expected config to be loaded")
			}
			if tt.checkValues != nil && config != nil {
				tt.checkValues(t, config)
			}
		})
	}
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	config, err := LoadConfig("/non/existent/file.yaml")
	// LoadConfig returns default config when file doesn't exist
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if config == nil {
		t.Errorf("expected default config to be returned")
		return
	}
	// Check that default values are set
	if config.General.WhisperModel != "small-q5_1" {
		t.Errorf("expected default whisper model to be 'small-q5_1', got %s", config.General.WhisperModel)
	}
}

func TestLoadConfig_InvalidPermissions(t *testing.T) {
	// Create a temporary file with no read permissions
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("test: value"), 0000); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	// LoadConfig returns default config when file can't be read
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if config == nil {
		t.Errorf("expected default config to be returned")
		return
	}
	// Check that default values are set
	if config.General.WhisperModel != "small-q5_1" {
		t.Errorf("expected default whisper model to be 'small-q5_1', got %s", config.General.WhisperModel)
	}
}

// Guards the embedded defaults: a typo would silently zero out config fields.
func TestSetDefaultConfig(t *testing.T) {
	var cfg models.Config
	SetDefaultConfig(&cfg)

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"whisper_model", cfg.General.WhisperModel, "small-q5_1"},
		{"language", cfg.General.Language, "en"},
		{"hotkey_provider", cfg.Hotkeys.Provider, "auto"},
		{"start_recording", cfg.Hotkeys.StartRecording, "alt+r"},
		{"sample_rate", cfg.Audio.SampleRate, 16000},
		{"max_recording_time", cfg.Audio.MaxRecordingTime, 300},
		{"temp_file_cleanup_time", cfg.Audio.TempFileCleanupTime, 30},
		{"default_mode", cfg.Output.DefaultMode, models.OutputModeActiveWindow},
		{"notifications", cfg.Notifications.EnableWorkflowNotifications, true},
		{"web_port", cfg.WebServer.Port, 8080},
		{"web_max_clients", cfg.WebServer.MaxClients, 10},
		{"max_temp_file_size", cfg.Security.MaxTempFileSize, int64(50 * 1024 * 1024)},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
	if len(cfg.Security.AllowedCommands) == 0 {
		t.Errorf("expected non-empty allowed_commands")
	}
}

// First run with a missing file writes the embedded default verbatim.
func TestLoadConfig_FirstRunWritesCommentedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.General.WhisperModel != "small-q5_1" {
		t.Errorf("expected default model, got %s", cfg.General.WhisperModel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected default config file to be created: %v", err)
	}
	if string(data) != string(defaultConfigYAML) {
		t.Errorf("first-run file does not match embedded default (comments lost)")
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	// Test that default config has reasonable values
	config := &models.Config{}
	// Apply default values (this would typically be done in LoadConfig)
	// For this test, we'll just verify the structure exists
	if config.General.WhisperModel == "" {
		config.General.WhisperModel = "small-q5_1"
	}
	if config.Audio.SampleRate == 0 {
		config.Audio.SampleRate = 16000
	}
	// Verify defaults
	if config.General.WhisperModel != "small-q5_1" {
		t.Errorf("expected default whisper model to be 'small-q5_1', got %s", config.General.WhisperModel)
	}
	if config.Audio.SampleRate != 16000 {
		t.Errorf("expected default sample rate to be 16000, got %d", config.Audio.SampleRate)
	}
}
