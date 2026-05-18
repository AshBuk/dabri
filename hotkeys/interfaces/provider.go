// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package interfaces

import (
	"time"

	"github.com/AshBuk/dabri/v2/internal/platform"
)

// KeyboardEventProvider defines the contract for a keyboard event source
type KeyboardEventProvider interface {
	// Start listening for keyboard events
	Start() error
	// Stop listening for keyboard events
	Stop()
	// Register a hotkey combination and its associated callback function
	RegisterHotkey(hotkey string, callback func() error) error
	// Check if the provider is supported on the current system
	IsSupported() bool
	// Capture a single hotkey combination within a given timeout
	CaptureOnce(timeout time.Duration) (string, error)
	// Check if the provider supports one-shot hotkey capture
	SupportsCaptureOnce() bool
}

// KeyCombination represents a hotkey combination
type KeyCombination struct {
	Modifiers []string // Modifier keys like "ctrl", "alt", "shift"
	Key       string   // The primary, non-modifier key
}

// EnvironmentType is an alias for platform.EnvironmentType to avoid converter boilerplate
type EnvironmentType = platform.EnvironmentType

// Re-exported environment constants for package-local convenience
const (
	EnvironmentUnknown = platform.EnvironmentUnknown
	EnvironmentWayland = platform.EnvironmentWayland
	EnvironmentX11     = platform.EnvironmentX11
)
