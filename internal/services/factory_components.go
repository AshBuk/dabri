// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/AshBuk/dabri/v2/audio/factory"
	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/hotkeys/manager"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/notify"
	"github.com/AshBuk/dabri/v2/internal/platform"
	"github.com/AshBuk/dabri/v2/internal/ui/tray"
	"github.com/AshBuk/dabri/v2/internal/ui/window"
	outputFactory "github.com/AshBuk/dabri/v2/output/factory"
	outputInterfaces "github.com/AshBuk/dabri/v2/output/interfaces"
	"github.com/AshBuk/dabri/v2/output/outputters"
	"github.com/AshBuk/dabri/v2/websocket"
	"github.com/AshBuk/dabri/v2/whisper"
)

// FactoryComponents is responsible for creating low-level components
// Stage 1 of Multi-Stage Factory Pattern (see factory.go)
type FactoryComponents struct {
	config ServiceFactoryConfig // Factory configuration (logger, config, environment)
}

// NewFactoryComponents creates a new component factory
func NewFactoryComponents(config ServiceFactoryConfig) *FactoryComponents {
	return &FactoryComponents{config: config}
}

// InitializeComponents creates all low-level components in dependency order
// Dependency-aware initialization:
//  1. ModelManager → TempFileManager → Recorder → WhisperEngine (core pipeline)
//  2. OutputManager (with Graceful Degradation fallback)
//  3. HotkeyManager, WebSocketServer, TrayManager, NotifyManager (UI/control)
func (cf *FactoryComponents) InitializeComponents() (*Components, error) {
	components := &Components{}
	// Initialize model manager
	components.ModelManager = whisper.NewModelManager(cf.config.Config)
	if err := components.ModelManager.Initialize(cf.config.Ctx); err != nil {
		cf.config.Logger.Warning("Failed to initialize model manager: %v", err)
	}
	// Ensure model is available
	if err := cf.ensureModelAvailable(components.ModelManager); err != nil {
		return nil, fmt.Errorf("failed to ensure model availability: %w", err)
	}
	// Get model file path
	modelFilePath, err := components.ModelManager.GetModelPath(cf.config.Ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve model path: %w", err)
	}
	cf.config.Logger.Info("Model path resolved: %s", modelFilePath)
	// Initialize temp file manager
	cleanupTimeout := time.Duration(cf.config.Config.Audio.TempFileCleanupTime) * time.Minute
	if cleanupTimeout <= 0 {
		cleanupTimeout = 30 * time.Minute
	}
	components.TempFileManager = processing.NewTempFileManager(cleanupTimeout, cf.config.Logger)
	components.TempFileManager.Start()
	// Initialize audio recorder
	components.Recorder, err = factory.GetRecorder(cf.config.Config, cf.config.Logger, components.TempFileManager)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audio recorder: %w", err)
	}
	// Initialize whisper engine
	components.WhisperEngine, err = whisper.NewWhisperEngine(cf.config.Config, modelFilePath, cf.config.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize whisper engine: %w", err)
	}
	// Initialize output manager with graceful degradation
	// If typing fails fallback to clipboard only
	// platform.EnvironmentType is aliased across output/hotkeys packages — no conversion needed
	outputEnv := cf.config.Environment
	components.OutputManager, err = outputFactory.GetOutputterFromConfig(cf.config.Config, outputEnv)
	if err != nil {
		cf.config.Logger.Warning("Failed to initialize text outputter: %v", err)
		if fallbackOut := cf.createFallbackOutputManager(outputEnv); fallbackOut != nil {
			components.OutputManager = fallbackOut
		} else {
			return nil, fmt.Errorf("failed to initialize any output manager")
		}
	}
	// Start ydotoold when ydotool is the chosen typing backend (Flatpak/wlroots)
	components.InputDaemon = cf.startInputDaemonIfNeeded(components.OutputManager)
	// Initialize hotkey manager
	components.HotkeyManager = cf.createHotkeyManager()
	// Initialize WebSocket server (always initialized but may not be started).
	// AudioController is wired in Stage 2 by FactoryAssembler once AudioService exists.
	components.WebSocketServer = cf.createWebSocketServer()
	// Initialize tray manager
	components.TrayManager = cf.createTrayManager()
	// Start tray manager (no-op in mock). Ensures systray is initialized early.
	if components.TrayManager != nil {
		components.TrayManager.Start()
		components.TrayManager.UpdateSettings(cf.config.Config)
	}
	// Initialize main window controller (no-op without a GUI backend).
	components.WindowManager = cf.createWindowManager()
	// Initialize notification manager
	components.NotifyManager = notify.NewNotificationManager("Dabri", cf.config.Config)

	return components, nil
}

