//go:build cgo

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package stt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/utils"
	"github.com/go-audio/wav"
)

// backend runs a loaded model on decoded 16 kHz mono samples
type backend interface {
	transcribe(samples []float32) (string, error)
	Close() error
}

// Engine provides an interface for speech recognition with Whisper or Parakeet models.
// It validates and decodes the audio, and the model's backend runs the inference
type Engine struct {
	backend backend
	logger  logger.Logger
}

// NewEngine initializes and loads a new model from the given path,
// picking the backend from the model's catalog entry.
// Return an error if the model file is not found or fails to load
func NewEngine(config func() *config.Config, modelPath string, loggers ...logger.Logger) (*Engine, error) {
	if !utils.IsValidFile(modelPath) {
		return nil, fmt.Errorf("model not found: %s", modelPath)
	}
	loadBackends()
	var logSink logger.Logger = logger.NewDefaultLogger(logger.WarningLevel)
	if len(loggers) > 0 && loggers[0] != nil {
		logSink = loggers[0]
	}
	var b backend
	var err error
	if m := constants.ModelByFileName(filepath.Base(modelPath)); m != nil && m.Engine == constants.EngineParakeet {
		b, err = newParakeetModel(modelPath, logSink)
	} else {
		b, err = newWhisperModel(config, modelPath, logSink)
	}
	if err != nil {
		return nil, err
	}
	return &Engine{backend: b, logger: logSink}, nil
}

// Close releases the resources associated with the loaded model
func (e *Engine) Close() error {
	return e.backend.Close()
}

// Transcribe performs speech-to-text conversion on the given audio file.
// Handle file validation, audio loading, context management, and processing
func (e *Engine) Transcribe(audioFile string) (string, error) {
	if !utils.IsValidFile(audioFile) {
		return "", fmt.Errorf("audio file not found or invalid: %s", audioFile)
	}
	// Enforce a reasonable file size limit to prevent excessive memory usage
	fileSize, err := utils.GetFileSize(audioFile)
	if err != nil {
		return "", fmt.Errorf("error checking audio file size: %w", err)
	}
	const maxFileSize int64 = 50 * 1024 * 1024 // 50MB
	if fileSize > maxFileSize {
		return "", fmt.Errorf("audio file too large (%d bytes), max allowed is %d bytes", fileSize, maxFileSize)
	}
	// Check for sufficient disk space before proceeding
	if err := utils.CheckDiskSpace(audioFile); err != nil {
		return "", fmt.Errorf("insufficient disk space: %w", err)
	}
	audioData, err := e.loadAudioData(audioFile)
	if err != nil {
		return "", fmt.Errorf("failed to load audio data: %w", err)
	}
	// Guard against empty input: whisper.cpp bindings panic on a zero-length
	// sample slice (index out of range).
	if len(audioData) == 0 {
		return "", fmt.Errorf("no audio samples decoded from %s", audioFile)
	}
	text, err := e.backend.transcribe(audioData)
	if err != nil {
		return "", err
	}
	return utils.SanitizeTranscript(text), nil
}

// TranscribeWithContext performs speech-to-text conversion.
// whisper.cpp and libparakeet processing cannot be interrupted safely, so context cancellation is
// checked before and after the synchronous call instead of spawning hidden work.
func (e *Engine) TranscribeWithContext(ctx context.Context, audioFile string) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("transcription cancelled: %w", ctx.Err())
	default:
	}

	txt, err := e.Transcribe(audioFile)
	if err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("transcription cancelled: %w", ctx.Err())
	default:
		return txt, nil
	}
}

// Open a WAV file, decode it, and convert the PCM data
// into the float32 sample format both backends expect
func (e *Engine) loadAudioData(audioFile string) ([]float32, error) {
	// Sanitize path to prevent directory traversal and other file-based attacks
	clean := filepath.Clean(audioFile)
	if clean != audioFile || strings.Contains(clean, "..") {
		return nil, fmt.Errorf("invalid audio file path")
	}
	// #nosec G304 -- Path is sanitized and validated by the caller.
	file, err := os.Open(clean)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			e.logger.Warning("Failed to close audio file: %v", err)
		}
	}()
	decoder := wav.NewDecoder(file)
	if decoder == nil {
		return nil, fmt.Errorf("failed to create WAV decoder")
	}
	audioBuffer, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to read audio buffer: %w", err)
	}
	// Convert integer PCM samples to float32 samples normalized to [-1.0, 1.0]
	samples := make([]float32, audioBuffer.NumFrames())
	for i := 0; i < audioBuffer.NumFrames(); i++ {
		intSample := audioBuffer.Data[i]
		samples[i] = float32(intSample) / 32768.0
	}
	return samples, nil
}
