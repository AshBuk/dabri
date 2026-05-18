// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/platform"
	outputFactory "github.com/AshBuk/dabri/v2/output/factory"
	outputInterfaces "github.com/AshBuk/dabri/v2/output/interfaces"
	"github.com/AshBuk/dabri/v2/websocket"
)

// Handles text output routing and transcription synchronization
type IOService struct {
	logger          logger.Logger
	config          *config.Config
	outputManager   outputInterfaces.Outputter
	webSocketServer *websocket.WebSocketServer
	// Dependencies
	ui  UIServiceInterface
	cfg ConfigServiceInterface

	// Transcription synchronization
	mu                      sync.Mutex
	transcriptionInProgress bool
	transcriptionResultChan chan string
}

// Create a new service instance
func NewIOService(
	logger logger.Logger,
	config *config.Config,
	outputManager outputInterfaces.Outputter,
	webSocketServer *websocket.WebSocketServer,
) *IOService {
	return &IOService{
		logger:          logger,
		config:          config,
		outputManager:   outputManager,
		webSocketServer: webSocketServer,
	}
}

// Prevent clipboard race conditions during async transcription
func (ios *IOService) BeginTranscription() {
	ios.mu.Lock()
	defer ios.mu.Unlock()
	ios.transcriptionInProgress = true
	// Reset result channel for a new transcription session
	ios.transcriptionResultChan = make(chan string, 1)
	// Clear clipboard to prevent race condition with old content
	// Only when clipboard mode is active to avoid unnecessary side-effects
	if ios.outputManager != nil && ios.config.Output.DefaultMode == "clipboard" {
		if err := ios.outputManager.CopyToClipboard(""); err != nil {
			ios.logger.Debug("Failed to clear clipboard: %v", err)
		}
	}
}

// Release clipboard protection and notify waiting readers
func (ios *IOService) CompleteTranscription(result string) {
	ios.mu.Lock()
	inProgress := ios.transcriptionInProgress
	ch := ios.transcriptionResultChan
	ios.transcriptionInProgress = false
	ios.mu.Unlock()

	if inProgress && ch != nil {
		select {
		case ch <- result:
		default:
		}
	}
}

// Block until transcription completes or timeout for synchronized reads
func (ios *IOService) WaitForTranscription(timeout time.Duration) (string, error) {
	ios.mu.Lock()
	inProgress := ios.transcriptionInProgress
	ch := ios.transcriptionResultChan
	ios.mu.Unlock()
	if !inProgress || ch == nil {
		return "", nil
	}
	select {
	case res := <-ch:
		return res, nil
	case <-time.After(timeout):
		// Auto-cleanup on timeout
		ios.mu.Lock()
		ios.transcriptionInProgress = false
		ios.mu.Unlock()
		return "", fmt.Errorf("timeout waiting for transcription result")
	}
}

// Route text to configured output method with fallback strategy
func (ios *IOService) OutputText(text string) error {
	if ios.outputManager == nil {
		return fmt.Errorf("output manager not available")
	}
	ios.logger.Info("Outputting text: %s", text)
	switch ios.config.Output.DefaultMode {
	case "clipboard":
		if err := ios.outputManager.CopyToClipboard(text); err != nil {
			return ios.fallbackToTyping(text, err)
		}
		ios.logger.Debug("Successfully copied text to clipboard")
	case "active_window":
		if err := ios.outputManager.TypeToActiveWindow(text); err != nil {
			return ios.fallbackToClipboard(text, err)
		}
		ios.logger.Debug("Successfully typed text to active window")
	default:
		ios.logger.Warning("Unknown output mode '%s', using typing with clipboard fallback", ios.config.Output.DefaultMode)
		if err := ios.outputManager.TypeToActiveWindow(text); err != nil {
			return ios.fallbackToClipboard(text, err)
		}
	}
	return nil
}

// Force clipboard output bypassing default routing
func (ios *IOService) OutputToClipboard(text string) error {
	if ios.outputManager == nil {
		return fmt.Errorf("output manager not available")
	}
	if err := ios.outputManager.CopyToClipboard(text); err != nil {
		ios.logger.Error("Failed to copy to clipboard: %v", err)
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}
	return nil
}

// Force typing output bypassing default routing
func (ios *IOService) OutputByTyping(text string) error {
	if ios.outputManager == nil {
		return fmt.Errorf("output manager not available")
	}
	if err := ios.outputManager.TypeToActiveWindow(text); err != nil {
		ios.logger.Error("Failed to type to active window: %v", err)
		return fmt.Errorf("failed to type to active window: %w", err)
	}
	return nil
}

