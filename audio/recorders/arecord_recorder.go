// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package recorders

import (
	"fmt"

	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// Implements the AudioRecorder interface using the `arecord` command-line tool
type ArecordRecorder struct {
	BaseRecorder
}

// Create a new instance of the arecord-based recorder
func NewArecordRecorder(config *config.Config, logger logger.Logger, tempManager *processing.TempFileManager) *ArecordRecorder {
	return &ArecordRecorder{
		BaseRecorder: NewBaseRecorder(config, logger, tempManager),
	}
}

// Start an audio recording using the `arecord` command
func (a *ArecordRecorder) StartRecording() error {
	// Build the command-line arguments for arecord
	formatArg := a.getArecordFormat()

	baseArgs := []string{
		"-D", a.config.Audio.Device,
		"-f", formatArg,
		"-r", fmt.Sprintf("%d", a.config.Audio.SampleRate),
		"-c", "1",
	}

	// Ensure a WAV header is used for both file and buffer modes
	baseArgs = append(baseArgs, "-t", "wav")

	// Use the BaseRecorder to execute the command and manage the process
	return a.ExecuteRecordingCommand("arecord", baseArgs)
}

// Convert a format string from the application's convention (e.g., s16le)
// to the format required by the `arecord` tool (e.g., S16_LE)
func (a *ArecordRecorder) getArecordFormat() string {
	format := a.config.Audio.Format
	switch format {
	case "s16le":
		return "S16_LE"
	case "s24le":
		return "S24_LE"
	case "s32le":
		return "S32_LE"
	default:
		return format
	}
}
