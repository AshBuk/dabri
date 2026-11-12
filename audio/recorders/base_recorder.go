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

// Time allowed for a process to finalize its output after a graceful stop
// request (e.g. ffmpeg writing the WAV trailer) before escalating to signals.
const gracefulQuitTimeout = 1500 * time.Millisecond

// BaseRecorder implements common functionality for audio recorders
type BaseRecorder struct {
	config      *config.Config
	cmd         *exec.Cmd
	outputFile  string
	ctx         context.Context
	cancel      context.CancelFunc
	mutex       sync.Mutex
	cmdTimeout  time.Duration
	tempManager *processing.TempFileManager // Manager for temporary file lifecycle

	// Graceful stop: when quitToken is set, StopProcess first writes it to the
	// process stdin (e.g. ffmpeg "q") so the process finalizes its own output
	// before any signal escalation. Nil token keeps the signal-only behavior.
	quitToken []byte
	stdinPipe io.WriteCloser

	// Diagnostics
	stderrBuf bytes.Buffer

	// Process synchronization
	waitOnce   sync.Once
	processErr error

	logger logger.Logger
}

// NewBaseRecorder creates a new base recorder instance
func NewBaseRecorder(config *config.Config, logger logger.Logger, tempManager *processing.TempFileManager) BaseRecorder {
	// Determine the command timeout from the configuration
	maxTime := time.Duration(config.Audio.MaxRecordingTime) * time.Second
	if maxTime <= 0 {
		maxTime = 5 * time.Minute
	}

	return BaseRecorder{
		config:      config,
		cmdTimeout:  maxTime,
		tempManager: tempManager,
		logger:      logger,
	}
}

// GetOutputFile returns the path to the recorded audio file
func (b *BaseRecorder) GetOutputFile() string {
	return b.outputFile
}

// CleanupFile removes the temporary audio file
func (b *BaseRecorder) CleanupFile() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.outputFile == "" {
		return nil // Nothing to clean up
	}
	// Use the temp file manager for cleanup
	b.tempManager.RemoveFile(b.outputFile, true)
	return nil
}

// Create a temporary file for the audio recording
func (b *BaseRecorder) createTempFile() error {
	path, err := b.tempManager.CreateTempWav(b.config.General.TempAudioPath)
	if err != nil {
		return fmt.Errorf("failed to create temp audio file: %w", err)
	}
	b.outputFile = path
	return nil
}

// StopRecording provides a common implementation for stopping the recording process
func (b *BaseRecorder) StopRecording() (string, error) {
	if err := b.StopProcess(); err != nil {
		return "", err
	}
	return b.outputFile, nil
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
	// The process has been reaped, so its output file is complete
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

	// Ensure there is at least 0.3s of audio payload after the 44-byte WAV header.
	// Shorter clips are taps, not speech: Whisper hallucinates text on them.
	// We force output to 16-bit PCM mono, so bytesPerSample = 2
	payloadBytes := info.Size() - 44
	sr := b.config.Audio.SampleRate
	if sr <= 0 {
		sr = 16000
	}
	minPayload := int64(sr * 3 / 10 * 2) // 0.3s of audio
	if payloadBytes < minPayload {
		b.logger.Debug("Audio file too short (payload=%d bytes, expected >= %d)", payloadBytes, minPayload)
		return fmt.Errorf("%w (payload=%d < %d)", interfaces.ErrRecordingTooShort, payloadBytes, minPayload)
	}

	b.logger.Debug("Audio file validation successful: %d bytes", info.Size())
	return nil
}

// StopProcess stops the recording process gracefully, with retries and final termination
func (b *BaseRecorder) StopProcess() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.cmd == nil || b.cmd.Process == nil {
		return fmt.Errorf("recording not started")
	}

	b.attemptGracefulShutdown()
	b.ensureProcessReaped()
	b.cleanup()
	return b.validateAudioFile()
}

func (b *BaseRecorder) attemptGracefulShutdown() {
	// Prefer process-native graceful stop (e.g. ffmpeg "q" writes the file
	// trailer) so output is finalized without a lossy SIGKILL.
	if b.tryGracefulQuit() {
		return
	}

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

// tryGracefulQuit asks the process to stop via its stdin token (e.g. ffmpeg
// "q"), letting it finalize its output. Returns true if it exited in time.
func (b *BaseRecorder) tryGracefulQuit() bool {
	if len(b.quitToken) == 0 || b.stdinPipe == nil {
		return false
	}
	if _, err := b.stdinPipe.Write(b.quitToken); err != nil {
		b.logger.Debug("Graceful quit write failed: %v", err)
		return false
	}
	// EOF on stdin also tells most tools to stop.
	_ = b.stdinPipe.Close()

	done := make(chan error, 1)
	go func() { done <- b.waitForProcess() }()
	select {
	case err := <-done:
		b.logProcessExit(err)
		return true
	case <-time.After(gracefulQuitTimeout):
		b.logger.Warning("Graceful quit timed out, escalating to signals")
		return false
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

	finalDone := make(chan struct{})
	go func() { _ = b.waitForProcess(); close(finalDone) }()

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
	if b.stdinPipe != nil {
		_ = b.stdinPipe.Close()
		b.stdinPipe = nil
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

// ExecuteRecordingCommand executes a recording command with context, timeout, and output handling.
// This is the primary method for concrete recorder implementations to use
func (b *BaseRecorder) ExecuteRecordingCommand(cmdName string, args []string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.cmd != nil && b.cmd.Process != nil {
		return fmt.Errorf("recording already in progress")
	}
	b.ctx, b.cancel = context.WithTimeout(context.Background(), b.cmdTimeout)
	// Set up the output file and append it to the command arguments
	if err := b.createTempFile(); err != nil {
		return err
	}
	finalArgs := append(args, b.outputFile)
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
	// Wire a controlled stdin pipe for graceful shutdown when requested
	if len(b.quitToken) > 0 {
		stdin, err := b.cmd.StdinPipe()
		if err != nil {
			b.cancel()
			return fmt.Errorf("failed to create stdin pipe: %w", err)
		}
		b.stdinPipe = stdin
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
