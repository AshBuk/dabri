// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package factory

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/AshBuk/dabri/v2/audio/interfaces"
	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/audio/recorders"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// AudioRecorderFactory creates audio recorder instances based on configuration
// Specialized subfactory used by ServiceFactory hierarchy
//
// Factory Hierarchy:
//
//	ServiceFactory (internal/services/factory.go)
//	    │
//	    ├── Stage 1: FactoryComponents
//	    │       │
//	    │       └── uses → AudioRecorderFactory (this file)
//	    │                     │
//	    │                     └── creates → ArecordRecorder or FFmpegRecorder
//	    │
//	    ├── Stage 2: FactoryAssembler
//	    └── Stage 3: FactoryWirer
//
// Design:
//   - Factory Method: creates different recorder implementations (arecord, ffmpeg)
//   - Strategy: recorders implement common AudioRecorder interface
//   - Fallback: automatic fallback to working recorder method
//   - Diagnostics Pattern: pre-creation system validation
//
// Usage:
//
//	GetRecorder(config, logger, tempManager)             // simple creation
//	GetRecorderWithFallback(config, logger, tempManager) // with auto-fallback
type AudioRecorderFactory struct {
	config      *config.Config
	logger      logger.Logger
	tempManager *processing.TempFileManager
}

// NewAudioRecorderFactory Constructor - initializes factory with required dependencies
func NewAudioRecorderFactory(config *config.Config, logger logger.Logger, tempManager *processing.TempFileManager) *AudioRecorderFactory {
	return &AudioRecorderFactory{ // Injected:
		config:      config,      // (recorder selection)
		logger:      logger,      // (diagnostics)
		tempManager: tempManager, // (file handling)
	}
}

// CreateRecorder Factory Method implementation - creates recorder instance from config
// Returns AudioRecorder interface with arecord or ffmpeg concrete implementation
// Runs diagnostics before creation to aid troubleshooting if recorder fails later
func (f *AudioRecorderFactory) CreateRecorder() (interfaces.AudioRecorder, error) {
	f.DiagnoseAudioSystem()
	switch f.config.Audio.RecordingMethod {
	case "arecord":
		return recorders.NewArecordRecorder(f.config, f.logger, f.tempManager), nil
	case "ffmpeg":
		return recorders.NewFFmpegRecorder(f.config, f.logger, f.tempManager), nil
	default:
		return nil, fmt.Errorf("unsupported recording method: %s", f.config.Audio.RecordingMethod)
	}
}

// DiagnoseAudioSystem Pre-creation validation - checks tools and devices before recording
// Logs warnings for missing tools (ffmpeg/arecord) and available audio devices
// Helps troubleshoot "recorder created but fails to record" scenarios
func (f *AudioRecorderFactory) DiagnoseAudioSystem() {
	f.logger.Info("[AUDIO DIAGNOSTICS] Recording method: %s, Device: %s",
		f.config.Audio.RecordingMethod, f.config.Audio.Device)
	switch f.config.Audio.RecordingMethod {
	case "ffmpeg":
		if _, err := exec.LookPath("ffmpeg"); err != nil {
			f.logger.Warning("[AUDIO DIAGNOSTICS] WARNING: ffmpeg not found: %v", err)
		}
		// Check PulseAudio sources for ffmpeg
		if out, err := exec.Command("pactl", "list", "short", "sources").Output(); err == nil {
			f.logger.Info("[AUDIO DIAGNOSTICS] PulseAudio sources available:")
			f.logger.Info("%s", string(out))
		} else {
			f.logger.Warning("[AUDIO DIAGNOSTICS] WARNING: Cannot list PulseAudio sources: %v", err)
		}
	case "arecord":
		if _, err := exec.LookPath("arecord"); err != nil {
			f.logger.Warning("[AUDIO DIAGNOSTICS] WARNING: arecord not found: %v", err)
		}
		// Check ALSA devices for arecord
		if out, err := exec.Command("arecord", "-l").Output(); err == nil {
			f.logger.Info("[AUDIO DIAGNOSTICS] ALSA capture devices:")
			f.logger.Info("%s", string(out))
		} else {
			f.logger.Warning("[AUDIO DIAGNOSTICS] WARNING: Cannot list ALSA devices: %v", err)
		}
	}
}

