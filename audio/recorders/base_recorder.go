// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package recorders

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/AshBuk/dabri/v2/audio/interfaces"
	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// Implements common functionality for audio recorders
type BaseRecorder struct {
	config             *config.Config
	cmd                *exec.Cmd
	outputFile         string
	ctx                context.Context
	cancel             context.CancelFunc
	mutex              sync.Mutex
	cmdTimeout         time.Duration
	buffer             *bytes.Buffer                 // In-memory buffer for short recordings
	useBuffer          bool                          // Flag to determine if recording to memory or file
	tempManager        *processing.TempFileManager   // Manager for temporary file lifecycle
	audioLevelCallback interfaces.AudioLevelCallback // Callback for audio level updates
	currentAudioLevel  float64                       // Current audio level (0.0 to 1.0)
	levelMutex         sync.RWMutex                  // Mutex for audio level access

	// Diagnostics
	stderrBuf bytes.Buffer

	// Process synchronization
	waitOnce   sync.Once
	processErr error

	logger logger.Logger
}

// Create a new base recorder instance
func NewBaseRecorder(config *config.Config, logger logger.Logger, tempManager *processing.TempFileManager) BaseRecorder {
	// Use an in-memory buffer for short recordings to avoid disk I/O
	useBuffer := config.Audio.ExpectedDuration > 0 &&
		config.Audio.ExpectedDuration < 10 &&
		config.Audio.SampleRate <= 16000

	// Determine the command timeout from the configuration
	maxTime := time.Duration(config.Audio.MaxRecordingTime) * time.Second
	if maxTime <= 0 {
		maxTime = 5 * time.Minute
	}

	return BaseRecorder{
		config:      config,
		cmdTimeout:  maxTime,
		useBuffer:   useBuffer,
		buffer:      bytes.NewBuffer(nil),
		tempManager: tempManager,
		logger:      logger,
	}
}

// Return the path to the recorded audio file
func (b *BaseRecorder) GetOutputFile() string {
	return b.outputFile
}

// Set the callback function for audio level monitoring
func (b *BaseRecorder) SetAudioLevelCallback(callback interfaces.AudioLevelCallback) {
	b.levelMutex.Lock()
	defer b.levelMutex.Unlock()
	b.audioLevelCallback = callback
}

// Return the current audio level (0.0 to 1.0)
func (b *BaseRecorder) GetAudioLevel() float64 {
	b.levelMutex.RLock()
	defer b.levelMutex.RUnlock()
	return b.currentAudioLevel
}

// Update the current audio level and invoke the callback if it is set
func (b *BaseRecorder) updateAudioLevel(level float64) {
	b.levelMutex.Lock()
	b.currentAudioLevel = level
	callback := b.audioLevelCallback
	b.levelMutex.Unlock()
	if callback != nil {
		callback(level)
	}
}

// Calculate the audio level (RMS) from a raw PCM data buffer
func (b *BaseRecorder) calculateAudioLevel(data []byte) float64 {
	if len(data) == 0 {
		return 0.0
	}
	// Assume 16-bit PCM data
	var sum int64
	samples := len(data) / 2

	for i := 0; i < len(data)-1; i += 2 {
		// Convert bytes to a 16-bit signed integer
		sample := int16(data[i]) | int16(data[i+1])<<8
		sum += int64(sample) * int64(sample)
	}

	if samples == 0 {
		return 0.0
	}
	// Calculate Root Mean Square (RMS) and normalize to a 0-1 range
	rms := float64(sum) / float64(samples)
	rms = rms / (32768.0 * 32768.0)

	// Amplify for better visibility
	if rms > 0 {
		return rms * 10.0
	}

	return 0.0
}

// Remove the temporary audio file or clear the in-memory buffer
func (b *BaseRecorder) CleanupFile() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.useBuffer {
		b.buffer.Reset()
		return nil
	}
	if b.outputFile == "" {
		return nil // Nothing to clean up
	}
	// Use the temp file manager for cleanup
	b.tempManager.RemoveFile(b.outputFile, true)
	return nil
}

// Create a temporary file for the audio recording or reset the buffer
func (b *BaseRecorder) createTempFile() error {
	if b.useBuffer {
		// Using in-memory buffer, no file needed
		b.buffer.Reset()
		return nil
	}

	path, err := b.tempManager.CreateTempWav(b.config.General.TempAudioPath)
	if err != nil {
		return fmt.Errorf("failed to create temp audio file: %w", err)
	}
	b.outputFile = path
	return nil
}

