// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package recorders

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/testutils"
)

// TestNewFFmpegRecorder tests the creation of FFmpegRecorder
func TestNewFFmpegRecorder(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Device = "default"
	cfg.Audio.Format = "S16_LE"
	cfg.Audio.SampleRate = 16000

	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewFFmpegRecorder(cfg, mockLogger, tempMgr)
	if recorder == nil {
		t.Fatal("Expected recorder to be created, got nil")
	}

	if recorder.config != cfg {
		t.Error("Expected config to be set correctly")
	}
	// Test that BaseRecorder is properly initialized
	if recorder.config.Audio.Device != "default" {
		t.Errorf("Expected device 'default', got %s", recorder.config.Audio.Device)
	}
}

// TestFFmpegRecorder_buildBaseCommandArgs tests command argument building with real scenarios
func TestFFmpegRecorder_buildBaseCommandArgs(t *testing.T) {
	tests := []struct {
		name         string
		device       string
		sampleRate   int
		useBuffer    bool
		expectDevice string
		expectRate   string
	}{
		{
			name:         "basic file output",
			device:       "hw:0,0",
			sampleRate:   44100,
			useBuffer:    false,
			expectDevice: "hw:0,0",
			expectRate:   "44100",
		},
		{
			name:         "buffer mode should output to stdout",
			device:       "plughw:1,0",
			sampleRate:   48000,
			useBuffer:    true,
			expectDevice: "plughw:1,0",
			expectRate:   "48000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Audio.Device = tt.device
			cfg.Audio.SampleRate = tt.sampleRate

			mockLogger := testutils.NewMockLogger()
			tempMgr := processing.NewTempFileManager(30 * time.Minute)
			recorder := NewFFmpegRecorder(cfg, mockLogger, tempMgr)
			recorder.useBuffer = tt.useBuffer
			recorder.outputFile = "/tmp/test.wav"
			args := recorder.buildBaseCommandArgs()
			verifyArgFlag(t, args, "-i", tt.expectDevice, "device")
			verifyArgFlag(t, args, "-ar", tt.expectRate, "sample rate")
			verifyArgFlag(t, args, "-q:a", "0", "quality setting")
			verifyOutputMode(t, args, tt.useBuffer)
		})
	}
}

// TestFFmpegRecorder_OutputFileHandling tests proper output file management
func TestFFmpegRecorder_OutputFileHandling(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Device = "default"
	cfg.Audio.SampleRate = 16000

	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewFFmpegRecorder(cfg, mockLogger, tempMgr)
	// Initially, no output file should be set
	if recorder.GetOutputFile() != "" {
		t.Errorf("Expected empty output file initially, got %s", recorder.GetOutputFile())
	}

	// Test that we can get/set output file through the interface
	tempDir := t.TempDir()
	expectedFile := filepath.Join(tempDir, "test_ffmpeg_recording.wav")
	recorder.outputFile = expectedFile
	if recorder.GetOutputFile() != expectedFile {
		t.Errorf("Expected output file %s, got %s", expectedFile, recorder.GetOutputFile())
	}
}

// TestFFmpegRecorder_StopRecording tests the stop recording functionality
func TestFFmpegRecorder_StopRecording(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Device = "default"
	cfg.Audio.SampleRate = 16000

	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewFFmpegRecorder(cfg, mockLogger, tempMgr)
	// Set up output file before testing stop
	tempDir := t.TempDir()
	expectedFile := filepath.Join(tempDir, "test_ffmpeg_output.wav")
	recorder.outputFile = expectedFile
	// Test stopping without starting (should not panic)
	outputFile, err := recorder.StopRecording()
	// When no recording process is started, StopProcess() returns error
	// and the function returns empty string
	if err != nil {
		// This is expected - no recording was started
		t.Logf("StopRecording returned error as expected: %v", err)
		if outputFile != "" {
			t.Errorf("Expected empty output file on error, got %s", outputFile)
		}
	} else {
		// If no error, should return the file path
		if outputFile != expectedFile {
			t.Errorf("Expected output file %s, got %s", expectedFile, outputFile)
		}
	}
}

