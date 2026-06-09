//go:build systray

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package tray

import (
	"context"

	"fyne.io/systray"
	"github.com/AshBuk/dabri/v2/internal/constants"
)

// Create settings submenus
func (tm *TrayManager) createSettingsSubmenus() {
	tm.hotkeysMenu = tm.settingsItem.AddSubMenuItem("Hotkeys", "Hotkey settings")
	tm.audioRecorderMenu = tm.settingsItem.AddSubMenuItem("Audio Recorder", "Select audio recorder")
	tm.languageMenu = tm.settingsItem.AddSubMenuItem("Set Language", "Select recognition language")
	tm.modelMenu = tm.settingsItem.AddSubMenuItem("Whisper Model", "Select whisper model")

	tm.outputMenu = tm.settingsItem.AddSubMenuItem("Output", "Output settings")

	// Populate with initial values if config is available
	if tm.config != nil {
		tm.populateSettingsMenus()
	}
}

// populateSettingsMenus populates the settings submenus with current config values
func (tm *TrayManager) populateSettingsMenus() {
	if tm.config == nil {
		return
	}

	tm.updateHotkeysMenuUI()
	tm.setupAudioRecorderMenu()
	tm.setupLanguageMenu()
	tm.setupModelMenu()
	tm.setupOutputMenu()
	tm.setupWorkflowNotifications()
}

// setupAudioRecorderMenu creates and configures the audio recorder submenu
func (tm *TrayManager) setupAudioRecorderMenu() {
	// Create menu items
	tm.audioItems["recorder_arecord"] = tm.audioRecorderMenu.AddSubMenuItem(
		"○ arecord (ALSA)",
		"Use arecord (ALSA)",
	)
	tm.audioItems["recorder_ffmpeg"] = tm.audioRecorderMenu.AddSubMenuItem(
		"○ ffmpeg (PulseAudio)",
		"Use ffmpeg (PulseAudio)",
	)

	// Set up click handlers.
	// Use closure to read tm.onSelectRecorder at invocation time (not capture time),
	// because callbacks are wired after tray setup via setter methods.
	recorderCb := func(method string) error {
		if tm.onSelectRecorder != nil {
			return tm.onSelectRecorder(method)
		}
		return nil
	}
	tm.handleRadioItemClick(
		tm.audioItems["recorder_arecord"],
		"arecord",
		"Audio recorder switched to %s (UI)",
		tm.updateRecorderRadioUI,
		recorderCb,
	)
	tm.handleRadioItemClick(
		tm.audioItems["recorder_ffmpeg"],
		"ffmpeg",
		"Audio recorder switched to %s (UI)",
		tm.updateRecorderRadioUI,
		recorderCb,
	)

	// Update initial UI state
	tm.updateRecorderRadioUI(tm.config.Audio.RecordingMethod)
}

// setupLanguageMenu creates and configures the language selection submenu.
// Uses flat alphabetical list for GNOME compatibility
// (GNOME AppIndicator has 3-level menu depth limit).
func (tm *TrayManager) setupLanguageMenu() {
	// Create current language display at the top
	currentLang := constants.LanguageByCode(tm.config.General.Language)
	currentName := tm.config.General.Language
	if currentLang != nil {
		currentName = currentLang.Name + " (" + currentLang.Code + ")"
	}
	tm.languageItems["current"] = tm.languageMenu.AddSubMenuItem(
		"● "+currentName,
		"Current language",
	)
	tm.languageItems["current"].Disable()

	// Use closure to read tm.onSelectLang at invocation time (not capture time)
	langCb := func(code string) error {
		if tm.onSelectLang != nil {
			return tm.onSelectLang(code)
		}
		return nil
	}

	// Flat list of all languages (GNOME-compatible, 3 levels: Settings > Set Language > Language)
	for _, lang := range constants.WhisperLanguages {
		indicator := "○ "
		if tm.config.General.Language == lang.Code {
			indicator = "● "
		}
		itm := tm.languageMenu.AddSubMenuItem(indicator+lang.Name+" ("+lang.Code+")", lang.Code)
		tm.languageItems["lang_"+lang.Code] = itm

		langCode := lang.Code
		tm.handleRadioItemClick(
			itm,
			langCode,
			"Language switched to %s (UI)",
			tm.updateLanguageRadioUI,
			langCb,
		)
	}
}

