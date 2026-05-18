// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package manager

import (
	"fmt"
	"strings"

	"github.com/AshBuk/dabri/v2/hotkeys/interfaces"
	"github.com/AshBuk/dabri/v2/hotkeys/providers"
)

// Register all configured hotkeys on the given provider
func (h *HotkeyManager) registerAllHotkeysOn(provider interfaces.KeyboardEventProvider) error {
	// Register the primary start/stop recording hotkey
	if err := provider.RegisterHotkey(h.config.GetStartRecordingHotkey(), func() error {
		h.hotkeysMutex.Lock()
		defer h.hotkeysMutex.Unlock()

		if !h.isRecording && h.recordingStarted != nil {
			h.logger.Info("Start recording hotkey detected")
			if err := h.recordingStarted(); err != nil {
				h.logger.Error("Error starting recording: %v", err)
				return err
			}
			h.isRecording = true
		} else if h.isRecording && h.recordingStopped != nil {
			h.logger.Info("Stop recording hotkey detected")
			if err := h.recordingStopped(); err != nil {
				h.logger.Error("Error stopping recording: %v", err)
				return err
			}
			h.isRecording = false
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to register start/stop recording hotkey: %w", err)
	}
	// Register any additional custom hotkeys
	h.hotkeysMutex.Lock()
	defer h.hotkeysMutex.Unlock()
	for actionName, action := range h.hotkeyActions {
		hk := h.config.GetActionHotkey(actionName)
		if strings.TrimSpace(hk) == "" {
			// Skip actions that do not have a configured hotkey
			continue
		}

		act := action
		if err := provider.RegisterHotkey(hk, func() error {
			h.logger.Info("Custom hotkey detected: %s (%s)", actionName, hk)
			if err := act(); err != nil {
				h.logger.Error("Error executing hotkey action for %s: %v", actionName, err)
				return err
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to register hotkey %s for action %s: %w", hk, actionName, err)
		}
	}

	return nil
}

// Attempt to switch to a fallback provider if the primary one fails
func startFallbackAfterRegistration(h *HotkeyManager, startErr error) error {
	h.logger.Error("Primary keyboard provider failed to start: %v", startErr)
	// Only fall back to evdev if current provider doesn't support capture (i.e., D-Bus)
	if !h.provider.SupportsCaptureOnce() {
		fallback := providers.NewEvdevKeyboardProvider(h.logger)
		if fallback != nil && fallback.IsSupported() {
			h.logger.Info("Falling back to evdev keyboard provider")
			h.provider = fallback
			if err := h.registerAllHotkeysOn(h.provider); err != nil {
				return fmt.Errorf("failed to register hotkeys on fallback provider: %w", err)
			}
			if err := h.provider.Start(); err != nil {
				return fmt.Errorf("failed to start fallback keyboard provider: %w", err)
			}
			h.logger.Info("Fallback keyboard provider started successfully")
			return nil
		}
	}
	return fmt.Errorf("failed to start keyboard provider: %w", startErr)
}