// TestFFmpegRecorder_InvalidConfiguration tests error handling with bad configs
func TestFFmpegRecorder_InvalidConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() *config.Config
		expectError bool
	}{
		{
			name: "empty device name",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Audio.Device = ""
				cfg.Audio.SampleRate = 16000
				return cfg
			},
			expectError: false, // ffmpeg might handle empty device
		},
		{
			name: "zero sample rate",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Audio.Device = "default"
				cfg.Audio.SampleRate = 0
				return cfg
			},
			expectError: true, // This should cause issues
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			mockLogger := testutils.NewMockLogger()
			tempMgr := processing.NewTempFileManager(30 * time.Minute)
			recorder := NewFFmpegRecorder(cfg, mockLogger, tempMgr)

			// Build args to see if they're reasonable
			args := recorder.buildBaseCommandArgs()

			if tt.expectError {
				// Check for obviously problematic arguments
				for i, arg := range args {
					if arg == "-ar" && i+1 < len(args) && args[i+1] == "0" {
						t.Log("Correctly detected zero sample rate in args")
						return
					}
				}
				t.Error("Expected to detect invalid configuration in command args")
			}
		})
	}
}

// TestFFmpegRecorder_AudioLevelCallbacks tests audio level monitoring
func TestFFmpegRecorder_AudioLevelCallbacks(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Device = "default"
	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewFFmpegRecorder(cfg, mockLogger, tempMgr)

	// Test initial audio level
	if recorder.GetAudioLevel() != 0.0 {
		t.Errorf("Expected initial audio level 0.0, got %f", recorder.GetAudioLevel())
	}

	// Test callback setting
	var receivedLevel float64
	var callbackCalled bool

	recorder.SetAudioLevelCallback(func(level float64) {
		receivedLevel = level
		callbackCalled = true
	})

	// Test audio level update
	testLevel := 0.85
	recorder.updateAudioLevel(testLevel)

	// Give callback time to execute
	time.Sleep(1 * time.Millisecond)

	if !callbackCalled {
		t.Error("Expected audio level callback to be called")
	}

	if receivedLevel != testLevel {
		t.Errorf("Expected callback to receive level %f, got %f", testLevel, receivedLevel)
	}

	if recorder.GetAudioLevel() != testLevel {
		t.Errorf("Expected stored audio level %f, got %f", testLevel, recorder.GetAudioLevel())
	}
}

// TestFFmpegRecorder_CleanupFile tests file cleanup functionality
func TestFFmpegRecorder_CleanupFile(t *testing.T) {
	cfg := &config.Config{}
	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewFFmpegRecorder(cfg, mockLogger, tempMgr)

	// Create a temporary file to test cleanup
	tempFile, err := os.CreateTemp("", "ffmpeg_test_*.wav")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()

	fileName := tempFile.Name()
	recorder.outputFile = fileName

	// Verify file exists
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		t.Fatal("Temp file should exist before cleanup")
	}

	// Test cleanup
	err = recorder.CleanupFile()
	if err != nil {
		t.Errorf("CleanupFile returned error: %v", err)
	}

	// File should be removed
	if _, err := os.Stat(fileName); !os.IsNotExist(err) {
		t.Error("File should be removed after cleanup")
		os.Remove(fileName) // Clean up in case test failed
	}
}

// TestFFmpegRecorder_BufferMode tests in-memory buffer functionality
func TestFFmpegRecorder_BufferMode(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.ExpectedDuration = 5 // Short duration should trigger buffer mode
	cfg.Audio.SampleRate = 16000   // Low sample rate should trigger buffer mode

	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewFFmpegRecorder(cfg, mockLogger, tempMgr)

	// Should be using buffer mode for short, low-quality recordings
	if !recorder.useBuffer {
		t.Error("Expected buffer mode to be enabled for short recordings")
	}
}

// verifyArgFlag checks if a command-line flag and its value are present in args
func verifyArgFlag(t *testing.T, args []string, flag, expectedValue, description string) {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) && args[i+1] == expectedValue {
			return
		}
	}
	t.Errorf("Expected %s %s not found in args: %v", description, expectedValue, args)
}

// verifyOutputMode validates FFmpeg output configuration based on buffer/file mode
func verifyOutputMode(t *testing.T, args []string, useBuffer bool) {
	hasWavMode := false
	hasStdout := false

	for i, arg := range args {
		if arg == "-f" && i+1 < len(args) && args[i+1] == "wav" {
			hasWavMode = true
		}
		if arg == "-" {
			hasStdout = true
		}
	}

	if useBuffer {
		if !hasWavMode || !hasStdout {
			t.Error("Expected wav stdout mode (-f wav -) in buffer mode")
		}
	} else {
		if hasStdout {
			t.Error("Did not expect stdout mode in file output mode")
		}
		if !hasWavMode {
			t.Error("Expected wav file output mode (-f wav)")
		}
	}
}
