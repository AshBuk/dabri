//go:build !systray

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package tray

import (
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// CreateDefaultTrayManager creates the default tray manager
// based on available dependencies.
func CreateDefaultTrayManager(logger logger.Logger) Manager {
	// Use the mock implementation as fallback when systray is not available
	return CreateMockTrayManager(logger)
}

// CreateTrayManagerWithConfig creates tray manager with initial configuration.
func CreateTrayManagerWithConfig(config *config.Config, logger logger.Logger) Manager {
	trayManager := CreateDefaultTrayManager(logger)
	trayManager.UpdateSettings(config)
	return trayManager
}