// setupModelMenu creates and configures the whisper model selection submenu.
// Unlike other radio menus, model items use custom click handling that supports
// cancel-on-reclick: clicking a downloading model cancels the download (✖ indicator).
func (tm *TrayManager) setupModelMenu() {
	currentModel := constants.ModelByID(tm.config.General.WhisperModel)
	currentName := tm.config.General.WhisperModel
	if currentModel != nil {
		currentName = currentModel.Name
	}
	tm.modelItems["current"] = tm.modelMenu.AddSubMenuItem(
		"● "+currentName,
		"Current model",
	)
	tm.modelItems["current"].Disable()

	for _, m := range constants.WhisperModels {
		indicator := "○ "
		if tm.config.General.WhisperModel == m.ID {
			indicator = "● "
		}
		itm := tm.modelMenu.AddSubMenuItem(indicator+m.Name, m.ID)
		tm.modelItems["model_"+m.ID] = itm

		modelID := m.ID
		tm.handleMenuItemClick(itm, func() {
			tm.handleModelClick(modelID)
		})
	}
}

// handleModelClick handles model item click with download cancellation support.
// Re-clicking a model that is currently downloading cancels the download.
func (tm *TrayManager) handleModelClick(modelID string) {
	tm.modelMu.Lock()

	// Re-click on the same downloading model → cancel
	if tm.downloadingModelID == modelID && tm.modelDownloadCancel != nil {
		cancel := tm.modelDownloadCancel
		tm.modelDownloadCancel = nil
		tm.downloadingModelID = ""
		tm.modelMu.Unlock()
		cancel()
		tm.logger.Info("Model download cancelled by user: %s", modelID)
		if tm.config != nil {
			tm.updateModelRadioUI(tm.config.General.WhisperModel)
		}
		return
	}

	// Cancel any in-progress download for a different model
	prevModelID := tm.downloadingModelID
	if tm.modelDownloadCancel != nil {
		tm.modelDownloadCancel()
	}

	ctx, cancel := context.WithCancel(tm.ctx)
	tm.modelDownloadCancel = cancel
	tm.downloadingModelID = modelID
	tm.modelMu.Unlock()

	// Restore UI for the previously downloading model
	if prevModelID != "" && prevModelID != modelID && tm.config != nil {
		tm.updateModelRadioUI(tm.config.General.WhisperModel)
	}

	tm.setModelDownloadingUI(modelID)

	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		var err error
		if tm.onSelectModel != nil {
			err = tm.onSelectModel(ctx, modelID)
		}

		// Cancelled — caller (re-click or auto-cancel) already restored UI
		if ctx.Err() != nil {
			return
		}

		tm.modelMu.Lock()
		if tm.downloadingModelID == modelID {
			tm.modelDownloadCancel = nil
			tm.downloadingModelID = ""
		}
		tm.modelMu.Unlock()

		if err != nil {
			tm.logger.Error("Model switch failed: %v", err)
			if tm.config != nil {
				tm.updateModelRadioUI(tm.config.General.WhisperModel)
			}
			return
		}
		tm.updateModelRadioUI(modelID)
	}()
}

// setModelDownloadingUI shows download-in-progress state with ✖ cancel indicator
func (tm *TrayManager) setModelDownloadingUI(modelID string) {
	name := modelID
	if model := constants.ModelByID(modelID); model != nil {
		name = model.Name
	}
	if itm := tm.modelItems["model_"+modelID]; itm != nil {
		itm.SetTitle("✖ " + name + " (downloading…)")
	}
	if currentDisplay := tm.modelItems["current"]; currentDisplay != nil {
		currentDisplay.SetTitle("⏳ Switching to " + name + "…")
	}
}

// updateModelRadioUI updates selection marks for model menu
func (tm *TrayManager) updateModelRadioUI(modelID string) {
	for _, m := range constants.WhisperModels {
		if itm := tm.modelItems["model_"+m.ID]; itm != nil {
			if m.ID == modelID {
				itm.SetTitle("● " + m.Name)
			} else {
				itm.SetTitle("○ " + m.Name)
			}
		}
	}
	if currentDisplay := tm.modelItems["current"]; currentDisplay != nil {
		model := constants.ModelByID(modelID)
		name := modelID
		if model != nil {
			name = model.Name
		}
		currentDisplay.SetTitle("● " + name)
	}
}