// ensureModelAvailable ensures the whisper model is available
func (cf *FactoryComponents) ensureModelAvailable(modelManager whisper.ModelManager) error {
	// Try to get the model path, which will download if needed
	_, err := modelManager.GetModelPath(cf.config.Ctx)
	if err != nil {
		cf.config.Logger.Info("Model not found locally, checking download...")
		return fmt.Errorf("failed to ensure model available: %w", err)
	}
	return nil
}

// createFallbackOutputManager creates fallback clipboard-only output manager
func (cf *FactoryComponents) createFallbackOutputManager(outputEnv outputFactory.EnvironmentType) outputInterfaces.Outputter {
	clipboardTool := ""
	if outputEnv == outputFactory.EnvironmentWayland {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			clipboardTool = "wl-copy"
		}
	}
	if clipboardTool == "" {
		if _, err := exec.LookPath("xsel"); err == nil {
			clipboardTool = "xsel"
		}
	}
	if clipboardTool != "" {
		cf.config.Logger.Info("Falling back to clipboard output using %s", clipboardTool)
		oldMode := cf.config.Config.Output.DefaultMode
		cf.config.Config.Output.DefaultMode = config.OutputModeClipboard
		cf.config.Config.Output.ClipboardTool = clipboardTool

		if out, err := outputters.NewClipboardOutputter(clipboardTool, cf.config.Config); err == nil {
			return out
		}
		// Restore original mode if fallback failed
		cf.config.Config.Output.DefaultMode = oldMode
	}
	return nil
}

// startInputDaemonIfNeeded launches ydotoold only when ydotool is the resolved
// typing backend inside Flatpak. Returns nil otherwise (portal/clipboard/native),
// and logs guidance when uinput access is missing so typing degrades to clipboard.
func (cf *FactoryComponents) startInputDaemonIfNeeded(out outputInterfaces.Outputter) *outputters.YdotoolDaemon {
	if out == nil || !platform.IsFlatpak() {
		return nil
	}
	if _, typeTool := out.GetToolNames(); typeTool != "ydotool" {
		return nil
	}
	if daemon := outputters.StartYdotoolDaemon(); daemon != nil {
		cf.config.Logger.Info("Started ydotoold for active-window typing")
		return daemon
	}
	cf.config.Logger.Warning("Active-window typing needs uinput access; run 'flatpak override --user --device=all io.github.ashbuk.dabri' (see docs) — falling back to clipboard otherwise")
	return nil
}

// createHotkeyManager creates and configures hotkey manager
func (cf *FactoryComponents) createHotkeyManager() *manager.HotkeyManager {
	configAdapter := adapters.NewConfigAdapter(cf.config.Config.Hotkeys.StartRecording, cf.config.Config.Hotkeys.Provider).
		WithAdditionalHotkeys(
			cf.config.Config.Hotkeys.ShowConfig,
			cf.config.Config.Hotkeys.ResetToDefaults,
		)
	return manager.NewHotkeyManager(configAdapter, cf.config.Environment, cf.config.Logger)
}

// createWebSocketServer creates WebSocket server
func (cf *FactoryComponents) createWebSocketServer() *websocket.WebSocketServer {
	return websocket.NewWebSocketServer(cf.config.Config, cf.config.Logger)
}

// createTrayManager creates system tray manager.
// Callbacks are wired later in Stage 3 (FactoryWirer).
func (cf *FactoryComponents) createTrayManager() tray.Manager {
	return tray.CreateTrayManagerWithConfig(cf.config.Config, cf.config.Logger)
}

// createWindowManager creates the main window backend (no-op without a GUI
// backend). Actions are wired later in Stage 3 (FactoryWirer).
func (cf *FactoryComponents) createWindowManager() window.Manager {
	cfg := cf.config.Config
	hasTray := platform.HasStatusNotifierWatcher()
	opts := window.Options{
		HasTray:      hasTray,
		StartVisible: !hasTray || isFirstRun(),
		ActiveModel:  cfg.General.WhisperModel,
		OutputMode:   cfg.Output.DefaultMode,
		Hotkey:       cfg.Hotkeys.StartRecording,
	}
	for _, m := range constants.WhisperModels {
		opts.Models = append(opts.Models, window.ModelChoice{ID: m.ID, Name: m.Name})
	}
	return window.New(cf.config.Logger, opts)
}

// isFirstRun reports whether the config file does not yet exist, used to show
// the window once on a fresh install even when a tray is available.
func isFirstRun() bool {
	path, err := config.ConfigFilePath()
	if err != nil {
		return false
	}
	_, statErr := os.Stat(path)
	return os.IsNotExist(statErr)
}
