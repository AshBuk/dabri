// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/AshBuk/dabri/v2/internal/ipc"
	"github.com/AshBuk/dabri/v2/internal/services"
	"github.com/AshBuk/dabri/v2/internal/utils"
)

// IPC Handlers - Protocol adapter between Unix socket IPC and Business Services
// Request-response IPC commands (sync, wait-for-result with timeout)
//
// Architecture Flow:
//   CLI client (dabri start-recording)
//       ↓
//   Unix Socket (utils.GetDefaultSocketPath())
//       ↓
//   IPC Server (internal/ipc package)
//       ↓
//   IPC Handlers (this file) ← protocol adapter layer
//       ↓
//   Handler methods (handlers.go) ← business logic adapter
//       ↓
//   Business Services (AudioService/IOService)

const (
	ipcTranscriptionTimeout = 45 * time.Second
)

// startIPCServer Initializes Unix socket IPC server for CLI client communication
// Creates server on default socket path, registers command handlers, starts listening
func (a *App) startIPCServer() error {
	if a.Services == nil {
		return fmt.Errorf("services not initialized")
	}
	socketPath := utils.GetDefaultSocketPath()
	server := ipc.NewServer(socketPath, a.Runtime.Logger)
	a.registerIPCHandlers(server)
	if err := server.Start(); err != nil {
		return err
	}
	a.ipcServer = server
	return nil
}

// registerIPCHandlers registers IPC command handlers with server
// Maps command names to handler functions for request routing
func (a *App) registerIPCHandlers(server *ipc.Server) {
	server.Register("start-recording", a.ipcHandleStartRecording)
	server.Register("stop-recording", a.ipcHandleStopRecording)
	server.Register("toggle-recording", a.ipcHandleToggleRecording)
	server.Register("status", a.ipcHandleStatus)
	server.Register("last-transcript", a.ipcHandleLastTranscript)
	server.Register("set-model", a.ipcHandleSetModel)
	server.Register("delete-model", a.ipcHandleDeleteModel)
}

// ipcHandleStartRecording Command handler - starts audio recording via handlers.go
// Returns JSON response with recording state
func (a *App) ipcHandleStartRecording(ipc.Request) (ipc.Response, error) {
	if err := a.handleStartRecording(); err != nil {
		return ipc.Response{}, err
	}
	return ipc.NewSuccessResponse("recording started", map[string]any{
		"recording": true,
	}), nil
}

// ipcHandleStopRecording Command handler - stops recording and waits for transcript
// Synchronous: blocks CLI until transcription ready (45s timeout) or returns last transcript
// Graceful errors: ErrNoRecordingInProgress returns success (not error) for idempotency
func (a *App) ipcHandleStopRecording(ipc.Request) (ipc.Response, error) {
	if err := a.handleStopRecordingAndTranscribe(); err != nil {
		if errors.Is(err, services.ErrNoRecordingInProgress) {
			return ipc.NewSuccessResponse("recording already stopped", map[string]any{
				"recording":  false,
				"transcript": "",
			}), nil
		}
		return ipc.Response{}, err
	}
	transcript, err := a.waitForTranscription(ipcTranscriptionTimeout)
	if err != nil {
		return ipc.NewSuccessResponse("recording stopped", map[string]any{
			"recording":  false,
			"transcript": a.getLastTranscript(),
			"warning":    err.Error(),
		}), nil
	}
	return ipc.NewSuccessResponse("recording stopped", map[string]any{
		"recording":  false,
		"transcript": transcript,
	}), nil
}

// ipcHandleToggleRecording Command handler - toggles recording state
// Same logic as tray toggle (see factory_wirer.go makeToggleCallback)
func (a *App) ipcHandleToggleRecording(ipc.Request) (ipc.Response, error) {
	if a.Services == nil || a.Services.Audio == nil {
		return ipc.Response{}, fmt.Errorf("audio service not available")
	}
	if a.Services.Audio.IsRecording() {
		return a.ipcHandleStopRecording(ipc.Request{})
	}
	return a.ipcHandleStartRecording(ipc.Request{})
}