// setupOutputMenu creates and configures the output mode submenu
func (tm *TrayManager) setupOutputMenu() {
	modeDefs := []struct{ key, title string }{
		{"clipboard", "Clipboard"}, {"active_window", "Active Window"},
	}

	// Create output mode items
	for _, m := range modeDefs {
		indicator := "○ "
		if tm.config.Output.DefaultMode == m.key {
			indicator = "● "
		}
		itm := tm.outputMenu.AddSubMenuItem(indicator+m.title, "")
		tm.outputItems["mode_"+m.key] = itm
	}

	// Create display items (gray text)
	tm.outputItems["mode"] = tm.outputMenu.AddSubMenuItem(
		"Mode: "+humanizeOutputMode(tm.config.Output.DefaultMode),
		"Current output mode",
	)
	tm.outputItems["mode"].Disable()

	tm.outputItems["clipboard_tool"] = tm.outputMenu.AddSubMenuItem(
		"Clipboard Tool: "+tm.config.Output.ClipboardTool,
		"Current clipboard tool",
	)
	tm.outputItems["clipboard_tool"].Disable()

	tm.outputItems["type_tool"] = tm.outputMenu.AddSubMenuItem(
		"Type Tool: "+tm.config.Output.TypeTool,
		"Current typing tool",
	)
	tm.outputItems["type_tool"].Disable()

	// Set up click handlers.
	// Use closure to read tm.onSelectOutputMode at invocation time (not capture time)
	outputCb := func(mode string) error {
		if tm.onSelectOutputMode != nil {
			return tm.onSelectOutputMode(mode)
		}
		return nil
	}
	for _, k := range []string{"clipboard", "active_window"} {
		if itm := tm.outputItems["mode_"+k]; itm != nil {
			key := k
			tm.handleRadioItemClick(
				itm,
				key,
				"Output mode switched to %s (UI)",
				tm.updateOutputModeRadioUI,
				outputCb,
			)
		}
	}
}

// setupWorkflowNotifications creates and configures the workflow notifications toggle
func (tm *TrayManager) setupWorkflowNotifications() {
	if tm.audioItems["workflow_notifications"] == nil {
		return
	}

	tm.handleMenuItemClick(tm.audioItems["workflow_notifications"], func() {
		tm.logger.Info("Workflow notifications toggle clicked")
		if tm.onToggleWorkflowNotify != nil {
			if err := tm.onToggleWorkflowNotify(); err != nil {
				tm.logger.Error("Error toggling workflow notifications: %v", err)
			}
			// Reflect new state in UI using updated config
			if tm.config != nil {
				tm.updateWorkflowNotificationUI(tm.config.Notifications.EnableWorkflowNotifications)
			}
		}
	})

	// Update initial UI state
	tm.updateWorkflowNotificationUI(tm.config.Notifications.EnableWorkflowNotifications)
}

// hotkeyMenuConfig describes configuration for a single hotkey menu item
type hotkeyMenuConfig struct {
	actionName     string // Action identifier for rebind callback (e.g., "start_recording")
	rebindKey      string // Key in hotkeyItems map for rebind button (e.g., "rebind_start_stop")
	displayKey     string // Key in hotkeyItems map for display item (e.g., "display_start_stop")
	rebindTitle    string // Title for rebind button (e.g., "Rebind Start/Stop…")
	rebindTooltip  string // Tooltip for rebind button
	displayPrefix  string // Prefix for display item title (e.g., "Start/Stop Recording: ")
	displayTooltip string // Tooltip for display item
	currentValue   string // Current hotkey value from config
}

