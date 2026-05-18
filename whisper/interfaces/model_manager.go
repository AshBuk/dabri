// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package interfaces

import (
	"context"

	"github.com/AshBuk/dabri/v2/config"
)

// Defines the contract for managing the lifecycle of Whisper models
type ModelManager interface {
	// Initialize and validate the configured model
	Initialize(ctx context.Context) error
	// Return the absolute path to the validated model file
	GetModelPath(ctx context.Context) (string, error)
	// Check if the model file at the given path is valid
	ValidateModel(modelPath string) error
	// Switch to a different model by ID, downloading if needed. Returns new model path.
	SwitchModel(ctx context.Context, modelID string) (string, error)
	// Delete a downloaded model by ID. Cannot delete the active model.
	DeleteModel(modelID string) error
}

// Represents a whisper.Model type without a direct CGO dependency
type WhisperModel interface{}

// Defines the contract for a speech-to-text transcription engine
type WhisperEngine interface {
	// Transcribe an audio file and return the resulting text
	Transcribe(audioFile string) (string, error)
	// Add cancellation support for long running operations
	TranscribeWithContext(ctx context.Context, audioFile string) (string, error)
	// Close the engine and release any associated resources
	Close() error
	// Return the underlying model object
	GetModel() WhisperModel
	// Return the configuration used by the engine
	GetConfig() *config.Config
}
