// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

// Provides a high-level facade for interacting with the speech-to-text
// functionality. It abstracts the underlying implementation details of model management
// and transcription, exposing a simple entry point for external packages.
//
// Subpackages:
//   - interfaces: Define contracts for model lifecycle and transcription flows
//   - manager:    Provide the default model manager implementation
//   - providers:  Supply adapters for resolving bundled and custom model paths
package whisper

import (
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/whisper/interfaces"
	"github.com/AshBuk/dabri/v2/whisper/manager"
)

// Re-export key interfaces for convenience
type (
	ModelManager = interfaces.ModelManager
)

// Create a new manager for the bundled Whisper model
func NewModelManager(config *config.Config) ModelManager {
	return manager.NewModelManager(config)
}