// TestRecorderMethod Validates recorder by attempting short capture (0.5-1s)
// Creates temporary config with tested method, runs actual audio capture command
// Used by CreateRecorderWithFallback to find working method on multi-recorder systems
// Security: uses command allowlist and argument sanitization (config.IsCommandAllowed)
func (f *AudioRecorderFactory) TestRecorderMethod(method string) error {
	testConfig := *f.config
	testConfig.Audio.RecordingMethod = method
	f.logger.Info("[AUDIO TEST] Testing %s recorder...", method)
	var testArgs []string
	var cmdName string
	switch method {
	case "ffmpeg":
		cmdName = "ffmpeg"
		testArgs = []string{
			"-y", "-f", "pulse", "-i", testConfig.Audio.Device,
			"-ar", "16000", "-ac", "1", "-acodec", "pcm_s16le",
			"-t", "0.5", "-f", "null", "-"}
	case "arecord":
		cmdName = "arecord"
		testArgs = []string{
			"-D", testConfig.Audio.Device, "-f", "S16_LE",
			"-r", "16000", "-c", "1", "-d", "1", "/dev/null"}
	default:
		return fmt.Errorf("unsupported test method: %s", method)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if !config.IsCommandAllowed(&testConfig, cmdName) {
		return fmt.Errorf("command not allowed: %s", cmdName)
	}
	safeArgs := config.SanitizeCommandArgs(testArgs)
	// #nosec G204 -- Safe: command is allowlisted and arguments are sanitized.
	cmd := exec.CommandContext(ctx, cmdName, safeArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		f.logger.Error("[AUDIO TEST] %s test failed: %v", method, err)
		f.logger.Info("[AUDIO TEST] %s output: %s", method, string(output))
		return fmt.Errorf("%s test failed: %w", method, err)
	}
	f.logger.Info("[AUDIO TEST] %s test passed", method)
	return nil
}

// CreateRecorderWithFallback Resilient creation - tries configured method, falls back if test fails
// Fallback sequence: configured method → arecord → ffmpeg (skips original if already tested)
// Mutates config.Audio.RecordingMethod on successful fallback for session persistence
// Use case: handle systems where configured recorder is unavailable but alternative exists
func (f *AudioRecorderFactory) CreateRecorderWithFallback() (interfaces.AudioRecorder, error) {
	recorder, err := f.CreateRecorder()
	if err != nil {
		return nil, err
	}
	testErr := f.TestRecorderMethod(f.config.Audio.RecordingMethod)
	if testErr == nil {
		f.logger.Info("[AUDIO] Using configured recorder: %s", f.config.Audio.RecordingMethod)
		return recorder, nil
	}
	f.logger.Warning("[AUDIO] Configured method %s failed test: %v", f.config.Audio.RecordingMethod, testErr)
	fallbacks := []string{"arecord", "ffmpeg"}
	originalMethod := f.config.Audio.RecordingMethod
	for _, method := range fallbacks {
		if method == originalMethod {
			continue
		}
		if f.TestRecorderMethod(method) == nil {
			f.logger.Info("[AUDIO] Automatically switching to %s (fallback)", method)
			f.config.Audio.RecordingMethod = method
			return f.CreateRecorder()
		}
	}
	return nil, fmt.Errorf("no working audio recorder found (tested: %s)", f.config.Audio.RecordingMethod)
}

// GetRecorder Convenience function - one-line recorder creation without factory instance
// Calls CreateRecorder() - no fallback, fails if configured method unavailable
func GetRecorder(config *config.Config, logger logger.Logger, tempManager *processing.TempFileManager) (interfaces.AudioRecorder, error) {
	factory := NewAudioRecorderFactory(config, logger, tempManager)
	return factory.CreateRecorder()
}

// GetRecorderWithFallback Convenience function - one-line recorder creation with auto-fallback
// Calls CreateRecorderWithFallback() - automatically tries alternative methods on failure
func GetRecorderWithFallback(config *config.Config, logger logger.Logger, tempManager *processing.TempFileManager) (interfaces.AudioRecorder, error) {
	factory := NewAudioRecorderFactory(config, logger, tempManager)
	return factory.CreateRecorderWithFallback()
}
