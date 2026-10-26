//go:build !cgo || nocgo

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package stt

import (
	"context"
	"errors"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// WhisperEngine is a no-cgo stub that fails gracefully when CGO is disabled
type WhisperEngine struct{}

// NewWhisperEngine returns an error indicating that CGO is required
func NewWhisperEngine(config func() *config.Config, modelPath string, _ ...logger.Logger) (*WhisperEngine, error) {
	return nil, errors.New("whisper engine is unavailable: built without cgo")
}

// Close is a no-op in the stub implementation
func (w *WhisperEngine) Close() error { return nil }

// Transcribe returns an error in the stub implementation
func (w *WhisperEngine) Transcribe(audioFile string) (string, error) {
	return "", errors.New("transcription unavailable: built without cgo")
}

// TranscribeWithContext returns an error in the stub implementation
func (w *WhisperEngine) TranscribeWithContext(_ context.Context, _ string) (string, error) {
	return "", errors.New("transcription unavailable: built without cgo")
}
