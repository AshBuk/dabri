// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package interfaces

import (
	"context"
)

// ModelManager defines the contract for managing the lifecycle of Whisper models
type ModelManager interface {
	// Return the absolute path to the validated model file
	GetModelPath(ctx context.Context) (string, error)
	// Check if the model file at the given path is valid
	ValidateModel(modelPath string) error
	// Switch to a different model by ID, downloading if needed. Returns new model path.
	SwitchModel(ctx context.Context, modelID string) (string, error)
	// Delete a downloaded model by ID. Cannot delete the active model.
	DeleteModel(modelID string) error
}
