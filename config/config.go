// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

// Package config provides configuration management functionality with support for multiple
// configuration formats, validation, and security features.
//
// Subpackages:
//   - models:     Defines the core configuration data structures.
//   - loaders:    Handles loading and saving configuration from/to different formats (e.g., YAML).
//   - validators: Implements validation logic to ensure configuration integrity.
//   - security:   Provides security-related utilities like integrity checks and command validation.
package config

import (
	"os"
	"path/filepath"

	"github.com/AshBuk/dabri/v2/config/loaders"
	"github.com/AshBuk/dabri/v2/config/models"
	"github.com/AshBuk/dabri/v2/config/security"
	"github.com/AshBuk/dabri/v2/config/validators"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// Config is a type alias for the main configuration structure defined in the models package.
// This provides a convenient way to reference the configuration type without importing the models package directly.
type Config = models.Config

const (
	// Output mode constants, aliased from the models package for convenience.
	OutputModeClipboard    = models.OutputModeClipboard
	OutputModeActiveWindow = models.OutputModeActiveWindow

	// AppName is the application identifier used in paths.
	AppName = "dabri"
	// ConfigFileName is the default configuration file name.
	ConfigFileName = "config.yaml"
)

// ConfigDir returns the XDG config directory for the application.
// Returns $XDG_CONFIG_HOME/dabri or ~/.config/dabri
func ConfigDir() (string, error) {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, AppName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", AppName), nil
}

// ConfigFilePath returns the default configuration file path.
// Returns $XDG_CONFIG_HOME/dabri/config.yaml or ~/.config/dabri/config.yaml
func ConfigFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Load configuration from the specified file using the configured loader.
func LoadConfig(filename string, loggers ...logger.Logger) (*Config, error) {
	return loaders.LoadConfig(filename, loggers...)
}

// Write the configuration to the specified file.
func SaveConfig(filename string, config *Config) error {
	return loaders.SaveConfig(filename, config)
}

// Apply the default values to a configuration object.
func SetDefaultConfig(config *Config) {
	loaders.SetDefaultConfig(config)
}

// Check the configuration for correctness and apply corrections if necessary.
func ValidateConfig(config *Config) error {
	return validators.ValidateConfig(config)
}

// Check if a command is permitted by the security policy.
func IsCommandAllowed(config *Config, command string) bool {
	return security.IsCommandAllowed(config, command)
}

// Remove potentially unsafe arguments from a command.
func SanitizeCommandArgs(args []string) []string {
	return security.SanitizeCommandArgs(args)
}

// Verify if the configuration file has been tampered with.
func VerifyConfigIntegrity(filename string, config *Config, loggers ...logger.Logger) error {
	return security.VerifyConfigIntegrity(filename, config, loggers...)
}

// Calculate and update the integrity hash for the configuration file.
func UpdateConfigHash(filename string, config *Config, loggers ...logger.Logger) error {
	return security.UpdateConfigHash(filename, config, loggers...)
}

// Compute the SHA-256 hash of a file.
func CalculateFileHash(filename string, loggers ...logger.Logger) (string, error) {
	return security.CalculateFileHash(filename, loggers...)
}

// Enforce that a file does not exceed the configured size limit.
func EnforceFileSizeLimit(filename string, config *Config) error {
	return security.EnforceFileSizeLimit(filename, config)
}
