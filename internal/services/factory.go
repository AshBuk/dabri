// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/AshBuk/dabri/v2/audio/factory"
	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/manager"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/notify"
	"github.com/AshBuk/dabri/v2/internal/platform"
	"github.com/AshBuk/dabri/v2/internal/ui/tray"
	"github.com/AshBuk/dabri/v2/internal/ui/window"
	outputFactory "github.com/AshBuk/dabri/v2/output/factory"
	outputInterfaces "github.com/AshBuk/dabri/v2/output/interfaces"
	"github.com/AshBuk/dabri/v2/output/outputters"
	"github.com/AshBuk/dabri/v2/stt"
	"github.com/AshBuk/dabri/v2/websocket"
)

// NewContainer builds every component, assembles the services and wires the
// tray and window callbacks. Order matters:
//  1. ModelManager → TempFileManager → Recorder → Engine (core pipeline)
//  2. Outputter (falls back to clipboard-only), ydotoold when needed
//  3. Hotkeys, WebSocket, tray, window, notifications (UI/control)
//  4. Services and their cross-references, then UI callbacks
func NewContainer(ctx context.Context, log logger.Logger, cfg *config.Config, cfgFile string, env platform.EnvironmentType) (*ServiceContainer, error) {
	// Components that apply settings live read them through current.
	// The rest keep cfg, which later updates never mutate.
	configSvc := NewConfigService(log, cfg, cfgFile)
	current := configSvc.GetConfig
	modelManager := stt.NewModelManager(current, log)
	// Resolving the model downloads it when missing
	modelPath, err := modelManager.GetModelPath(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve model path: %w", err)
	}
	log.Info("Model path resolved: %s", modelPath)

	cleanupTimeout := time.Duration(cfg.Audio.TempFileCleanupTime) * time.Minute
	if cleanupTimeout <= 0 {
		cleanupTimeout = 30 * time.Minute
	}
	tempFiles := processing.NewTempFileManager(cleanupTimeout, log)
	tempFiles.Start()

	recorder, err := factory.GetRecorder(cfg, log, tempFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audio recorder: %w", err)
	}
	engine, err := stt.NewEngine(current, modelPath, log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize whisper engine: %w", err)
	}

	// Graceful degradation: if typing fails, fall back to clipboard only.
	// The fallback edits cfg in place, which is safe only because nothing
	// reads it concurrently yet.
	// platform.EnvironmentType is aliased across output/hotkeys packages — no conversion needed
	outputter, err := outputFactory.GetOutputterFromConfig(cfg, env)
	if err != nil {
		log.Warning("Failed to initialize text outputter: %v", err)
		if outputter = newFallbackOutputter(cfg, env, log); outputter == nil {
			return nil, fmt.Errorf("failed to initialize any output manager")
		}
	}
	inputDaemon := startInputDaemonIfNeeded(outputter, log)

	hotkeyManager := manager.NewHotkeyManager(hotkeyConfig(cfg), env, log)
	// Always created, started only when enabled in config
	wsServer := websocket.NewWebSocketServer(cfg, log)
	trayManager := tray.CreateTrayManagerWithConfig(cfg, log)
	// Start the tray early so systray is initialized (no-op in mock)
	trayManager.Start()
	trayManager.UpdateSettings(cfg)
	win := newWindow(ctx, cfg, log)
	notifier := notify.NewNotificationManager("Dabri", current)

	audioSvc := NewAudioService(log, current, recorder, engine, modelManager, tempFiles)
	uiSvc := NewUIService(log, trayManager, notifier, win, current)
	ioSvc := NewIOService(log, current, outputter, wsServer)

	configSvc.SetUIService(uiSvc)     // reload notifications
	ioSvc.SetUIService(uiSvc)         // output notifications
	ioSvc.SetConfigService(configSvc) // output settings
	wsServer.SetAudioController(audioSvc)
	audioSvc.SetDependencies(uiSvc, ioSvc)
	audioSvc.SetConfig(configSvc)

	c := &ServiceContainer{
		Logger:          log,
		Audio:           audioSvc,
		UI:              uiSvc,
		IO:              ioSvc,
		Config:          configSvc,
		Hotkeys:         NewHotkeyService(log, hotkeyManager),
		Window:          win,
		TempFileManager: tempFiles,
		InputDaemon:     inputDaemon,
	}
	wireUI(c, trayManager, win)
	return c, nil
}

