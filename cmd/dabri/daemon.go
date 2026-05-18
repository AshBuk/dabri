// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/app"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/utils"
)

// addDaemonFlags registers daemon-specific flags on the root command.
// These flags are only relevant when running as daemon (no subcommand).
func addDaemonFlags(cmd *cobra.Command) {
	defaultConfigPath, err := config.ConfigFilePath()
	if err != nil {
		defaultConfigPath = "config.yaml" // fallback to current directory
	}
	cmd.Flags().String("config", defaultConfigPath, "Path to configuration file")
	cmd.Flags().Bool("debug", false, "Enable debug mode")
}

// Daemon orchestrator - initializes and runs the main application
// Execution flow:
//  1. Read flags (--config, --debug) from cobra.Command
//  2. Bootstrap logger (Info/Debug level)
//  3. Single-instance lock (prevent multiple daemon processes)
//  4. App lifecycle: NewApp() → Initialize() → RunAndWait()
func runDaemonCobra(cmd *cobra.Command) error {
	configFile, _ := cmd.Flags().GetString("config")
	debug, _ := cmd.Flags().GetBool("debug")

	// Bootstrap
	logLevel := logger.InfoLevel
	if debug {
		logLevel = logger.DebugLevel
	}
	appLogger := logger.NewDefaultLogger(logLevel)

	// Log AppImage environment if detected
	if appDir := os.Getenv("APPDIR"); appDir != "" {
		appLogger.Info("Running inside AppImage, base path: %s", appDir)
	}

	// Single-instance protection
	lockFile := utils.NewLockFile(utils.GetDefaultLockPath())
	if isRunning, pid, err := lockFile.CheckExistingInstance(); err != nil {
		appLogger.Warning("Failed to check existing instance: %v", err)
	} else if isRunning {
		fmt.Fprintf(os.Stderr, "Another instance of dabri is already running (PID: %d)\n", pid)
		fmt.Fprintf(os.Stderr, "If you're sure no other instance is running, remove the lock file: %s\n", lockFile.GetLockFilePath())
		return fmt.Errorf("daemon already running")
	}
	if err := lockFile.TryLock(); err != nil {
		appLogger.Error("Failed to acquire application lock: %v", err)
		return err
	}

	// Resource cleanup:
	// defer guarantees unlock even on panic or early return
	defer func() {
		if err := lockFile.Unlock(); err != nil {
			appLogger.Warning("Failed to release lock: %v", err)
		}
	}()

	// App orchestration: delegate to App module
	// → see internal/app/app.go
	application := app.NewApp(appLogger)
	if err := application.Initialize(configFile, debug); err != nil {
		appLogger.Error("Failed to initialize application: %v", err)
		return err
	}
	if err := application.RunAndWait(); err != nil {
		appLogger.Error("Application error: %v", err)
		return err
	}
	return nil
}