// Provide a common implementation for stopping the recording process
func (b *BaseRecorder) StopRecording() (string, error) {
	if err := b.StopProcess(); err != nil {
		return "", err
	}
	return b.outputFile, nil
}

// Read audio data from a reader and calculate audio levels
func (b *BaseRecorder) monitorAudioLevel(reader io.Reader) {
	buf := make([]byte, 4096) // Buffer for reading audio data

	for {
		select {
		case <-b.ctx.Done():
			return
		default:
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					b.logger.Error("Error reading audio data: %v", err)
				}
				return
			}
			if n > 0 {
				// Write to the in-memory buffer
				b.buffer.Write(buf[:n])
				// Calculate and update the audio level
				level := b.calculateAudioLevel(buf[:n])
				b.updateAudioLevel(level)
			}
		}
	}
}

// Wait for the command to finish, ensuring it's only called once
func (b *BaseRecorder) waitForProcess() error {
	if b.cmd == nil {
		return fmt.Errorf("no command to wait for")
	}
	b.waitOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				b.processErr = fmt.Errorf("panic in Wait(): %v", r)
			}
		}()
		b.processErr = b.cmd.Wait()
	})

	return b.processErr
}

// Verify that recording process started successfully and is still running
func (b *BaseRecorder) verifyProcessStarted(startTime time.Time) error {
	// Check if process is still running after 100ms
	time.Sleep(100 * time.Millisecond)
	if b.cmd.ProcessState != nil && b.cmd.ProcessState.Exited() {
		b.logger.Error("Process exited immediately after start! Exit code: %d", b.cmd.ProcessState.ExitCode())
		return fmt.Errorf("process exited immediately with code %d", b.cmd.ProcessState.ExitCode())
	}
	b.logger.Debug("Process is running (alive for %v)", time.Since(startTime))
	return nil
}

// Validate that the audio file was created and has valid content
func (b *BaseRecorder) validateAudioFile() error {
	// Allow buffers to flush to disk
	b.logger.Debug("Waiting for buffers to flush...")
	time.Sleep(500 * time.Millisecond)

	b.logger.Debug("Checking audio file: %s", b.outputFile)
	info, err := os.Stat(b.outputFile)
	if err != nil {
		if os.IsNotExist(err) {
			b.logger.Error("Audio file was not created: %s", b.outputFile)
			return fmt.Errorf("audio file was not created")
		}
		return fmt.Errorf("failed to stat audio file: %w", err)
	}

	b.logger.Debug("Audio file size: %d bytes", info.Size())

	// A minimal valid WAV header is 44 bytes
	if info.Size() <= 44 {
		b.logger.Error("Audio file empty (size=%d) - likely recording failed", info.Size())
		b.logger.Info("Check audio device availability and permissions")
		return fmt.Errorf("audio file is empty or invalid (size=%d)", info.Size())
	}

	// Additional safety: ensure there is at least 0.05s of audio payload to avoid downstream crashes
	// We force output to 16-bit PCM mono, so bytesPerSample = 2
	payloadBytes := info.Size() - 44
	sr := b.config.Audio.SampleRate
	if sr <= 0 {
		sr = 16000
	}
	minPayload := int64((sr / 20) * 2) // ~0.05s of audio
	if payloadBytes < minPayload {
		b.logger.Error("Audio file too short (payload=%d bytes, expected >= %d)", payloadBytes, minPayload)
		return fmt.Errorf("audio file too short (payload=%d < %d)", payloadBytes, minPayload)
	}

	b.logger.Debug("Audio file validation successful: %d bytes", info.Size())
	return nil
}

// Stop the recording process gracefully, with retries and final termination
func (b *BaseRecorder) StopProcess() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.cmd == nil || b.cmd.Process == nil {
		return fmt.Errorf("recording not started")
	}

	b.attemptGracefulShutdown()
	b.ensureProcessReaped()
	b.cleanup()

	if !b.useBuffer {
		return b.validateAudioFile()
	}
	return nil
}

func (b *BaseRecorder) attemptGracefulShutdown() {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			b.logger.Warning("Retry %d to stop recording process", i)
		}

		if b.sendSIGINTAndWait() {
			return
		}

		b.escalateToSIGKILL()
		time.Sleep(100 * time.Millisecond)
	}
}

