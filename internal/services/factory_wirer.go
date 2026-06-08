// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/ui/window"
)

// FactoryWirer wires tray menu callbacks to service business logic
// Stage 3 of Multi-Stage Factory (see factory.go)
// Callbacks require fully assembled container with all services ready
//
// Callback wiring (UI → Services):
//
//	Core Actions      → AudioService, UIService, ConfigService (toggle, config, reset)
//	Audio Actions     → ConfigService, AudioService (recorder selection)
//	Settings Actions  → ConfigService, UIService, IOService (language, notifications, output)
//	Dynamic Callbacks → IOService, HotkeyService (output tools, capture support)
//	Hotkey Rebinding  → All Services (UI, Config, Hotkeys, Audio)
//	Exit Action       → System Signal (SIGTERM)
type FactoryWirer struct {
	logger logger.Logger
}

func NewFactoryWirer(logger logger.Logger) *FactoryWirer {
	return &FactoryWirer{logger: logger}
}

// Wire connects tray menu callbacks to services via closure pattern
// Wiring process:
//  1. Core actions     (toggle recording, show config/about, reset defaults)
//  2. Audio actions    (recorder selection)
//  3. Settings actions (language, notifications, output mode)
//  4. Dynamic queries  (output tools, capture once support)
//  5. UI sync          (update tray menu with current settings)
//  6. Hotkey rebinding (rebind hotkeys dynamically)
//  7. Exit handler     (clean shutdown via SIGTERM)
//
// Each callback is a closure capturing ServiceContainer for service access
func (cw *FactoryWirer) Wire(container *ServiceContainer, components *Components) {
	if components == nil || components.TrayManager == nil {
		return
	}
	// Step 1: Core actions (toggle, show config, reset defaults)
	components.TrayManager.SetCoreActions(
		cw.makeToggleCallback(container),
		cw.makeShowConfigCallback(container),
		cw.makeShowAboutCallback(container),
		cw.makeResetToDefaultsCallback(container),
	)
	// Open the main window from the tray
	components.TrayManager.SetShowWindowAction(func() {
		if container.Window != nil {
			container.Window.Show()
		}
	})
	// Step 2: Audio actions (recorder selection)
	components.TrayManager.SetAudioActions(cw.makeRecorderSelectionCallback(container))
	// Step 2b: Model selection
	components.TrayManager.SetModelAction(cw.makeModelSelectionCallback(container))
	// Step 3: Settings actions (language, notifications, output mode)
	components.TrayManager.SetSettingsActions(
		cw.makeLanguageCallback(container),
		cw.makeToggleNotificationsCallback(container),
		cw.makeOutputModeCallback(container),
	)
	// Step 4: Dynamic queries (output tools, capture once support)
	components.TrayManager.OutputToolsCallback(cw.makeGetOutputToolsCallback(container))
	components.TrayManager.SetCaptureOnceSupport(cw.makeCaptureOnceSupportCallback(container))
	// Step 5: UI sync (update tray menu with current settings)
	cw.updateTraySettings(container, components.TrayManager)
	// Step 6: Hotkey rebinding (rebind hotkeys dynamically)
	components.TrayManager.SetHotkeyRebindAction(cw.makeHotkeyRebindCallback(container))
	// Step 7: Exit handler (clean shutdown via SIGTERM)
	components.TrayManager.SetExitAction(func() {
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	})

	// Step 8: Main window actions (no-op backend ignores these)
	if components.WindowManager != nil {
		components.WindowManager.SetActions(window.Actions{
			OnToggleRecording: cw.makeToggleCallback(container),
			OnSelectModel:     cw.makeModelSelectionCallback(container),
			OnSelectLanguage:  cw.makeLanguageCallback(container),
			OnSelectOutput:    cw.makeOutputModeCallback(container),
			OnQuit: func() {
				_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
			},
		})
	}
}