// newFallbackOutputter creates a clipboard-only outputter
func newFallbackOutputter(cfg *config.Config, env outputFactory.EnvironmentType, log logger.Logger) outputInterfaces.Outputter {
	clipboardTool := ""
	if env == outputFactory.EnvironmentWayland {
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
		log.Info("Falling back to clipboard output using %s", clipboardTool)
		oldMode := cfg.Output.DefaultMode
		cfg.Output.DefaultMode = config.OutputModeClipboard
		cfg.Output.ClipboardTool = clipboardTool

		if out, err := outputters.NewClipboardOutputter(clipboardTool, cfg); err == nil {
			return out
		}
		// Restore original mode if fallback failed
		cfg.Output.DefaultMode = oldMode
	}
	return nil
}

// startInputDaemonIfNeeded launches ydotoold only when ydotool is the resolved
// typing backend inside Flatpak. Returns nil otherwise (portal/clipboard/native),
// and logs guidance when uinput access is missing so typing degrades to clipboard.
func startInputDaemonIfNeeded(out outputInterfaces.Outputter, log logger.Logger) *outputters.YdotoolDaemon {
	if out == nil || !platform.IsFlatpak() {
		return nil
	}
	if _, typeTool := out.GetToolNames(); typeTool != "ydotool" {
		return nil
	}
	if daemon := outputters.StartYdotoolDaemon(); daemon != nil {
		log.Info("Started ydotoold for active-window typing")
		return daemon
	}
	log.Warning("Active-window typing needs uinput access; run 'flatpak override --user --device=all io.github.ashbuk.dabri' (see docs) — falling back to clipboard otherwise")
	return nil
}

// newWindow creates the main window backend (no-op without a GUI backend).
// Actions are wired later by wireUI.
func newWindow(ctx context.Context, cfg *config.Config, log logger.Logger) window.Manager {
	hasTray := platform.HasStatusNotifierWatcher()
	opts := window.Options{
		HasTray:        hasTray,
		StartVisible:   !hasTray || isFirstRun(),
		ActiveModel:    cfg.General.Model,
		ActiveLanguage: cfg.General.Language,
		OutputMode:     cfg.Output.DefaultMode,
		Hotkey:         cfg.Hotkeys.StartRecording,
		HotkeyMode:     cfg.Hotkeys.Mode,
	}
	for _, m := range constants.Models {
		opts.Models = append(opts.Models, window.ModelChoice{ID: m.ID, Name: m.Name, DetectsLanguage: m.DetectsLanguage()})
	}
	for _, l := range constants.WhisperLanguages {
		opts.Languages = append(opts.Languages, window.LangChoice{Code: l.Code, Name: l.Name})
	}
	win := window.New(log, opts)
	if !hasTray {
		// On login, autostart can beat the GNOME AppIndicator extension to the
		// session bus; pick up a tray that registers shortly after startup.
		go func() {
			for range 30 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
				if platform.HasStatusNotifierWatcher() {
					win.EnableTray()
					return
				}
			}
		}()
	}
	return win
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

// wireUI connects tray menu and main window callbacks to the container's
// actions. Callbacks require the fully assembled container.
func wireUI(c *ServiceContainer, tm tray.Manager, win window.Manager) {
	tm.SetCoreActions(c.ToggleRecording, c.UI.ShowConfigFile, c.UI.ShowAboutPage, c.ResetToDefaults)
	tm.SetShowWindowAction(win.Show)
	tm.SetAudioActions(c.setRecordingMethod)
	tm.SetModelAction(c.selectModel)
	tm.SetSettingsActions(c.setLanguage, c.toggleNotifications, c.setOutputMode)
	tm.OutputToolsCallback(c.IO.GetOutputToolNames)
	tm.SetCaptureOnceSupport(c.Hotkeys.SupportsCaptureOnce)
	tm.UpdateSettings(c.Config.GetConfig())
	tm.SetHotkeyRebindAction(c.rebindHotkey)
	tm.SetHotkeyModeAction(c.setHotkeyMode)
	tm.SetExitAction(quit)

	// The no-op window backend ignores these
	win.SetActions(window.Actions{
		OnToggleRecording:  c.ToggleRecording,
		OnSelectModel:      c.selectModel,
		OnSelectLanguage:   c.setLanguage,
		OnSelectOutput:     c.setOutputMode,
		OnSelectHotkeyMode: c.setHotkeyMode,
		OnQuit:             quit,
	})
}

// quit requests a clean shutdown through the app's signal handler.
func quit() { _ = syscall.Kill(os.Getpid(), syscall.SIGTERM) }
