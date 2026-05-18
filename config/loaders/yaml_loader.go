// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package loaders

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AshBuk/dabri/v2/config/models"
	"github.com/AshBuk/dabri/v2/config/validators"
	"github.com/AshBuk/dabri/v2/internal/logger"
	yaml "gopkg.in/yaml.v2"
)

// Read a configuration file, apply defaults, and validate the result.
// If the file doesn't exist, log a warning and return a default configuration.
// The process is: 1. Apply defaults. 2. Read file. 3. Unmarshal YAML. 4. Validate
func LoadConfig(filename string, loggers ...logger.Logger) (*models.Config, error) {
	var logSink logger.Logger = logger.NewDefaultLogger(logger.WarningLevel)
	if len(loggers) > 0 && loggers[0] != nil {
		logSink = loggers[0]
	}
	var config models.Config
	// Start with a default configuration to ensure all fields are initialized
	SetDefaultConfig(&config)

	// Sanitize path to prevent directory traversal attacks
	clean := filepath.Clean(filename)
	if strings.Contains(clean, "..") {
		return nil, fmt.Errorf("invalid config path: %s", filename)
	}
	// #nosec G304 -- Path is cleaned and validated, mitigating directory traversal risks.
	data, err := os.ReadFile(clean)
	if err != nil {
		logSink.Info("Could not read config file: %v, using defaults", err)
		return &config, nil
	}
	// Parse the YAML content into the config struct
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	// Validate the loaded configuration and apply corrections if necessary
	if err := validators.ValidateConfig(&config); err != nil {
		logSink.Warning("Configuration validation error: %v", err)
		logSink.Info("Using validated configuration with corrections")
	}

	return &config, nil
}

// Apply sensible, safe-by-default values to a configuration struct.
// These defaults are used when a configuration file is not found or a field is missing
func SetDefaultConfig(config *models.Config) {
	// General settings
	config.General.Debug = false
	config.General.WhisperModel = "small-q5_1" // Fixed whisper model
	config.General.TempAudioPath = "/tmp"
	config.General.Language = "en" // Default to English

	// Hotkey settings
	config.Hotkeys.Provider = "auto"
	config.Hotkeys.StartRecording = "alt+r"  // Start/stop recording
	config.Hotkeys.StopRecording = "alt+r"   // Same combination for start/stop
	config.Hotkeys.ShowConfig = "alt+c"      // Show config
	config.Hotkeys.ResetToDefaults = "alt+d" // Reset to defaults

	// Audio settings
	config.Audio.Device = "default"
	config.Audio.SampleRate = 16000
	config.Audio.Format = "s16le"
	config.Audio.RecordingMethod = "arecord"
	config.Audio.ExpectedDuration = 0     // No expected duration by default
	config.Audio.MaxRecordingTime = 300   // 5 minutes max by default
	config.Audio.TempFileCleanupTime = 30 // 30 minutes cleanup timeout by default

	// Output settings
	config.Output.DefaultMode = models.OutputModeActiveWindow
	config.Output.ClipboardTool = "auto" // auto-detect
	config.Output.TypeTool = "auto"      // auto-detect

	// Notification settings
	config.Notifications.EnableWorkflowNotifications = true // Enable workflow notifications by default

	// Web server settings (disabled by default for security)
	config.WebServer.Enabled = false
	config.WebServer.Port = 8080
	config.WebServer.Host = "localhost"
	config.WebServer.AuthToken = "" // No auth by default
	config.WebServer.APIVersion = "v1"
	config.WebServer.LogRequests = true
	config.WebServer.CORSOrigins = "*" // Allow all origins by default
	config.WebServer.MaxClients = 10

	// Security settings
	config.Security.AllowedCommands = []string{"arecord", "ffmpeg", "whisper", "xdotool", "wtype", "ydotool", "wl-copy", "wl-paste", "xsel", "notify-send", "xdg-open"}
	config.Security.CheckIntegrity = false
	config.Security.ConfigHash = ""
	config.Security.MaxTempFileSize = 50 * 1024 * 1024 // 50MB by default
}

// Marshal the configuration to YAML and write it to a file.
// It ensures the target directory exists and sets restrictive file permissions (0600)
// for security
func SaveConfig(filename string, config *models.Config) error {
	// Sanitize path to prevent directory traversal
	safe := filepath.Clean(filename)
	if strings.Contains(safe, "..") {
		return fmt.Errorf("invalid config path: %s", filename)
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	// Ensure the directory exists before writing the file
	if err := os.MkdirAll(filepath.Dir(safe), 0o750); err != nil {
		return err
	}
	// Write with restrictive permissions (read/write for owner only)
	return os.WriteFile(safe, data, 0o600)
}
