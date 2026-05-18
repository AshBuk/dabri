// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/AshBuk/dabri/v2/audio/factory"
	"github.com/AshBuk/dabri/v2/audio/interfaces"
	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/utils"
	"github.com/AshBuk/dabri/v2/whisper"
)

// Orchestrates recording, transcription, and output workflows
type AudioService struct {
	logger        logger.Logger
	config        *config.Config
	recorder      interfaces.AudioRecorder
	whisperEngine *whisper.WhisperEngine
	modelManager  whisper.ModelManager
	tempManager   *processing.TempFileManager

	// State management
	mu                       sync.RWMutex
	isRecording              bool
	lastTranscript           string
	audioRecorderNeedsReinit bool

	// Guards whisperEngine while transcription is using it.
	engineMu sync.RWMutex

	// Goroutine ownership: tracks background transcription tasks
	wg sync.WaitGroup

	// Context for operations
	ctx    context.Context
	cancel context.CancelFunc

	// Dependencies
	ui  UIServiceInterface
	io  IOServiceInterface
	cfg ConfigServiceInterface
}

// ErrNoRecordingInProgress indicates a stop request when no session is active.
var ErrNoRecordingInProgress = errors.New("no recording in progress")

// Create a new AudioService instance
func NewAudioService(
	logger logger.Logger,
	config *config.Config,
	recorder interfaces.AudioRecorder,
	whisperEngine *whisper.WhisperEngine,
	modelManager whisper.ModelManager,
	tempManager *processing.TempFileManager,
) *AudioService {
	ctx, cancel := context.WithCancel(context.Background())

	return &AudioService{
		logger:        logger,
		config:        config,
		recorder:      recorder,
		whisperEngine: whisperEngine,
		modelManager:  modelManager,
		tempManager:   tempManager,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Wire dependencies to prevent circular imports during initialization
func (as *AudioService) SetDependencies(ui UIServiceInterface, io IOServiceInterface) {
	as.ui = ui
	as.io = io
}

// Wire config service for runtime setting persistence
func (as *AudioService) SetConfig(cfg ConfigServiceInterface) { as.cfg = cfg }

// HandleStartRecording starts audio recording
func (as *AudioService) HandleStartRecording() error {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.logger.Info("Starting recording...")

	// Ensure model is available
	if err := as.ensureModelAvailable(); err != nil {
		as.logger.Error("Model not available: %v", err)
		as.setUIError(constants.MsgModelUnavailable)
		return fmt.Errorf("model not available: %w", err)
	}
	// Ensure audio recorder is available
	if err := as.ensureAudioRecorderAvailable(); err != nil {
		as.logger.Error("Audio recorder not available: %v", err)
		as.setUIError(constants.MsgRecorderUnavailable)
		return fmt.Errorf("audio recorder not available: %w", err)
	}
	// Standard recording
	return as.startStandardRecording()
}

// HandleStopRecording stops recording and starts async transcription.
func (as *AudioService) HandleStopRecording() error {
	audioFile, err := as.stopAndPrepareTranscription(true)
	if err != nil || audioFile == "" {
		return err
	}
	select {
	case <-as.ctx.Done():
		as.logger.Warning("Shutdown in progress, skipping transcription")
		return nil
	default:
	}
	if as.io != nil {
		as.io.BeginTranscription()
	}
	as.wg.Add(1)
	go func() {
		defer as.wg.Done()
		as.transcribeAsync(audioFile)
	}()
	return nil
}

// HandleStopRecordingSync stops recording and returns the transcript.
func (as *AudioService) HandleStopRecordingSync(ctx context.Context) (string, error) {
	audioFile, err := as.stopAndPrepareTranscription(false)
	if err != nil || audioFile == "" {
		return "", err
	}
	// Always release the recorded WAV; TempFileManager would eventually do
	// it, but eager cleanup keeps disk usage bounded under heavy API use.
	defer as.cleanupRecording(audioFile)

	select {
	case <-as.ctx.Done():
		return "", fmt.Errorf("shutdown in progress")
	default:
	}

	transcript, err := as.transcribeWithCurrentEngine(ctx, audioFile)
	if err != nil {
		return "", err
	}
	sanitized := utils.SanitizeTranscript(transcript)
	as.mu.Lock()
	as.lastTranscript = sanitized
	as.mu.Unlock()
	return sanitized, nil
}

// cleanupRecording removes the captured WAV. Errors are ignored: temp
// files are also swept by TempFileManager on a schedule.
func (as *AudioService) cleanupRecording(audioFile string) {
	if audioFile != "" && as.tempManager != nil {
		as.tempManager.RemoveFile(audioFile, true)
	}
}

// stopAndPrepareTranscription stops recording and returns the captured file.
func (as *AudioService) stopAndPrepareTranscription(swallowStopError bool) (string, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	if !as.isRecording {
		return "", ErrNoRecordingInProgress
	}
	as.logger.Info("Stopping recording and transcribing...")

	audioFile, err := as.recorder.StopRecording()
	if err != nil {
		as.logger.Warning("StopRecording returned error: %v", err)
		as.handleRecordingError(err)

		// Auto-fallback to arecord if using ffmpeg
		if as.config.Audio.RecordingMethod == "ffmpeg" {
			if as.cfg != nil {
				_ = as.cfg.UpdateRecordingMethod("arecord")
			}
			as.config.Audio.RecordingMethod = "arecord"
			as.ClearSession()
			if as.ui != nil {
				as.ui.ShowNotification("Audio Fallback", "Switched to arecord due to ffmpeg capture error. Try recording again.")
				as.ui.UpdateSettings(as.config)
			}
			as.logger.Info("Auto-fallback: switched to arecord due to ffmpeg failure")
		}
		// Ensure state is reset so the hotkey toggle can recover
		as.isRecording = false
		if as.ui != nil {
			as.ui.SetRecordingState(false)
		}
		if swallowStopError {
			// Keep hotkey/tray stop idempotent so users can recover with the next start.
			return "", nil
		}
		return "", err
	}
	as.isRecording = false
	if as.ui != nil {
		as.ui.SetRecordingState(false)
		as.ui.ShowNotification(constants.NotifyRecordingStopped, constants.NotifyRecordingStopMsg)
	}
	if audioFile == "" {
		return "", fmt.Errorf("recording produced no audio file")
	}
	return audioFile, nil
}

// IsRecording returns current recording state
func (as *AudioService) IsRecording() bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.isRecording
}

// GetLastTranscript returns the last transcription result
func (as *AudioService) GetLastTranscript() string {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.lastTranscript
}

// SwitchModel hot-reloads the whisper engine.
func (as *AudioService) SwitchModel(ctx context.Context, modelID string) error {
	as.mu.Lock()
	if as.isRecording {
		as.mu.Unlock()
		return fmt.Errorf("cannot switch model while recording")
	}
	as.mu.Unlock()

	newPath, err := as.modelManager.SwitchModel(ctx, modelID)
	if err != nil {
		return fmt.Errorf("model switch failed: %w", err)
	}

	newEngine, err := whisper.NewWhisperEngine(as.config, newPath, as.logger)
	if err != nil {
		return fmt.Errorf("failed to create engine for model %s: %w", modelID, err)
	}

	as.mu.Lock()
	if as.isRecording {
		as.mu.Unlock()
		_ = newEngine.Close()
		return fmt.Errorf("cannot switch model while recording")
	}
	as.engineMu.Lock()
	oldEngine := as.whisperEngine
	as.whisperEngine = newEngine
	as.engineMu.Unlock()
	as.mu.Unlock()

	if oldEngine != nil {
		_ = oldEngine.Close()
	}
	return nil
}

// DeleteModel removes a downloaded model file.
// Rejects the request if recording is in progress.
func (as *AudioService) DeleteModel(modelID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.isRecording {
		return fmt.Errorf("cannot delete model while recording")
	}
	return as.modelManager.DeleteModel(modelID)
}

// ensureModelAvailable ensures whisper model is ready
func (as *AudioService) ensureModelAvailable() error {
	if as.modelManager == nil {
		return fmt.Errorf("model manager not available")
	}
	// Try to get the model path, which will download if needed
	_, err := as.modelManager.GetModelPath(as.ctx)
	if err != nil {
		as.logger.Info("Model not found locally, checking download...")
		return fmt.Errorf("failed to ensure model available: %w", err)
	}
	return nil
}

// ensureAudioRecorderAvailable ensures audio recorder is ready
func (as *AudioService) ensureAudioRecorderAvailable() error {
	if as.audioRecorderNeedsReinit || as.recorder == nil {
		as.logger.Info("Reinitializing audio recorder...")
		recorder, err := factory.GetRecorder(as.config, as.logger, as.tempManager)
		if err != nil {
			return fmt.Errorf("failed to reinitialize audio recorder: %w", err)
		}
		as.recorder = recorder
		as.audioRecorderNeedsReinit = false
	}
	return nil
}

// Shutdown gracefully shuts down the audio service
func (as *AudioService) Shutdown() error {
	// Use function scope to ensure mutex is released even on panic
	func() {
		as.mu.Lock()
		defer as.mu.Unlock()

		// Cancel context first to signal all operations and prevent new goroutines
		as.cancel()

		if as.isRecording && as.recorder != nil {
			if _, err := as.recorder.StopRecording(); err != nil {
				as.logger.Error("Error stopping recording during shutdown: %v", err)
			}
			as.isRecording = false
		}
	}()

	// Wait for background transcription tasks with timeout
	done := make(chan struct{})
	go func() {
		as.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		as.logger.Info("AudioService shutdown complete")
	case <-time.After(5 * time.Second):
		as.logger.Warning("AudioService shutdown timeout - transcription may still be running")
	}
	return nil
}

// Private helper methods

// startStandardRecording starts standard recording mode
func (as *AudioService) startStandardRecording() error {
	// Set up audio level monitoring
	as.recorder.SetAudioLevelCallback(func(level float64) {
		if as.ui != nil {
			as.ui.UpdateRecordingUI(true, level)
		}
		as.logger.Debug("Audio level: %.2f", level)
	})
	// Start recording
	if err := as.recorder.StartRecording(); err != nil {
		return fmt.Errorf("failed to start recording: %w", err)
	}
	as.isRecording = true
	// Update UI
	if as.ui != nil {
		as.ui.SetRecordingState(true)
		as.ui.ShowNotification(constants.NotifyRecordingStarted, "Speak now...")
	}
	return nil
}

// transcribeAsync performs async transcription.
// Note: The inner goroutine calling TranscribeWithContext is intentionally not tracked
// by the WaitGroup. Whisper.cpp CGO calls cannot be cancelled, so tracking them would
// cause Shutdown() to block for up to 2 minutes. Instead, we accept that the CGO work
// may outlive shutdown (bounded to ~30s max). See whisper/engine.go for details.
func (as *AudioService) transcribeAsync(audioFile string) {
	ctx, cancel := context.WithTimeout(as.ctx, 2*time.Minute)
	defer cancel()

	type result struct {
		transcript string
		err        error
	}

	resultChan := make(chan result, 1)
	go func() {
		transcript, err := as.transcribeWithCurrentEngine(ctx, audioFile)
		select {
		case resultChan <- result{transcript: transcript, err: err}:
		case <-ctx.Done():
		}
	}()
	select {
	case res := <-resultChan:
		// Inner goroutine finished; safe to release the WAV.
		as.cleanupRecording(audioFile)
		as.handleTranscriptionResult(res.transcript, res.err)
	case <-ctx.Done():
		// Inner CGO call may still be reading the file — leave it for
		// TempFileManager rather than risk a use-after-delete.
		as.handleTranscriptionCancellation(ctx.Err())
	}
}

func (as *AudioService) transcribeWithCurrentEngine(ctx context.Context, audioFile string) (string, error) {
	as.engineMu.RLock()
	defer as.engineMu.RUnlock()
	if as.whisperEngine == nil {
		return "", fmt.Errorf("whisper engine not available")
	}
	return as.whisperEngine.TranscribeWithContext(ctx, audioFile)
}

// handleTranscriptionResult processes transcription results
func (as *AudioService) handleTranscriptionResult(transcript string, err error) {
	if err != nil {
		as.handleTranscriptionError(err)
		return
	}
	sanitized := utils.SanitizeTranscript(transcript)
	as.mu.Lock()
	as.lastTranscript = sanitized
	as.mu.Unlock()

	if sanitized == "" {
		as.handleEmptyTranscript()
		return
	}
	as.logger.Info("Transcription result: %s", sanitized)

	// Output text
	if as.io != nil {
		if err := as.io.OutputText(sanitized); err != nil {
			as.logger.Error("Failed to output text: %v", err)
			if as.ui != nil {
				as.ui.SetError("Output failed")
			}
			return
		}
	}
	// Notify IO about completion for clipboard protection release
	if as.io != nil {
		as.io.CompleteTranscription(sanitized)
	}
	// Update UI
	if as.ui != nil {
		as.ui.SetSuccess(constants.MsgTranscriptionComplete)
	}
}

// handleTranscriptionError handles transcription errors
func (as *AudioService) handleTranscriptionError(err error) {
	as.logger.Error("Transcription error: %v", err)
	if as.ui != nil {
		as.ui.SetError(constants.MsgTranscriptionFailed)
		as.ui.ShowNotification(constants.NotifyTranscriptionErr, err.Error())
	}
	// Release clipboard protection
	if as.io != nil {
		as.io.CompleteTranscription("")
	}
}

// handleEmptyTranscript handles empty transcription results
func (as *AudioService) handleEmptyTranscript() {
	as.logger.Info("Empty transcript received")
	if as.ui != nil {
		as.ui.SetError(constants.MsgNoSpeechDetected)
		as.ui.ShowNotification(constants.NotifyNoSpeech, constants.MsgTranscriptionEmpty)
	}
}

// handleRecordingError handles recording errors
func (as *AudioService) handleRecordingError(err error) {
	as.logger.Error("Recording error: %v", err)
	if as.ui != nil {
		as.ui.SetError("Recording error")
		as.ui.ShowNotification("Recording Error", err.Error())
	}
}

// handleTranscriptionCancellation handles transcription cancellation
func (as *AudioService) handleTranscriptionCancellation(err error) {
	as.logger.Warning("Transcription cancelled: %v", err)
	if as.ui != nil {
		as.ui.SetError("Transcription cancelled")
		as.ui.ShowNotification("Transcription Cancelled", "Operation timed out")
	}
	// Release clipboard protection
	if as.io != nil {
		as.io.CompleteTranscription("")
	}
}

// ClearSession clears audio session state and temp files
func (as *AudioService) ClearSession() {
	if as.recorder != nil {
		_ = as.recorder.CleanupFile()
	}
	as.audioRecorderNeedsReinit = true
	as.lastTranscript = ""
}

// setUIError sets UI error state
func (as *AudioService) setUIError(message string) {
	if as.ui != nil {
		as.ui.SetError(message)
	}
}
