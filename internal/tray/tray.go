//go:build systray

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package tray

import (
	"context"
	"sync"

	"fyne.io/systray"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// TrayManager manages the system tray icon and menu
type TrayManager struct {
	isRecording       bool
	iconMicOff        []byte
	iconMicOn         []byte
	onExit            func()
	onToggle          func() error
	onShowConfig      func() error
	onShowAbout       func() error
	onResetToDefaults func() error
	config            *config.Config
	logger            logger.Logger

	// Menu items
	toggleItem       *systray.MenuItem
	settingsItem     *systray.MenuItem
	showConfigItem   *systray.MenuItem
	aboutItem        *systray.MenuItem
	reloadConfigItem *systray.MenuItem
	exitItem         *systray.MenuItem

	// Settings submenus
	hotkeysMenu       *systray.MenuItem
	audioRecorderMenu *systray.MenuItem
	languageMenu      *systray.MenuItem
	modelMenu         *systray.MenuItem
	outputMenu        *systray.MenuItem

	// Dynamic settings items
	hotkeyItems   map[string]*systray.MenuItem
	audioItems    map[string]*systray.MenuItem
	languageItems map[string]*systray.MenuItem
	modelItems    map[string]*systray.MenuItem
	outputItems   map[string]*systray.MenuItem

	// Audio action callbacks
	onSelectRecorder func(method string) error
	onSelectModel    func(ctx context.Context, modelID string) error

	// Model download cancellation (protected by modelMu)
	modelDownloadCancel context.CancelFunc
	downloadingModelID  string
	modelMu             sync.Mutex
	// Settings callbacks
	onSelectLang           func(language string) error
	onToggleWorkflowNotify func() error
	onGetOutputTools       func() (clipboardTool, typeTool string)
	onSelectOutputMode     func(mode string) error
	onRebindHotkey         func(action string) error

	// Capability callbacks
	getCaptureOnceSupport func() bool

	// Cancellation context for background menu handlers
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewTrayManager creates a new tray manager instance.
// Callbacks are wired later via setter methods.
func NewTrayManager(iconMicOff, iconMicOn []byte, logger logger.Logger) *TrayManager {
	return &TrayManager{
		iconMicOff:    iconMicOff,
		iconMicOn:     iconMicOn,
		hotkeyItems:   make(map[string]*systray.MenuItem),
		audioItems:    make(map[string]*systray.MenuItem),
		languageItems: make(map[string]*systray.MenuItem),
		modelItems:    make(map[string]*systray.MenuItem),
		outputItems:   make(map[string]*systray.MenuItem),
		logger:        logger,
	}
}

// SetCoreActions allows wiring core menu callbacks after construction
func (tm *TrayManager) SetCoreActions(onToggle func() error, onShowConfig func() error, onShowAbout func() error, onResetToDefaults func() error) {
	tm.onToggle = onToggle
	tm.onShowConfig = onShowConfig
	tm.onShowAbout = onShowAbout
	tm.onResetToDefaults = onResetToDefaults
}

// Start initializes and starts the system tray icon and menu
func (tm *TrayManager) Start() {
	// Initialize context for background handlers
	if tm.cancel != nil {
		tm.cancel()
	}
	tm.ctx, tm.cancel = context.WithCancel(context.Background())
	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		systray.Run(tm.onReady, func() {
			if tm.onExit != nil {
				tm.onExit()
			}
		})
	}()
}

// onReady sets up the system tray when it's ready
func (tm *TrayManager) onReady() {
	systray.SetIcon(tm.iconMicOff)
	systray.SetTitle("Dabri")
	// Create main menu items
	tm.toggleItem = systray.AddMenuItem("Start Recording", "Start/Stop recording")
	// Workflow notifications toggle (radio indicator is set by updateWorkflowNotificationUI)
	tm.audioItems["workflow_notifications"] = systray.AddMenuItem(
		"○ Workflow Notifications",
		"Toggle workflow notifications (recording, transcription)",
	)

	systray.AddSeparator()
	// Settings submenu
	tm.settingsItem = systray.AddMenuItem("Settings", "Application settings")
	tm.createSettingsSubmenus()

	systray.AddSeparator()
	// Config actions
	tm.showConfigItem = systray.AddMenuItem("Show Config File", "Open configuration file")
	tm.reloadConfigItem = systray.AddMenuItem("Reset to Defaults", "Reset all settings to default values")
	tm.aboutItem = systray.AddMenuItem("About", "About Dabri")

	systray.AddSeparator()
	tm.exitItem = systray.AddMenuItem("Quit", "Quit Dabri")
	// Handle menu item clicks
	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		tm.handleMenuClicks()
	}()

	// Apply the current recording state once menu items are ready
	// This avoids missing early state updates before systray initialization
	tm.SetRecordingState(tm.isRecording)
}

