// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package processing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AshBuk/dabri/v2/internal/logger"
)

// Manages the lifecycle of temporary audio files
type TempFileManager struct {
	tempFiles      map[string]time.Time
	mutex          sync.Mutex
	cleanupTimeout time.Duration
	running        bool
	stopChan       chan bool
	stopClosed     bool
	logger         logger.Logger
	wg             sync.WaitGroup
}

// Create a new TempFileManager instance
func NewTempFileManager(cleanupTimeout time.Duration, loggers ...logger.Logger) *TempFileManager {
	var logSink logger.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		logSink = loggers[0]
	} else {
		logSink = logger.NewDefaultLogger(logger.WarningLevel)
	}
	return &TempFileManager{
		tempFiles:      make(map[string]time.Time),
		cleanupTimeout: cleanupTimeout,
		stopChan:       make(chan bool),
		stopClosed:     false,
		logger:         logSink,
	}
}

// Start the background cleanup routine
func (t *TempFileManager) Start() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.running {
		return
	}
	t.running = true
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.cleanupRoutine()
	}()
}

// Add a file to the manager for tracking and eventual cleanup
func (t *TempFileManager) AddFile(path string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.tempFiles[path] = time.Now()
}

// Remove a file from tracking and optionally delete it from disk
func (t *TempFileManager) RemoveFile(path string, shouldDelete bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	delete(t.tempFiles, path)

	if shouldDelete {
		if _, err := os.Stat(path); err == nil {
			if err := os.Remove(path); err != nil {
				t.logger.Warning("Error removing temp file %s: %v", path, err)
			}
		}
	}
}

// Run a background routine to periodically remove old files
func (t *TempFileManager) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.cleanupOldFiles()
		case <-t.stopChan:
			t.mutex.Lock()
			t.running = false
			t.mutex.Unlock()
			return
		}
	}
}

// Remove any tracked files that are older than the configured timeout
func (t *TempFileManager) cleanupOldFiles() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	now := time.Now()
	for path, timestamp := range t.tempFiles {
		if now.Sub(timestamp) > t.cleanupTimeout {
			if _, err := os.Stat(path); err == nil {
				if err := os.Remove(path); err != nil {
					t.logger.Warning("Error removing old temp file %s: %v", path, err)
				}
			}
			delete(t.tempFiles, path)
		}
	}
}

// Remove all tracked temporary files from disk immediately
func (t *TempFileManager) CleanupAll() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	for path := range t.tempFiles {
		if _, err := os.Stat(path); err == nil {
			if err := os.Remove(path); err != nil {
				t.logger.Warning("Error removing temp file %s: %v", path, err)
			}
		}
		delete(t.tempFiles, path)
	}
}

// Stop the background cleanup routine
func (t *TempFileManager) Stop() {
	t.mutex.Lock()
	if t.stopClosed {
		t.mutex.Unlock()
		return
	}
	t.stopClosed = true
	ch := t.stopChan
	t.mutex.Unlock()

	if ch != nil {
		close(ch)
	}
	t.wg.Wait()
}

// Create a new temporary .wav file and register it for cleanup
func (t *TempFileManager) CreateTempWav(baseDir string) (string, error) {
	// Determine the directory for the temporary file
	dir := baseDir
	if dir == "" {
		dir = os.TempDir()
	}
	// Ensure the target directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	// Generate a unique filename based on the current timestamp
	timestamp := time.Now().Format("20060102-150405")
	path := filepath.Join(dir, fmt.Sprintf("audio_%s.wav", timestamp))
	cleaned := filepath.Clean(path)
	// Ensure the path remains within the base directory to prevent traversal attacks
	if !strings.HasPrefix(cleaned+string(os.PathSeparator), filepath.Clean(dir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe temp file path outside base dir")
	}

	// Pre-create the file to reserve the path
	// #nosec G304 -- Safe: path is constructed, cleaned, and verified under a controlled base directory.
	f, err := os.OpenFile(cleaned, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	_ = f.Close()

	// Track the file for automatic cleanup
	t.AddFile(cleaned)
	return cleaned, nil
}
