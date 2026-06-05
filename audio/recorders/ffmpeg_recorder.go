// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package recorders

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

const (
	// Timing constants for warm-up and post-roll
	warmupTimeout      = 2500 * time.Millisecond
	warmupPollInterval = 40 * time.Millisecond
	postRollDelay      = 600 * time.Millisecond
	retryFlushDelay    = 1800 * time.Millisecond

	// Audio validation constants
	minAudioPayloadMs = 50 // Minimum audio payload in milliseconds
)

// Implements the AudioRecorder interface using the `ffmpeg` command-line tool
type FFmpegRecorder struct {
	BaseRecorder
	recordingStartTime time.Time
}

// Create a new instance of the ffmpeg-based recorder
func NewFFmpegRecorder(config *config.Config, logger logger.Logger, tempManager *processing.TempFileManager) *FFmpegRecorder {
	r := &FFmpegRecorder{
		BaseRecorder: NewBaseRecorder(config, logger, tempManager),
	}
	// ffmpeg finalizes the WAV trailer (correct header sizes) only on a clean
	// stop; reading "q" on stdin triggers that, whereas signals can leave it
	// SIGKILL'd with an unfinalized header that decoders read as empty.
	r.quitToken = []byte("q")
	return r
}

// Resolve PulseAudio source name if "default" is specified
func (f *FFmpegRecorder) resolvePulseAudioSource(device string) string {
	if device != "default" {
		return device
	}
	// Try to get actual source name using pactl
	cmd := exec.Command("pactl", "list", "short", "sources")
	output, err := cmd.Output()
	if err != nil {
		f.logger.Debug("Could not list PulseAudio sources: %v", err)
		return device
	}
	// Find first non-monitor input source (usually the microphone)
	// Priority: wired input > any non-monitor source
	var firstNonMonitor string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			sourceName := fields[1]
			// Skip monitor sources (output redirects)
			if strings.Contains(sourceName, ".monitor") {
				continue
			}
			// Skip bluetooth if there's a wired mic
			if strings.Contains(sourceName, "bluez") {
				continue
			}
			// Remember first non-monitor source as fallback
			if firstNonMonitor == "" {
				firstNonMonitor = sourceName
			}

			// Prefer sources with "input" in the name
			if strings.Contains(sourceName, "input") {
				f.logger.Info("Resolved 'default' to actual source: %s", sourceName)
				return sourceName
			}
		}
	}

	// Fallback: use first non-monitor source found
	if firstNonMonitor != "" {
		f.logger.Info("Resolved 'default' to first non-monitor source: %s", firstNonMonitor)
		return firstNonMonitor
	}

	f.logger.Debug("Could not resolve default source, using 'default'")
	return device
}

// Start an audio recording using the `ffmpeg` command
func (f *FFmpegRecorder) StartRecording() error {
	// Track recording start time for adaptive flush
	f.recordingStartTime = time.Now()

	// Build the command-line arguments for ffmpeg
	args := f.buildBaseCommandArgs()

	// Use the BaseRecorder to execute the command and manage the process
	if err := f.ExecuteRecordingCommand("ffmpeg", args); err != nil {
		return err
	}

	// Warm-up: wait until minimal payload is written to avoid clipped start
	// For WAV, header is 44 bytes. Ensure at least minAudioPayloadMs of payload is present.
	if !f.useBuffer {
		out := f.GetOutputFile()
		sr := f.config.Audio.SampleRate
		if sr <= 0 {
			sr = 16000
		}
		minPayload := int64((sr / (1000 / minAudioPayloadMs)) * 2) // 16-bit mono => 2 bytes per sample
		deadline := time.Now().Add(warmupTimeout)
		for time.Now().Before(deadline) {
			if info, err := os.Stat(out); err == nil && info.Size() >= 44+minPayload {
				break
			}
			time.Sleep(warmupPollInterval)
		}
	}
	return nil
}

// Stop recording with ffmpeg-specific adaptive flush for short recordings
func (f *FFmpegRecorder) StopRecording() (string, error) {
	// Always allow a small post-roll before stopping to avoid trimming
	time.Sleep(postRollDelay)

	// Stop the base recording process
	outputFile, err := f.BaseRecorder.StopRecording()
	if err == nil {
		return outputFile, nil // Success on first attempt
	}
	// Retry on empty/too-short files regardless of duration (ffmpeg flush)
	isEmptyFileError := strings.Contains(err.Error(), "audio file is empty") ||
		strings.Contains(err.Error(), "audio file too short")
	if !isEmptyFileError {
		return outputFile, err
	}
	// ffmpeg-specific retry: short recordings need extra time to flush
	f.logger.Warning("ffmpeg recording resulted in empty/short file, retrying with extra flush time...")
	time.Sleep(retryFlushDelay)
	// Re-check the file
	info, statErr := os.Stat(outputFile)
	if statErr == nil && info.Size() > 44 {
		f.logger.Info("Retry successful: file now has %d bytes", info.Size())
		return outputFile, nil
	}
	// Still failed after retry
	f.logger.Error("Retry failed, file still empty or too small")
	return outputFile, err
}

// Build the base command-line arguments for an ffmpeg recording process
func (f *FFmpegRecorder) buildBaseCommandArgs() []string {
	// Resolve actual PulseAudio source name (ffmpeg needs explicit source, not "default")
	device := f.resolvePulseAudioSource(f.config.Audio.Device)
	// Stable command with optimized options; stdin stays open so a "q" can
	// stop ffmpeg cleanly and let it finalize the WAV header.
	sr := f.config.Audio.SampleRate
	if sr <= 0 {
		sr = 16000
	}
	// Reduce PulseAudio buffering to lower start latency (~20ms fragments)
	fragmentBytes := (sr / 50) * 2 // bytes, 16-bit mono
	args := []string{
		"-hide_banner",
		"-y",                   // Overwrite output file if it exists
		"-fflags", "+nobuffer", // Reduce demuxer (input) buffering for low latency
		// Input options must precede the input specification
		"-analyzeduration", "0", // Start processing immediately, skip long stream analysis
		"-probesize", "32k", // Smaller probe size for faster start
		"-use_wallclock_as_timestamps", "1", // Ensure timestamps advance from wallclock
		"-wallclock", "1", // PulseAudio: set initial pts using current time
		"-fragment_size", fmt.Sprintf("%d", fragmentBytes), // PulseAudio: smaller fragments for lower latency
		"-rtbufsize", "32k", // Smaller realtime input buffer for low-latency capture
		"-thread_queue_size", "256",
		"-f", "pulse", // PulseAudio input
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", f.config.Audio.SampleRate),
		"-i", device,
		// Output options
		"-flags", "+low_delay",
		"-ac", "1", // Mono output
		"-ar", fmt.Sprintf("%d", f.config.Audio.SampleRate),
		"-vn", "-sn", // Disable video/subtitles just in case
		"-c:a", "pcm_s16le",
		"-q:a", "0",
		"-flush_packets", "1", // Flush output muxer to disk per packet (avoid data loss on stop)
	}

	// Configure the output format
	if f.useBuffer {
		// Output WAV to stdout for in-memory processing
		args = append(args, "-f", "wav", "-")
	} else {
		// Output to a WAV file. The path will be added by ExecuteRecordingCommand
		args = append(args, "-f", "wav")
	}

	return args
}