func (b *BaseRecorder) sendSIGINTAndWait() bool {
	b.logger.Debug("Sending SIGINT to process PID=%d", b.cmd.Process.Pid)
	if err := b.cmd.Process.Signal(syscall.SIGINT); err != nil {
		b.logger.Warning("Failed to send SIGINT: %v", err)
	} else {
		b.logger.Debug("SIGINT sent successfully")
	}

	done := make(chan error, 1)
	go func() { done <- b.waitForProcess() }()
	select {
	case err := <-done:
		b.logProcessExit(err)
		return true
	case <-time.After(2000 * time.Millisecond):
		b.logger.Warning("Process did not exit within timeout")
		return false
	}
}

func (b *BaseRecorder) logProcessExit(err error) {
	if exitErr, ok := err.(*exec.ExitError); ok {
		b.logger.Debug("Process exited with code: %d", exitErr.ExitCode())
	} else if err != nil && err.Error() != "signal: killed" {
		b.logger.Warning("Process exited with error: %v", err)
	} else {
		b.logger.Debug("Process exited successfully")
	}
}

func (b *BaseRecorder) escalateToSIGKILL() {
	if err := b.cmd.Process.Signal(syscall.SIGKILL); err != nil {
		b.logger.Warning("Failed to send SIGKILL: %v", err)
	}
}

func (b *BaseRecorder) ensureProcessReaped() {
	if b.cmd == nil || b.cmd.Process == nil {
		return
	}

	finalDone := make(chan error, 1)
	go func() { finalDone <- b.waitForProcess() }()

	select {
	case <-finalDone:
		return
	case <-time.After(300 * time.Millisecond):
		_ = b.cmd.Process.Signal(syscall.SIGKILL)
		select {
		case <-finalDone:
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (b *BaseRecorder) cleanup() {
	if b.cancel != nil {
		b.cancel()
	}

	b.logStderr()

	b.cmd = nil
	b.cancel = nil
}

func (b *BaseRecorder) logStderr() {
	cmdName := "recorder"
	if b.cmd != nil && b.cmd.Path != "" {
		cmdName = b.cmd.Path
	}

	if b.stderrBuf.Len() > 0 {
		b.logger.Debug("%s stderr (len=%d): %s", cmdName, b.stderrBuf.Len(), b.stderrBuf.String())
	} else {
		b.logger.Debug("%s stderr: (empty)", cmdName)
	}
}

// Execute a recording command with context, timeout, and output handling.
// This is the primary method for concrete recorder implementations to use
func (b *BaseRecorder) ExecuteRecordingCommand(cmdName string, args []string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.cmd != nil && b.cmd.Process != nil {
		return fmt.Errorf("recording already in progress")
	}
	b.ctx, b.cancel = context.WithTimeout(context.Background(), b.cmdTimeout)
	// Set up the output file or in-memory buffer
	if b.useBuffer {
		b.buffer.Reset()
	} else {
		if err := b.createTempFile(); err != nil {
			return err
		}
	}
	// Add the output file to the command arguments if not using a buffer
	finalArgs := args
	if !b.useBuffer {
		finalArgs = append(args, b.outputFile)
	}
	// Security: validate the command and sanitize arguments before execution
	if !config.IsCommandAllowed(b.config, cmdName) {
		return fmt.Errorf("command not allowed: %s", cmdName)
	}
	safeArgs := config.SanitizeCommandArgs(finalArgs)
	// Create the command with context
	// #nosec G204 -- Safe: command name is from an allowlist and arguments are sanitized.
	b.cmd = exec.CommandContext(b.ctx, cmdName, safeArgs...)
	// Log full command at Debug level for troubleshooting
	b.logger.Debug("Executing command: %s %v", cmdName, safeArgs)
	// Capture stderr for diagnostics
	b.stderrBuf.Reset()
	b.cmd.Stderr = &b.stderrBuf
	// If using a buffer, set up a pipe to read stdout for audio level monitoring
	if b.useBuffer {
		stdout, err := b.cmd.StdoutPipe()
		if err != nil {
			b.cancel()
			return fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		go b.monitorAudioLevel(stdout)
	}
	// Reset wait state and start the process
	b.waitOnce = sync.Once{}
	b.processErr = nil
	if err := b.cmd.Start(); err != nil {
		b.cancel()
		return fmt.Errorf("failed to start recording: %w", err)
	}

	// Verify process started successfully
	startTime := time.Now()
	b.logger.Info("Recording process started: PID=%d, command=%s", b.cmd.Process.Pid, cmdName)
	return b.verifyProcessStarted(startTime)
}