// Switch output mode and persist setting with UI refresh
func (ios *IOService) SetOutputMethod(method string) error {
	ios.logger.Info("Setting output method to: %s", method)
	// Validate method
	if method != "clipboard" && method != "active_window" {
		return fmt.Errorf("invalid output method: %s (must be 'clipboard' or 'active_window')", method)
	}
	// Persist via ConfigService if available
	if ios.cfg != nil {
		if err := ios.cfg.UpdateOutputMode(method); err != nil {
			return err
		}
	} else {
		ios.config.Output.DefaultMode = method
	}
	// Recreate output manager immediately to reflect new mode
	env := ios.detectOutputEnvironment()
	out, err := outputFactory.GetOutputterFromConfig(ios.config, env)
	if err != nil {
		return fmt.Errorf("failed to reinitialize output manager: %w", err)
	}
	ios.outputManager = out
	// Notify UI to refresh settings display
	if ios.ui != nil {
		ios.ui.UpdateSettings(ios.config)
	}
	ios.logger.Info("Output method set to: %s", method)
	return nil
}

// Enable external client communication channel
func (ios *IOService) StartWebSocketServer() error {
	if ios.webSocketServer == nil {
		return fmt.Errorf("WebSocket server not configured")
	}
	ios.logger.Info("Starting WebSocket server...")
	// Start the WebSocket server
	return ios.webSocketServer.Start()
}

// Gracefully close external client connections
func (ios *IOService) StopWebSocketServer() error {
	if ios.webSocketServer == nil {
		return nil
	}
	ios.logger.Info("Stopping WebSocket server...")
	// Stop the WebSocket server
	ios.webSocketServer.Stop()
	return nil
}

// Report active clipboard/typing tool implementations for debugging
func (ios *IOService) GetOutputToolNames() (clipboardTool, typeTool string) {
	if ios.outputManager == nil {
		return "unknown", "unknown"
	}
	return ios.outputManager.GetToolNames()
}

// Ensure all connections are closed before termination
func (ios *IOService) Shutdown() error {
	var lastErr error
	// Stop WebSocket server
	if err := ios.StopWebSocketServer(); err != nil {
		ios.logger.Error("Error stopping WebSocket server: %v", err)
		lastErr = err
	}
	ios.logger.Info("IOService shutdown complete")
	return lastErr
}

// detectOutputEnvironment returns the current platform display server type.
// platform.EnvironmentType is aliased across output/hotkeys packages — no conversion needed.
func (ios *IOService) detectOutputEnvironment() outputFactory.EnvironmentType {
	return platform.DetectEnvironment()
}

// Wire UI service for settings refresh notifications
func (ios *IOService) SetUIService(ui UIServiceInterface) { ios.ui = ui }

// Wire config service for persistent setting changes
func (ios *IOService) SetConfigService(cfg ConfigServiceInterface) { ios.cfg = cfg }

// fallbackToTyping switches to typing mode and executes the output
func (ios *IOService) fallbackToTyping(text string, originalErr error) error {
	ios.logger.Warning("Clipboard method failed: %v. Switching to active_window mode.", originalErr)
	if err := ios.switchOutputMode(config.OutputModeActiveWindow, "clipboard failed"); err != nil {
		return fmt.Errorf("failed to switch output mode: %w", err)
	}
	if err := ios.outputManager.TypeToActiveWindow(text); err != nil {
		return fmt.Errorf("both clipboard and typing failed - clipboard: %w, typing: %v", originalErr, err)
	}
	return nil
}

// fallbackToClipboard switches to clipboard mode and executes the output
func (ios *IOService) fallbackToClipboard(text string, originalErr error) error {
	ios.logger.Warning("Active window method failed: %v. Switching to clipboard mode.", originalErr)
	if err := ios.switchOutputMode(config.OutputModeClipboard, "typing failed"); err != nil {
		return fmt.Errorf("failed to switch output mode: %w", err)
	}
	if err := ios.outputManager.CopyToClipboard(text); err != nil {
		return fmt.Errorf("both typing and clipboard failed - typing: %w, clipboard: %v", originalErr, err)
	}
	ios.logger.Debug("Successfully copied text to clipboard (after switch)")
	return nil
}

// switchOutputMode switches the output mode and persists it in the config
func (ios *IOService) switchOutputMode(mode string, reason string) error {
	ios.logger.Warning("Switching output mode to '%s' due to '%s'", mode, reason)
	// Update and persist config if service is available
	if ios.cfg != nil {
		if err := ios.cfg.UpdateOutputMode(mode); err != nil {
			ios.logger.Error("Failed to persist output mode: %v", err)
			return fmt.Errorf("failed to persist output mode: %w", err)
		}
	} else {
		old := ios.config.Output.DefaultMode
		ios.config.Output.DefaultMode = mode
		ios.logger.Warning("ConfigService not available; switched output mode in memory from %s to %s", old, mode)
	}
	// Recreate output manager for the new mode based on current environment
	env := ios.detectOutputEnvironment()
	out, err := outputFactory.GetOutputterFromConfig(ios.config, env)
	if err != nil {
		ios.logger.Error("Failed to reinitialize output manager after mode switch: %v", err)
		return fmt.Errorf("failed to reinitialize output manager after mode switch: %w", err)
	}
	ios.outputManager = out
	// Notify user
	if ios.ui != nil {
		msg := fmt.Sprintf("Switched to %s due to %s", mode, reason)
		ios.ui.ShowNotification("Output Fallback", msg)
		ios.ui.UpdateSettings(ios.config)
	}
	ios.logger.Info("Output mode switched to '%s' and persisted", mode)
	return nil
}
