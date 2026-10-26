//go:build cgo

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package stt

import (
	"fmt"
	"strings"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// whisperModel runs a Whisper model through the whisper.cpp Go bindings
type whisperModel struct {
	config func() *config.Config // Current config; language changes apply live
	model  whisper.Model
}

// newWhisperModel loads the model and logs system info including GPU backend status
func newWhisperModel(config func() *config.Config, modelPath string, log logger.Logger) (*whisperModel, error) {
	model, err := whisper.New(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load whisper model: %w", err)
	}
	if ctx, err := model.NewContext(); err == nil {
		log.Info("Whisper %s", ctx.SystemInfo())
	}
	return &whisperModel{config: config, model: model}, nil
}

// Close releases the whisper model
func (m *whisperModel) Close() error {
	return m.model.Close()
}

// transcribe decodes 16 kHz mono samples in a fresh context, so calls may overlap
func (m *whisperModel) transcribe(samples []float32) (string, error) {
	context, err := m.model.NewContext()
	if err != nil {
		return "", fmt.Errorf("failed to create whisper context: %w", err)
	}
	// Set the target language for transcription if specified in the config
	if lang := m.config().General.Language; lang != "" {
		if err := context.SetLanguage(lang); err != nil {
			return "", fmt.Errorf("failed to set language: %w", err)
		}
	}

	if err := context.Process(samples, nil, nil, nil); err != nil {
		return "", fmt.Errorf("failed to process audio: %w", err)
	}
	// Collect all text segments from the processed audio
	var transcript strings.Builder
	for {
		segment, err := context.NextSegment()
		if err != nil {
			break // End of segments
		}
		transcript.WriteString(segment.Text)
		transcript.WriteString(" ")
	}
	return transcript.String(), nil
}
