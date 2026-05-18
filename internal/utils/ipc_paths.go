// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package utils

import (
	"os"
	"path/filepath"

	"github.com/AshBuk/dabri/v2/config"
)

const (
	// DefaultSocketFileName is the default IPC socket filename.
	DefaultSocketFileName = "dabri.sock"
)

// GetDefaultSocketPath returns the default IPC socket path.
func GetDefaultSocketPath() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, DefaultSocketFileName)
	}

	if configDir, err := config.EnsureConfigDir(); err == nil {
		return filepath.Join(configDir, DefaultSocketFileName)
	}

	return filepath.Join(os.TempDir(), DefaultSocketFileName)
}
