// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"os"
	"path/filepath"

	"github.com/AshBuk/dabri/v2/config"
)

const (
	// AppDataDirName is the application data directory name
	AppDataDirName = "dabri"
	// ModelsDirName is the subdirectory for model files
	ModelsDirName = "models"
)

// Implements the logic for resolving the path to the bundled model
// based on the execution environment (e.g., AppImage, system package, dev)
type ModelPathResolver struct {
	config        *config.Config
	modelFileName string
}

// NewModelPathResolver creates a resolver for locating the given model file
func NewModelPathResolver(config *config.Config, fileName string) *ModelPathResolver {
	return &ModelPathResolver{
		config:        config,
		modelFileName: fileName,
	}
}

// GetBundledModelPath returns the path to the model file.
// Search order:
//  1. AppImage bundled path ($APPDIR/sources/language-models/)
//  2. User data directory (~/.local/share/dabri/models/)
//  3. Development path (sources/language-models/)
func (r *ModelPathResolver) GetBundledModelPath() string {
	// 1. AppImage environment
	if appDir := os.Getenv("APPDIR"); appDir != "" {
		path := filepath.Join(appDir, "sources/language-models", r.modelFileName)
		if fileExists(path) {
			return path
		}
	}
	// 2. User data directory (XDG_DATA_HOME or ~/.local/share)
	userDataPath := r.getUserDataModelPath()
	if fileExists(userDataPath) {
		return userDataPath
	}
	// 3. Development path
	devPath := filepath.Join("sources/language-models", r.modelFileName)
	if fileExists(devPath) {
		return devPath
	}
	// Return user data path as default (for download location)
	return userDataPath
}

// GetUserDataModelPath returns the path where model should be downloaded
// This is XDG_DATA_HOME/dabri/models/<model-file-name>
func (r *ModelPathResolver) GetUserDataModelPath() string {
	return r.getUserDataModelPath()
}

// getUserDataModelPath returns the XDG-compliant user data path for the model
func (r *ModelPathResolver) getUserDataModelPath() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current directory if home is unavailable
			return filepath.Join(ModelsDirName, r.modelFileName)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, AppDataDirName, ModelsDirName, r.modelFileName)
}

// fileExists checks if a file exists and is not a directory
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
