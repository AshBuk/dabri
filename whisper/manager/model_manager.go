// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package manager

import (
	"context"
	"fmt"
	"os"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/utils"
	"github.com/AshBuk/dabri/v2/whisper/providers"
)

// Manages Whisper model lifecycle: resolution, download, and validation
type ModelManager struct {
	config *config.Config
	logger logger.Logger
}

// Create a new manager responsible for the Whisper model.
// An optional logger enables download-progress reporting.
func NewModelManager(config *config.Config, loggers ...logger.Logger) *ModelManager {
	m := &ModelManager{config: config}
	if len(loggers) > 0 {
		m.logger = loggers[0]
	}
	return m
}

// Initialize validates the configured model is present, downloading if needed
func (m *ModelManager) Initialize(ctx context.Context) error {
	_, err := m.resolveModel(ctx, m.configuredModelID())
	return err
}

// GetModelPath returns the absolute path to the configured model file
func (m *ModelManager) GetModelPath(ctx context.Context) (string, error) {
	return m.resolveModel(ctx, m.configuredModelID())
}

// SwitchModel resolves (and downloads if needed) a model by ID, returning its path.
func (m *ModelManager) SwitchModel(ctx context.Context, modelID string) (string, error) {
	return m.resolveModel(ctx, modelID)
}

// ValidateModel checks if the model file at the given path is valid (basic size check)
func (m *ModelManager) ValidateModel(modelPath string) error {
	if !utils.IsValidFile(modelPath) {
		return fmt.Errorf("model file not found: %s", modelPath)
	}
	size, err := utils.GetFileSize(modelPath)
	if err != nil {
		return fmt.Errorf("error checking model file: %w", err)
	}
	// Sanity check: the model should be at least 10MB
	if size < 10*1024*1024 {
		return fmt.Errorf("model file is too small (%d bytes), might be corrupted", size)
	}
	return nil
}

// resolveModel finds or downloads a model by ID, returning its validated path
func (m *ModelManager) resolveModel(ctx context.Context, modelID string) (string, error) {
	def := constants.ModelByID(modelID)
	if def == nil {
		return "", fmt.Errorf("unknown model ID: %s", modelID)
	}

	resolver := providers.NewModelPathResolver(m.config, def.FileName)

	// Check bundled / user data / dev paths
	modelPath := resolver.GetBundledModelPath()
	if utils.IsValidFile(modelPath) {
		if err := m.ValidateModel(modelPath); err == nil {
			return modelPath, nil
		}
	}

	// Download to user data directory
	downloadPath := resolver.GetUserDataModelPath()
	dl := providers.NewModelDownloaderForURL(def.URL, def.MinSize)
	if m.logger != nil {
		m.logger.Info("Downloading model %s (%s)...", modelID, def.FileName)
		dl.WithProgress(func(downloaded, total int64) {
			const mb = 1024 * 1024
			if total > 0 {
				m.logger.Info("Downloading model %s: %d%% (%.1f/%.1f MB)",
					modelID, downloaded*100/total, float64(downloaded)/mb, float64(total)/mb)
			} else {
				m.logger.Info("Downloading model %s: %.1f MB", modelID, float64(downloaded)/mb)
			}
		})
	}
	if err := dl.Download(ctx, downloadPath); err != nil {
		return "", fmt.Errorf("failed to download model %s: %w", modelID, err)
	}

	if err := m.ValidateModel(downloadPath); err != nil {
		return "", fmt.Errorf("downloaded model %s failed validation: %w", modelID, err)
	}
	return downloadPath, nil
}

// DeleteModel removes a downloaded model file by ID.
// Cannot delete the currently active model or bundled models.
func (m *ModelManager) DeleteModel(modelID string) error {
	def := constants.ModelByID(modelID)
	if def == nil {
		return fmt.Errorf("unknown model ID: %s", modelID)
	}
	if modelID == m.configuredModelID() {
		return fmt.Errorf("cannot delete active model: %s", modelID)
	}
	resolver := providers.NewModelPathResolver(m.config, def.FileName)
	path := resolver.GetUserDataModelPath()
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("model %s is not downloaded", modelID)
		}
		return fmt.Errorf("failed to delete model %s: %w", modelID, err)
	}
	return nil
}

// configuredModelID returns the model ID from config, falling back to default
func (m *ModelManager) configuredModelID() string {
	if id := m.config.General.WhisperModel; id != "" {
		return id
	}
	return constants.DefaultModelID
}
