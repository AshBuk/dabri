// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/AshBuk/dabri/v2/config"
)

const (
	// DefaultLockFileName is the default name for the lock file
	DefaultLockFileName = "dabri.lock"
)

// LockFile represents a file-based application lock
type LockFile struct {
	path string
	file *os.File
}

// Create a new lock file instance
func NewLockFile(path string) *LockFile {
	return &LockFile{
		path: path,
	}
}

// GetDefaultLockPath returns the default lock file path
func GetDefaultLockPath() string {
	// Try user runtime directory first (preferred for XDG systems)
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, DefaultLockFileName)
	}

	// Fall back to user config directory
	if configDir, err := config.EnsureConfigDir(); err == nil {
		return filepath.Join(configDir, DefaultLockFileName)
	}

	// Last resort: temp directory
	return filepath.Join(os.TempDir(), DefaultLockFileName)
}

// TryLock attempts to acquire the lock
func (lf *LockFile) TryLock() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(lf.path), 0700); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Try to open/create the lock file
	file, err := os.OpenFile(lf.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create lock file: %w", err)
	}
	// Try to acquire exclusive lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("another instance of dabri is already running")
		}
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	// Write current PID to lock file
	if _, err := file.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write PID to lock file: %w", err)
	}

	lf.file = file
	return nil
}

// Unlock releases the lock
func (lf *LockFile) Unlock() error {
	if lf.file == nil {
		return nil
	}
	// Release the lock
	_ = syscall.Flock(int(lf.file.Fd()), syscall.LOCK_UN)
	// Close the file
	if err := lf.file.Close(); err != nil {
		return fmt.Errorf("failed to close lock file: %w", err)
	}

	lf.file = nil
	// Remove the lock file
	if err := os.Remove(lf.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

	return nil
}

// CheckExistingInstance checks if another instance is running
func (lf *LockFile) CheckExistingInstance() (bool, int, error) {
	file, err := os.Open(lf.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil // No lock file exists
		}
		return false, 0, fmt.Errorf("failed to check lock file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Try to read PID from lock file
	data := make([]byte, 32)
	n, err := file.Read(data)
	if err != nil || n == 0 {
		return false, 0, nil // Invalid lock file
	}

	pidStr := string(data[:n])
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false, 0, nil // Invalid PID
	}
	// Check if process with this PID is still running and is our application
	if isOurProcess(pid) {
		return true, pid, nil // Process is running and is dabri
	}

	return false, pid, nil // Process is not running
}

// isOurProcess checks if the given PID belongs to a dabri process
func isOurProcess(pid int) bool {
	// First check if process exists using syscall.Kill with signal 0
	if err := syscall.Kill(pid, 0); err != nil {
		return false // Process doesn't exist or no permission
	}
	// Check if the process is actually dabri by reading cmdline
	// Validate PID to prevent path traversal
	if pid <= 0 || pid > 4194304 { // Reasonable PID range
		return false
	}
	cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
	cmdlineData, err := os.ReadFile(cmdlinePath) // #nosec G304 - PID is validated to be in safe range above
	if err != nil {
		return false // Can't read cmdline
	}
	// Convert null-terminated string to regular string
	cmdline := string(cmdlineData)
	cmdline = strings.ReplaceAll(cmdline, "\x00", " ")
	cmdline = strings.TrimSpace(cmdline)
	// Check if this is our dabri process
	// Handle direct execution and AppImage cases
	return strings.Contains(cmdline, "dabri") ||
		strings.Contains(cmdline, "AppRun")
}

// GetLockFilePath returns the path to the lock file
func (lf *LockFile) GetLockFilePath() string {
	return lf.path
}