// createHotkeyMenuItem creates or updates a hotkey menu item with rebind capability.
// This helper reduces code duplication for hotkey menu item creation.
func (tm *TrayManager) createHotkeyMenuItem(cfg hotkeyMenuConfig, supportsCaptureOnce bool) {
	// Create rebind button if supported and not exists
	if tm.hotkeyItems[cfg.rebindKey] == nil {
		if supportsCaptureOnce {
			rebindBtn := tm.hotkeysMenu.AddSubMenuItem(cfg.rebindTitle, cfg.rebindTooltip)
			tm.hotkeyItems[cfg.rebindKey] = rebindBtn

			// Use shared click helper to reduce duplication
			tm.handleMenuItemClick(rebindBtn, func() {
				if tm.onRebindHotkey != nil {
					if err := tm.onRebindHotkey(cfg.actionName); err != nil {
						tm.logger.Error("Error rebinding %s: %v", cfg.actionName, err)
					}
				}
			})
		}
	}

	// Create or update display item
	displayTitle := cfg.displayPrefix + cfg.currentValue

	if tm.hotkeyItems[cfg.displayKey] == nil {
		displayItem := tm.hotkeysMenu.AddSubMenuItem(displayTitle, cfg.displayTooltip)
		displayItem.Disable()
		tm.hotkeyItems[cfg.displayKey] = displayItem
	} else {
		tm.hotkeyItems[cfg.displayKey].SetTitle(displayTitle)
	}
}

// updateHotkeysMenuUI ensures hotkeys items exist and reflect current config
func (tm *TrayManager) updateHotkeysMenuUI() {
	if tm.hotkeysMenu == nil || tm.config == nil {
		return
	}

	// Determine capture once support (default true if callback unset)
	supportsCaptureOnce := true
	if tm.getCaptureOnceSupport != nil {
		supportsCaptureOnce = tm.getCaptureOnceSupport()
	}

	// Define configurations for all hotkey menu items
	hotkeyConfigs := []hotkeyMenuConfig{
		{
			actionName:     "start_recording",
			rebindKey:      "rebind_start_stop",
			displayKey:     "display_start_stop",
			rebindTitle:    "Rebind Start/Stop…",
			rebindTooltip:  "Change start/stop hotkey",
			displayPrefix:  "Start/Stop Recording: ",
			displayTooltip: "Current start/stop recording hotkey",
			currentValue:   tm.config.Hotkeys.StartRecording,
		},
		{
			actionName:     "show_config",
			rebindKey:      "rebind_show_config",
			displayKey:     "display_show_config",
			rebindTitle:    "Rebind Show Config…",
			rebindTooltip:  "Change show config hotkey",
			displayPrefix:  "Show Config: ",
			displayTooltip: "Current show config hotkey",
			currentValue:   tm.config.Hotkeys.ShowConfig,
		},
		{
			actionName:     "reset_to_defaults",
			rebindKey:      "rebind_reset_defaults",
			displayKey:     "display_reset_defaults",
			rebindTitle:    "Rebind Reset to Defaults…",
			rebindTooltip:  "Change reset defaults hotkey",
			displayPrefix:  "Reset to Defaults: ",
			displayTooltip: "Current reset to defaults hotkey",
			currentValue:   tm.config.Hotkeys.ResetToDefaults,
		},
	}

	// Create/update all hotkey menu items
	for _, cfg := range hotkeyConfigs {
		tm.createHotkeyMenuItem(cfg, supportsCaptureOnce)
	}

	// Without capture-once (portal path) bindings are owned by the compositor and
	// can only be changed in the system settings, so note that instead of rebind.
	if !supportsCaptureOnce && tm.hotkeyItems["rebind_note"] == nil {
		note := tm.hotkeysMenu.AddSubMenuItem(
			"Set shortcuts in system settings",
			"On Wayland, global shortcuts are managed by the desktop's Keyboard Shortcuts",
		)
		note.Disable()
		tm.hotkeyItems["rebind_note"] = note
	}
}

// updateRecorderRadioUI updates titles to emulate radio selection
func (tm *TrayManager) updateRecorderRadioUI(method string) {
	arecordItem := tm.audioItems["recorder_arecord"]
	ffmpegItem := tm.audioItems["recorder_ffmpeg"]
	if arecordItem == nil || ffmpegItem == nil {
		return
	}

	switch method {
	case "arecord":
		arecordItem.SetTitle("● arecord (ALSA)")
		ffmpegItem.SetTitle("○ ffmpeg (PulseAudio)")
	case "ffmpeg":
		ffmpegItem.SetTitle("● ffmpeg (PulseAudio)")
		arecordItem.SetTitle("○ arecord (ALSA)")
	default:
		arecordItem.SetTitle("○ arecord (ALSA)")
		ffmpegItem.SetTitle("○ ffmpeg (PulseAudio)")
	}
}

