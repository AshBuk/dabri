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
package stt

import (
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/stt/interfaces"
	"github.com/AshBuk/dabri/v2/stt/manager"
)

// ModelManager re-exports the model manager interface for convenience
type (
	ModelManager = interfaces.ModelManager
)

// NewModelManager creates a new manager for the bundled Whisper model.
// An optional logger enables download-progress reporting.
func NewModelManager(config func() *config.Config, loggers ...logger.Logger) ModelManager {
	return manager.NewModelManager(config, loggers...)
}