// Callback Makers
func (cw *FactoryWirer) makeToggleCallback(container *ServiceContainer) func() error {
	return func() error { // closure captures container
		if container == nil || container.Audio == nil { // Null Object - defensive check
			return fmt.Errorf("audio service not available")
		}
		if container.Audio.IsRecording() {
			return container.Audio.HandleStopRecording()
		}
		return container.Audio.HandleStartRecording()
	}
}
func (cw *FactoryWirer) makeShowConfigCallback(container *ServiceContainer) func() error {
	return func() error {
		if container == nil || container.UI == nil {
			return fmt.Errorf("UI service not available")
		}
		return container.UI.ShowConfigFile()
	}
}
func (cw *FactoryWirer) makeShowAboutCallback(container *ServiceContainer) func() error {
	return func() error {
		if container == nil || container.UI == nil {
			return fmt.Errorf("UI service not available")
		}
		return container.UI.ShowAboutPage()
	}
}
func (cw *FactoryWirer) makeResetToDefaultsCallback(container *ServiceContainer) func() error {
	return func() error {
		if container == nil || container.Config == nil {
			return fmt.Errorf("config service not available")
		}
		if err := container.Config.ResetToDefaults(); err != nil {
			return err
		}
		return cw.reloadHotkeysFromConfig(container)
	}
}
func (cw *FactoryWirer) makeRecorderSelectionCallback(container *ServiceContainer) func(string) error {
	return func(method string) error {
		if container == nil || container.Config == nil || container.Audio == nil {
			return fmt.Errorf("services not available")
		}
		if err := container.Config.UpdateRecordingMethod(method); err != nil {
			return err
		}
		container.Audio.ClearSession()
		cw.updateUISettings(container)
		return nil
	}
}
func (cw *FactoryWirer) makeModelSelectionCallback(container *ServiceContainer) func(context.Context, string) error {
	return func(ctx context.Context, modelID string) error {
		if container == nil || container.Audio == nil || container.Config == nil {
			return fmt.Errorf("services not available")
		}
		// Remember previous model so we can rollback on persist failure
		previousModel := ""
		if cfg := container.Config.GetConfig(); cfg != nil {
			previousModel = cfg.General.WhisperModel
		}
		if err := container.Audio.SwitchModel(ctx, modelID); err != nil {
			// Don't show error notification on user-initiated cancellation
			if ctx.Err() != nil {
				return err
			}
			if container.UI != nil {
				container.UI.ShowNotification("Model Switch Failed", err.Error())
			}
			return err
		}
		if err := container.Config.UpdateWhisperModel(modelID); err != nil {
			// Rollback engine to previous model so runtime state matches config
			if previousModel != "" {
				if rbErr := container.Audio.SwitchModel(ctx, previousModel); rbErr != nil {
					cw.logger.Error("Failed to rollback model after config save error: %v", rbErr)
				}
			}
			return err
		}
		cw.updateUISettings(container)
		if container.UI != nil {
			container.UI.ShowNotification("Model Switch", fmt.Sprintf("Switched to %s", modelID))
		}
		return nil
	}
}
func (cw *FactoryWirer) makeLanguageCallback(container *ServiceContainer) func(string) error {
	return func(language string) error {
		if container == nil || container.Config == nil {
			return fmt.Errorf("config service not available")
		}
		return container.Config.UpdateLanguage(language)
	}
}
func (cw *FactoryWirer) makeToggleNotificationsCallback(container *ServiceContainer) func() error {
	return func() error {
		if container == nil || container.Config == nil {
			return fmt.Errorf("config service not available")
		}
		if err := container.Config.ToggleWorkflowNotifications(); err != nil {
			return err
		}
		if container.UI != nil {
			enabled := cw.getNotificationStatus(container)
			container.UI.ShowNotification("Workflow Notifications", "Now "+enabled)
		}
		return nil
	}
}
func (cw *FactoryWirer) makeOutputModeCallback(container *ServiceContainer) func(string) error {
	return func(mode string) error {
		if container == nil || container.Config == nil || container.IO == nil {
			return fmt.Errorf("services not available")
		}
		if err := container.Config.UpdateOutputMode(mode); err != nil {
			return err
		}
		if err := container.IO.SetOutputMethod(mode); err != nil {
			return err
		}
		cw.updateUISettings(container)
		return nil
	}
}
func (cw *FactoryWirer) makeGetOutputToolsCallback(container *ServiceContainer) func() (string, string) {
	return func() (clipboardTool, typeTool string) {
		if container == nil || container.IO == nil {
			return "unknown", "unknown"
		}
		return container.IO.GetOutputToolNames()
	}
}
func (cw *FactoryWirer) makeCaptureOnceSupportCallback(container *ServiceContainer) func() bool {
	return func() bool {
		if container == nil || container.Hotkeys == nil {
			return false
		}
		return container.Hotkeys.SupportsCaptureOnce()
	}
}
func (cw *FactoryWirer) makeHotkeyRebindCallback(container *ServiceContainer) func(string) error {
	return func(action string) error {
		if container == nil || container.UI == nil || container.Config == nil || container.Hotkeys == nil {
			return fmt.Errorf("services not available")
		}

		container.UI.ShowNotification("Rebind Hotkey", "Press new hotkey… (Esc to cancel)")
		combo, err := container.Hotkeys.CaptureOnce(3000)
		if err != nil || strings.TrimSpace(combo) == "" {
			container.UI.ShowNotification("Rebind Hotkey", "Cancelled or timeout")
			return nil
		}
		if err := container.Config.UpdateHotkey(action, combo); err != nil {
			return err
		}
		if err := cw.reloadHotkeysFromConfig(container); err != nil {
			return err
		}
		container.UI.ShowNotification("Hotkey Updated", fmt.Sprintf("%s -> %s", action, combo))
		return nil
	}
}

