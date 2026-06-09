// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package loaders

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/AshBuk/dabri/v2/config/models"
	"github.com/AshBuk/dabri/v2/config/validators"
	"github.com/AshBuk/dabri/v2/internal/logger"
	yaml "gopkg.in/yaml.v2"
)

// Single source of truth for defaults: parsed by SetDefaultConfig and written
// verbatim (with comments) on first run.
//
//go:embed default_config.yaml
var defaultConfigYAML []byte

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
		// First run: write the default file (best-effort; defaults already in memory).
		if errors.Is(err, fs.ErrNotExist) {
			if werr := writeDefaultConfigFile(clean); werr != nil {
				logSink.Warning("Could not create default config file: %v", werr)
			} else {
				logSink.Info("Created default configuration file: %s", clean)
			}
		} else {
			logSink.Info("Could not read config file: %v, using defaults", err)
		}
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

// Reset the configuration to the embedded default values.
func SetDefaultConfig(config *models.Config) {
	*config = models.Config{}
	if err := yaml.Unmarshal(defaultConfigYAML, config); err != nil {
		// Embedded at build time, so a parse error is a bug. Guarded by TestSetDefaultConfig.
		panic(fmt.Sprintf("invalid embedded default config: %v", err))
	}
}

// Write the embedded default config (comments intact) to path.
func writeDefaultConfigFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, defaultConfigYAML, 0o600)
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
