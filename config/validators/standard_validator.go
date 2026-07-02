// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package validators

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AshBuk/dabri/v2/config/models"
	"github.com/AshBuk/dabri/v2/internal/constants"
)

// validateGeneralConfig validates general configuration settings
func validateGeneralConfig(config *models.Config, errors *[]string) {
	// Validate whisper model against known model IDs
	if constants.ModelByID(config.General.WhisperModel) == nil {
		*errors = append(*errors, fmt.Sprintf("invalid whisper model: %s, using '%s'", config.General.WhisperModel, constants.DefaultModelID))
		config.General.WhisperModel = constants.DefaultModelID
	}

	if config.General.TempAudioPath != "" {
		// Sanitize path to prevent directory traversal
		config.General.TempAudioPath = filepath.Clean(config.General.TempAudioPath)
		if strings.Contains(config.General.TempAudioPath, "..") {
			config.General.TempAudioPath = "/tmp"
			*errors = append(*errors, "suspicious temp audio path sanitized to /tmp")
		}
	}
}

// validateAudioConfig validates audio configuration settings
func validateAudioConfig(config *models.Config, errors *[]string) {
	// Audio sample rate must be within a reasonable range for audio processing
	if config.Audio.SampleRate < 8000 || config.Audio.SampleRate > 48000 {
		*errors = append(*errors, fmt.Sprintf("invalid sample rate: %d, correcting to 16000", config.Audio.SampleRate))
		config.Audio.SampleRate = 16000
	}

	// Ensure only supported recording methods are used
	validRecordingMethods := map[string]bool{
		"arecord": true,
		"ffmpeg":  true,
	}
	if !validRecordingMethods[config.Audio.RecordingMethod] {
		*errors = append(*errors, fmt.Sprintf("invalid recording method: %s, correcting to 'arecord'", config.Audio.RecordingMethod))
		config.Audio.RecordingMethod = "arecord"
	}

	// Max recording time is capped to prevent accidental resource exhaustion
	if config.Audio.MaxRecordingTime <= 0 || config.Audio.MaxRecordingTime > 1800 { // 30 minutes
		*errors = append(*errors, fmt.Sprintf("invalid max recording time: %d, correcting to 300s", config.Audio.MaxRecordingTime))
		config.Audio.MaxRecordingTime = 300 // 5 minutes
	}
}

// validateWebServerConfig validates web server configuration settings
func validateWebServerConfig(config *models.Config, errors *[]string) {
	if !config.WebServer.Enabled {
		return
	}
	if config.WebServer.Port <= 0 || config.WebServer.Port > 65535 {
		*errors = append(*errors, fmt.Sprintf("invalid port: %d, correcting to 8080", config.WebServer.Port))
		config.WebServer.Port = 8080
	}
	// Host must be a valid hostname
	if config.WebServer.Host == "" {
		config.WebServer.Host = "localhost"
		return
	}
	// Basic validation to prevent injection of malicious characters
	hostRegex := regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	if !hostRegex.MatchString(config.WebServer.Host) {
		*errors = append(*errors, fmt.Sprintf("invalid host: %s, correcting to 'localhost'", config.WebServer.Host))
		config.WebServer.Host = "localhost"
	}
}

// validateSecurityConfig validates security configuration settings
func validateSecurityConfig(config *models.Config, errors *[]string) {
	// Ensure there's always a baseline of allowed commands for security
	if len(config.Security.AllowedCommands) == 0 {
		config.Security.AllowedCommands = []string{"arecord", "ffmpeg", "whisper", "xdotool", "wl-copy", "wl-paste", "xsel"}
		*errors = append(*errors, "allowed_commands was empty, populated with defaults")
	}
}

// Inspect the configuration for invalid or unsafe values.
// It automatically corrects offending values to safe defaults and returns an error
// that aggregates all validation issues found. This ensures the application can
// always run with a sane configuration
func ValidateConfig(config *models.Config) error {
	var errors []string
	validateGeneralConfig(config, &errors)
	validateAudioConfig(config, &errors)
	validateWebServerConfig(config, &errors)
	validateSecurityConfig(config, &errors)

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation issues: %s", strings.Join(errors, "; "))
	}
	return nil
}