// updateLanguageRadioUI updates selection marks for language menu
func (tm *TrayManager) updateLanguageRadioUI(lang string) {
	// Update all language items
	for _, l := range constants.WhisperLanguages {
		if itm := tm.languageItems["lang_"+l.Code]; itm != nil {
			if l.Code == lang {
				itm.SetTitle("● " + l.Name + " (" + l.Code + ")")
			} else {
				itm.SetTitle("○ " + l.Name + " (" + l.Code + ")")
			}
		}
	}

	// Update the current language display at the top
	if currentDisplay := tm.languageItems["current"]; currentDisplay != nil {
		currentLang := constants.LanguageByCode(lang)
		currentName := lang
		if currentLang != nil {
			currentName = currentLang.Name + " (" + currentLang.Code + ")"
		}
		currentDisplay.SetTitle("● " + currentName)
	}
}

// updateWorkflowNotificationUI updates the workflow notifications toggle UI
func (tm *TrayManager) updateWorkflowNotificationUI(enabled bool) {
	item := tm.audioItems["workflow_notifications"]
	if item == nil {
		return
	}

	if enabled {
		item.SetTitle("● Workflow Notifications")
	} else {
		item.SetTitle("○ Workflow Notifications")
	}
}

// updateOutputModeRadioUI updates selection marks for output mode menu
func (tm *TrayManager) updateOutputModeRadioUI(mode string) {
	modeDefs := map[string]string{
		"clipboard": "Clipboard", "active_window": "Active Window",
	}

	for key, title := range modeDefs {
		if itm := tm.outputItems["mode_"+key]; itm != nil {
			if key == mode {
				itm.SetTitle("● " + title)
			} else {
				itm.SetTitle("○ " + title)
			}
		}
	}

	// Update the gray text display
	if modeDisplay := tm.outputItems["mode"]; modeDisplay != nil {
		modeDisplay.SetTitle("Mode: " + humanizeOutputMode(mode))
	}
}

// updateOutputUI updates the output settings display
func (tm *TrayManager) updateOutputUI() {
	if tm.config == nil {
		return
	}

	// Update mode radio buttons and display
	tm.updateOutputModeRadioUI(tm.config.Output.DefaultMode)

	// Get actual tool names if callback is available
	clipboardTool := tm.config.Output.ClipboardTool
	typeTool := tm.config.Output.TypeTool
	if tm.onGetOutputTools != nil {
		clipboardTool, typeTool = tm.onGetOutputTools()
	}

	// Update clipboard tool display
	if clipboardItem := tm.outputItems["clipboard_tool"]; clipboardItem != nil {
		clipboardItem.SetTitle("Clipboard Tool: " + clipboardTool)
	}

	// Update type tool display
	if typeItem := tm.outputItems["type_tool"]; typeItem != nil {
		typeItem.SetTitle("Type Tool: " + typeTool)
	}
}

// Helper functions for menu handling

// handleMenuItemClick creates a goroutine that handles menu item clicks
// with context cancellation support. This helper reduces boilerplate for menu handlers.
func (tm *TrayManager) handleMenuItemClick(item *systray.MenuItem, handler func()) {
	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		ch := item.ClickedCh
		for {
			select {
			case <-tm.ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				handler()
			}
		}
	}()
}

// handleRadioItemClick handles radio button menu items with automatic logging and UI update.
// This is a specialized helper for settings that behave like radio groups.
func (tm *TrayManager) handleRadioItemClick(
	item *systray.MenuItem,
	value string,
	logTemplate string,
	updateUI func(string),
	callback func(string) error,
) {
	tm.handleMenuItemClick(item, func() {
		tm.logger.Info(logTemplate, value)
		if callback != nil {
			if err := callback(value); err != nil {
				tm.logger.Error("Error: %v", err)
				return
			}
		}
		// Update UI only after successful callback
		updateUI(value)
	})
}

// humanizeOutputMode converts internal mode keys to human-readable titles
func humanizeOutputMode(mode string) string {
	switch mode {
	case "clipboard":
		return "Clipboard"
	case "active_window":
		return "Active Window"
	default:
		return mode
	}
}