// createSettingsSubmenus is implemented in settings_menu.go

// UpdateSettings updates the settings display with new configuration
func (tm *TrayManager) UpdateSettings(config *config.Config) {
	// Store and delegate all UI updates to helpers in settings_menu.go
	tm.config = config
	// Ensure hotkeys UI reflects latest config
	tm.updateHotkeysMenuUI()
	// The helpers gracefully no-op if items are not yet created
	tm.updateRecorderRadioUI(config.Audio.RecordingMethod)
	tm.updateLanguageRadioUI(config.General.Language)
	tm.updateModelRadioUI(config.General.WhisperModel)
	tm.updateWorkflowNotificationUI(config.Notifications.EnableWorkflowNotifications)
	tm.updateOutputUI()
}

// handleMenuClicks handles all menu item clicks
func (tm *TrayManager) handleMenuClicks() {
	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-tm.toggleItem.ClickedCh:
			tm.logger.Info("Toggle recording clicked")
			if tm.onToggle != nil {
				if err := tm.onToggle(); err != nil {
					tm.logger.Error("Error toggling recording: %v", err)
				}
			}
		case <-tm.showConfigItem.ClickedCh:
			tm.logger.Info("Show config clicked")
			if tm.onShowConfig != nil {
				if err := tm.onShowConfig(); err != nil {
					tm.logger.Error("Error showing config: %v", err)
				}
			}
		case <-tm.aboutItem.ClickedCh:
			tm.logger.Info("About clicked")
			if tm.onShowAbout != nil {
				if err := tm.onShowAbout(); err != nil {
					tm.logger.Error("Error showing about: %v", err)
				}
			}
		case <-tm.reloadConfigItem.ClickedCh:
			tm.logger.Info("Reset to defaults clicked")
			if tm.onResetToDefaults != nil {
				if err := tm.onResetToDefaults(); err != nil {
					tm.logger.Error("Error resetting to defaults: %v", err)
				}
			}
		case <-tm.exitItem.ClickedCh:
			tm.logger.Info("Exit clicked")
			if tm.cancel != nil {
				tm.cancel()
			}
			// Quit systray first to ensure UI responds
			systray.Quit()
			// Then trigger application shutdown
			if tm.onExit != nil {
				tm.onExit() // Call directly, not in goroutine to ensure cleanup
			}
			return
		}
	}
}

// SetRecordingState updates the tray icon and menu to reflect recording state
func (tm *TrayManager) SetRecordingState(isRecording bool) {
	tm.isRecording = isRecording
	// Guard against early calls before onReady creates menu items
	if tm.toggleItem == nil {
		return
	}

	if isRecording {
		systray.SetIcon(tm.iconMicOn)
		tm.toggleItem.SetTitle("Stop Recording")
	} else {
		systray.SetIcon(tm.iconMicOff)
		tm.toggleItem.SetTitle("Start Recording")
	}
}

// Stop stops the tray manager
func (tm *TrayManager) Stop() {
	if tm.cancel != nil {
		tm.cancel()
	}
	systray.Quit()
	tm.wg.Wait()
}

// SetAudioActions sets callbacks for audio-related actions (recorder selection)
func (tm *TrayManager) SetAudioActions(onSelectRecorder func(method string) error) {
	tm.onSelectRecorder = onSelectRecorder
}

// SetModelAction sets callback for whisper model selection
func (tm *TrayManager) SetModelAction(onSelectModel func(ctx context.Context, modelID string) error) {
	tm.onSelectModel = onSelectModel
}

// SetExitAction allows overriding the exit callback (useful once services are wired)
func (tm *TrayManager) SetExitAction(onExit func()) {
	tm.onExit = onExit
}

// SetSettingsActions sets callbacks for general settings
func (tm *TrayManager) SetSettingsActions(
	onSelectLanguage func(language string) error,
	onToggleWorkflowNotifications func() error,
	onSelectOutputMode func(mode string) error,
) {
	tm.onSelectLang = onSelectLanguage
	tm.onToggleWorkflowNotify = onToggleWorkflowNotifications
	tm.onSelectOutputMode = onSelectOutputMode
}

// SetHotkeyRebindAction sets callback for hotkey rebind action
func (tm *TrayManager) SetHotkeyRebindAction(onRebind func(action string) error) {
	tm.onRebindHotkey = onRebind
}

// OutputToolsCallback sets the callback for getting actual output tool names
func (tm *TrayManager) OutputToolsCallback(callback func() (clipboardTool, typeTool string)) {
	tm.onGetOutputTools = callback
}

// SetCaptureOnceSupport sets a callback indicating whether capture once is supported
func (tm *TrayManager) SetCaptureOnceSupport(callback func() bool) {
	tm.getCaptureOnceSupport = callback
}
