// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AshBuk/dabri/v2/config/models"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// Check if a command is in the security whitelist.
// It checks only the base name of the command, ignoring the path, to ensure
// that path-based bypasses are not possible (e.g., /usr/bin/evil is treated as evil)
func IsCommandAllowed(config *models.Config, command string) bool {
	base := filepath.Base(command)
	for _, cmd := range config.Security.AllowedCommands {
		if cmd == base {
			return true
		}
	}
	return false
}

// Filter a list of arguments to remove shell metacharacters
// and other constructs that could be used for command injection attacks
func SanitizeCommandArgs(args []string) []string {
	sanitized := make([]string, 0, len(args))

	for _, arg := range args {
		// Filter out shell metacharacters and directory traversal attempts
		if !strings.ContainsAny(arg, "&|;$<>(){}[]") && !strings.Contains(arg, "..") {
			sanitized = append(sanitized, arg)
		}
	}
	return sanitized
}

// Verify the config file against a stored hash to ensure
// it has not been modified without authorization
func VerifyConfigIntegrity(filename string, config *models.Config, loggers ...logger.Logger) error {
	if !config.Security.CheckIntegrity {
		return nil
	}
	if config.Security.ConfigHash == "" {
		// No hash to compare against, so we can't verify
		return nil
	}

	hash, err := CalculateFileHash(filename, loggers...)
	if err != nil {
		return fmt.Errorf("failed to calculate config file hash: %w", err)
	}
	if hash != config.Security.ConfigHash {
		return fmt.Errorf("config file integrity check failed: hash mismatch")
	}
	return nil
}

// Calculate a new hash for the configuration file and store it
// within the config struct. This is used to "seal" the config after valid changes
func UpdateConfigHash(filename string, config *models.Config, loggers ...logger.Logger) error {
	hash, err := CalculateFileHash(filename, loggers...)
	if err != nil {
		return fmt.Errorf("failed to calculate config file hash: %w", err)
	}
	config.Security.ConfigHash = hash
	return nil
}

// Compute the SHA-256 hash of a file's content
func CalculateFileHash(filename string, loggers ...logger.Logger) (string, error) {
	var logSink logger.Logger = logger.NewDefaultLogger(logger.WarningLevel)
	if len(loggers) > 0 && loggers[0] != nil {
		logSink = loggers[0]
	}
	// Clean the path to prevent null byte and other injection attacks
	safe := filepath.Clean(filename)
	if strings.Contains(safe, "\x00") {
		return "", fmt.Errorf("invalid filename")
	}

	// #nosec G304 -- Path is cleaned and expected to be a controlled local config file.
	f, err := os.Open(safe)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := f.Close(); err != nil {
			// Log the error but don't return it, as the primary operation (hashing) succeeded
			logSink.Warning("Failed to close file %s: %v", filename, err)
		}
	}()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Enforce the maximum configured size for a file.
// This is a security measure to prevent denial-of-service attacks using large files
func EnforceFileSizeLimit(filename string, config *models.Config) error {
	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() > config.Security.MaxTempFileSize {
		return fmt.Errorf("file size exceeds limit: %d bytes (limit: %d bytes)",
			info.Size(), config.Security.MaxTempFileSize)
	}
	return nil
}
