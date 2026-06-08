// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package manager

import (
	"fmt"
	"sync"
	"time"

	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/hotkeys/interfaces"
	"github.com/AshBuk/dabri/v2/hotkeys/providers"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// HotkeyAction represents a hotkey action callback
type HotkeyAction func() error

// Manages keyboard shortcuts, providers, and actions
type HotkeyManager struct {
	config          adapters.HotkeyConfig
	isListening     bool
	recordingToggle func() error
	hotkeyActions   map[string]HotkeyAction // Maps hotkey actions to their callbacks
	hotkeysMutex    sync.Mutex
	environment     interfaces.EnvironmentType
	provider        interfaces.KeyboardEventProvider
	logger          logger.Logger
}

// Create a new instance of the HotkeyManager
func NewHotkeyManager(config adapters.HotkeyConfig, environment interfaces.EnvironmentType, logger logger.Logger) *HotkeyManager {
	manager := &HotkeyManager{
		config:        config,
		isListening:   false,
		environment:   environment,
		hotkeyActions: make(map[string]HotkeyAction),
		logger:        logger,
	}
	// Initialize the appropriate keyboard provider
	manager.provider = selectProviderForEnvironment(manager.config, manager.environment, manager.logger)

	return manager
}

// selectProviderForEnvironment is defined in OS-specific files (e.g., manager_linux.go)

// RegisterToggle registers the callback that starts or stops recording. The
// callback owns the start/stop decision (it consults the recorder's real state),
// so the hotkey keeps no recording state of its own and never desyncs from the
// window or tray.
func (h *HotkeyManager) RegisterToggle(toggle func() error) {
	h.recordingToggle = toggle
}

// Register a custom hotkey action
func (h *HotkeyManager) RegisterHotkeyAction(hotkey string, action HotkeyAction) {
	h.hotkeysMutex.Lock()
	defer h.hotkeysMutex.Unlock()
	h.hotkeyActions[hotkey] = action
}

// Unregister a custom hotkey action
func (h *HotkeyManager) UnregisterHotkeyAction(hotkey string) {
	h.hotkeysMutex.Lock()
	defer h.hotkeysMutex.Unlock()
	delete(h.hotkeyActions, hotkey)
}

// Return all registered hotkeys
func (h *HotkeyManager) GetRegisteredHotkeys() []string {
	h.hotkeysMutex.Lock()
	defer h.hotkeysMutex.Unlock()

	var hotkeys []string
	// Add the primary recording hotkeys
	hotkeys = append(hotkeys, h.config.GetStartRecordingHotkey())
	// Add any custom hotkeys
	for hotkey := range h.hotkeyActions {
		hotkeys = append(hotkeys, hotkey)
	}

	return hotkeys
}

// Start listening for hotkeys
func (h *HotkeyManager) Start() error {
	if h.isListening {
		return fmt.Errorf("hotkey manager is already running")
	}
	if h.provider == nil {
		return fmt.Errorf("no keyboard provider available - hotkeys will not work")
	}
	h.isListening = true

	h.logger.Info("Starting hotkey manager...")
	h.logger.Info("- Start/Stop recording: %s", h.config.GetStartRecordingHotkey())
	// Register all hotkeys on the selected provider
	if err := h.registerAllHotkeysOn(h.provider); err != nil {
		return err
	}
	// Start the provider and handle potential fallbacks
	err := h.provider.Start()
	if err != nil {
		h.isListening = false
		return startFallbackAfterRegistration(h, err)
	}
	return nil
}

// Stop the hotkey listener
func (h *HotkeyManager) Stop() {
	if h.isListening {
		h.provider.Stop()
		h.isListening = false
	}
}

// Simulate a hotkey press for testing purposes
func (h *HotkeyManager) SimulateHotkeyPress(hotkeyName string) error {
	h.hotkeysMutex.Lock()
	defer h.hotkeysMutex.Unlock()

	switch hotkeyName {
	case "toggle_recording":
		if h.recordingToggle != nil {
			return h.recordingToggle()
		}
	default:
		return fmt.Errorf("unknown hotkey: %s", hotkeyName)
	}
	return nil
}

// Reload the configuration by stopping, updating the provider, and restarting
func (h *HotkeyManager) ReloadConfig(newConfig adapters.HotkeyConfig) error {
	if h.isListening && h.provider != nil {
		h.provider.Stop()
		h.isListening = false
	}
	h.config = newConfig
	h.provider = selectProviderForEnvironment(h.config, h.environment, h.logger)
	if h.provider == nil {
		return fmt.Errorf("no keyboard provider available - hotkeys will not work")
	}
	// Re-register all hotkeys on the new provider
	if err := h.registerAllHotkeysOn(h.provider); err != nil {
		return err
	}
	if err := h.provider.Start(); err != nil {
		return startFallbackAfterRegistration(h, err)
	}
	h.isListening = true
	return nil
}

// Attempt to capture a single hotkey combination
// Temporarily stops provider to release devices, then captures on fresh instance
func (h *HotkeyManager) CaptureOnce(timeout time.Duration) (string, error) {
	if h.provider == nil {
		return "", fmt.Errorf("no keyboard provider available")
	}
	// Provider supports capture: stop to release devices, capture on fresh instance, restart
	if h.provider.SupportsCaptureOnce() {
		h.provider.Stop()

		// Skip IsSupported() check - if evdev provider was running, evdev is available
		// CaptureOnce will return error if no devices found
		captureProvider := providers.NewEvdevKeyboardProvider(h.logger)
		result, err := captureProvider.CaptureOnce(timeout)
		if reloadErr := h.ReloadConfig(h.config); reloadErr != nil {
			h.logger.Error("Failed to restart hotkeys after capture: %v", reloadErr)
		}
		return result, err
	}
	// Provider doesn't support capture: use evdev fallback (D-Bus case)
	// Here we need IsSupported() since evdev wasn't the active provider
	fallback := providers.NewEvdevKeyboardProvider(h.logger)
	if fallback != nil && fallback.IsSupported() {
		return fallback.CaptureOnce(timeout)
	}
	return "", fmt.Errorf("capture not supported")
}

// Check if the active provider supports the capture-once functionality
func (h *HotkeyManager) SupportsCaptureOnce() bool {
	if h.provider == nil {
		return false
	}
	return h.provider.SupportsCaptureOnce()
}