// ipcHandleStatus Command handler - returns current state and configuration
// Non-blocking: returns immediately without waiting for transcription
func (a *App) ipcHandleStatus(ipc.Request) (ipc.Response, error) {
	recording := false
	if a.Services != nil && a.Services.Audio != nil {
		recording = a.Services.Audio.IsRecording()
	}

	status := map[string]any{
		"recording": recording,
	}

	// Add config if available
	if a.Services != nil && a.Services.Config != nil {
		cfg := a.Services.Config.GetConfig()
		if cfg != nil {
			status["config"] = map[string]any{
				"model":        cfg.General.WhisperModel,
				"language":     cfg.General.Language,
				"output_mode":  cfg.Output.DefaultMode,
				"audio_method": cfg.Audio.RecordingMethod,
			}
			status["hotkeys"] = map[string]any{
				"start_stop":  cfg.Hotkeys.StartRecording,
				"show_config": cfg.Hotkeys.ShowConfig,
			}
			status["websocket"] = map[string]any{
				"enabled": cfg.WebServer.Enabled,
				"host":    cfg.WebServer.Host,
				"port":    cfg.WebServer.Port,
			}
		}
	}
	return ipc.NewSuccessResponse("status", status), nil
}

// ipcHandleLastTranscript Command handler - returns last saved transcript
// Non-blocking: returns cached transcript without waiting
func (a *App) ipcHandleLastTranscript(ipc.Request) (ipc.Response, error) {
	return ipc.NewSuccessResponse("last transcript", map[string]any{
		"transcript": a.getLastTranscript(),
	}), nil
}

// ipcHandleSetModel Command handler - switches whisper model and persists config
func (a *App) ipcHandleSetModel(req ipc.Request) (ipc.Response, error) {
	if a.Services == nil || a.Services.Audio == nil || a.Services.Config == nil {
		return ipc.Response{}, fmt.Errorf("services not available")
	}
	modelID := req.Params["model"]
	if modelID == "" {
		return ipc.Response{}, fmt.Errorf("missing required parameter: model")
	}
	// Remember previous model so we can rollback on persist failure
	previousModel := ""
	if cfg := a.Services.Config.GetConfig(); cfg != nil {
		previousModel = cfg.General.WhisperModel
	}
	if err := a.Services.Audio.SwitchModel(a.Runtime.Ctx, modelID); err != nil {
		return ipc.Response{}, fmt.Errorf("model switch failed: %w", err)
	}
	if err := a.Services.Config.UpdateWhisperModel(modelID); err != nil {
		// Rollback engine to previous model so runtime state matches config
		if previousModel != "" {
			if rbErr := a.Services.Audio.SwitchModel(a.Runtime.Ctx, previousModel); rbErr != nil {
				a.Runtime.Logger.Error("Failed to rollback model after config save error: %v", rbErr)
			}
		}
		return ipc.Response{}, fmt.Errorf("failed to persist model setting: %w", err)
	}
	if a.Services.UI != nil {
		a.Services.UI.UpdateSettings(a.Services.Config.GetConfig())
	}
	return ipc.NewSuccessResponse("model switched", map[string]any{
		"model": modelID,
	}), nil
}

// ipcHandleDeleteModel Command handler - deletes a downloaded whisper model
func (a *App) ipcHandleDeleteModel(req ipc.Request) (ipc.Response, error) {
	if a.Services == nil || a.Services.Audio == nil {
		return ipc.Response{}, fmt.Errorf("services not available")
	}
	modelID := req.Params["model"]
	if modelID == "" {
		return ipc.Response{}, fmt.Errorf("missing required parameter: model")
	}
	if err := a.Services.Audio.DeleteModel(modelID); err != nil {
		return ipc.Response{}, fmt.Errorf("model delete failed: %w", err)
	}
	return ipc.NewSuccessResponse("model deleted", map[string]any{
		"model": modelID,
	}), nil
}

// waitForTranscription Blocks until transcription ready or timeout expires
// Used by ipcHandleStopRecording to provide synchronous CLI response
func (a *App) waitForTranscription(timeout time.Duration) (string, error) {
	if a.Services == nil || a.Services.IO == nil {
		return "", fmt.Errorf("io service not available")
	}
	return a.Services.IO.WaitForTranscription(timeout)
}

// getLastTranscript Returns cached transcript without waiting
// Used by status/last-transcript commands for immediate response
func (a *App) getLastTranscript() string {
	if a.Services == nil || a.Services.Audio == nil {
		return ""
	}
	return a.Services.Audio.GetLastTranscript()
}