// Helper methods - shared logic for callbacks
//   - reloadHotkeysFromConfig: reload hotkeys from config (used by reset + rebind)
//   - makeConfigProvider: adapt Config → HotkeyConfig
//   - updateUISettings: sync UI with config changes
//   - updateTraySettings: sync tray menu with config changes
//   - getNotificationStatus: query notification state from config
func (cw *FactoryWirer) reloadHotkeysFromConfig(container *ServiceContainer) error {
	if container.Hotkeys == nil {
		return nil
	}

	cfgProvider := cw.makeConfigProvider(container)
	return container.Hotkeys.ReloadFromConfig(
		cw.makeToggleCallback(container),
		cfgProvider,
	)
}
func (cw *FactoryWirer) makeConfigProvider(container *ServiceContainer) func() adapters.HotkeyConfig {
	return func() adapters.HotkeyConfig { // Adapter Pattern - converts Config → HotkeyConfig
		if container.Config == nil {
			return adapters.NewConfigAdapter("", "auto") // Graceful Degradation - return default
		}
		cfg := container.Config.GetConfig()
		if cfg == nil {
			return adapters.NewConfigAdapter("", "auto")
		}
		return adapters.NewConfigAdapter(cfg.Hotkeys.StartRecording, cfg.Hotkeys.Provider).
			WithAdditionalHotkeys(cfg.Hotkeys.ShowConfig, cfg.Hotkeys.ResetToDefaults)
	}
}
func (cw *FactoryWirer) updateUISettings(container *ServiceContainer) {
	if container.UI == nil || container.Config == nil {
		return
	}
	if cfg := container.Config.GetConfig(); cfg != nil {
		container.UI.UpdateSettings(cfg)
	}
}
func (cw *FactoryWirer) updateTraySettings(container *ServiceContainer, trayManager interface{ UpdateSettings(*config.Config) }) {
	if container == nil || container.Config == nil {
		return
	}
	if cfg := container.Config.GetConfig(); cfg != nil {
		trayManager.UpdateSettings(cfg)
	}
}
func (cw *FactoryWirer) getNotificationStatus(container *ServiceContainer) string {
	if container.Config == nil {
		return "disabled"
	}
	cfg := container.Config.GetConfig()
	if cfg != nil && cfg.Notifications.EnableWorkflowNotifications {
		return "enabled"
	}
	return "disabled"
}
