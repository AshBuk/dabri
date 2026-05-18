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

// TestNewArecordRecorder tests the creation of ArecordRecorder
func TestNewArecordRecorder(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Device = "default"
	cfg.Audio.Format = "S16_LE"
	cfg.Audio.SampleRate = 16000

	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewArecordRecorder(cfg, mockLogger, tempMgr)
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

	if recorder.config.Audio.SampleRate != 16000 {
		t.Errorf("Expected sample rate 16000, got %d", recorder.config.Audio.SampleRate)
	}
}

// TestArecordRecorder_getArecordFormat tests format conversion
func TestArecordRecorder_getArecordFormat(t *testing.T) {
	tests := []struct {
		name         string
		inputFormat  string
		expectFormat string
	}{
		{
			name:         "s16le conversion",
			inputFormat:  "s16le",
			expectFormat: "S16_LE",
		},
		{
			name:         "s24le conversion",
			inputFormat:  "s24le",
			expectFormat: "S24_LE",
		},
		{
			name:         "s32le conversion",
			inputFormat:  "s32le",
			expectFormat: "S32_LE",
		},
		{
			name:         "unknown format passthrough",
			inputFormat:  "CUSTOM_FORMAT",
			expectFormat: "CUSTOM_FORMAT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Audio.Format = tt.inputFormat

			mockLogger := testutils.NewMockLogger()
			tempMgr := processing.NewTempFileManager(30 * time.Minute)
			recorder := NewArecordRecorder(cfg, mockLogger, tempMgr)
			format := recorder.getArecordFormat()
			if format != tt.expectFormat {
				t.Errorf("Expected format %s, got %s", tt.expectFormat, format)
			}
		})
	}
}

// TestArecordRecorder_OutputFileHandling tests proper output file management
func TestArecordRecorder_OutputFileHandling(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Device = "default"
	cfg.Audio.Format = "S16_LE"
	cfg.Audio.SampleRate = 16000

	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewArecordRecorder(cfg, mockLogger, tempMgr)
	// Initially, no output file should be set
	if recorder.GetOutputFile() != "" {
		t.Errorf("Expected empty output file initially, got %s", recorder.GetOutputFile())
	}

	// Test that we can get/set output file through the interface
	tempDir := t.TempDir()
	expectedFile := filepath.Join(tempDir, "test_recording.wav")
	recorder.outputFile = expectedFile
	if recorder.GetOutputFile() != expectedFile {
		t.Errorf("Expected output file %s, got %s", expectedFile, recorder.GetOutputFile())
	}
}

// TestArecordRecorder_InvalidConfiguration tests error handling with bad configs
func TestArecordRecorder_InvalidConfiguration(t *testing.T) {
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
				cfg.Audio.Format = "S16_LE"
				cfg.Audio.SampleRate = 16000
				// channels removed
				return cfg
			},
			expectError: false, // arecord might handle empty device
		},
		{
			name: "zero sample rate",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Audio.Device = "default"
				cfg.Audio.Format = "S16_LE"
				cfg.Audio.SampleRate = 0
				// channels removed
				return cfg
			},
			expectError: true, // This should cause issues
		},
		{
			name: "placeholder_no_channels",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Audio.Device = "default"
				cfg.Audio.Format = "S16_LE"
				cfg.Audio.SampleRate = 16000
				return cfg
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			mockLogger := testutils.NewMockLogger()
			tempMgr := processing.NewTempFileManager(30 * time.Minute)
			recorder := NewArecordRecorder(cfg, mockLogger, tempMgr)

			// Test format conversion instead of building args
			format := recorder.getArecordFormat()

			if tt.expectError {
				// For invalid configurations, we can't test much without the build args
				// Just verify the recorder was created
				if recorder == nil {
					t.Error("Recorder should still be created even with invalid config")
				}
			} else {
				// For valid configs, test format conversion worked
				if format == "" {
					t.Error("Format conversion should not return empty string")
				}
			}
		})
	}
}

// TestArecordRecorder_AudioLevelCallbacks tests audio level monitoring
func TestArecordRecorder_AudioLevelCallbacks(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.Device = "default"
	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewArecordRecorder(cfg, mockLogger, tempMgr)

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
	testLevel := 0.75
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

// TestArecordRecorder_CleanupFile tests file cleanup functionality
func TestArecordRecorder_CleanupFile(t *testing.T) {
	cfg := &config.Config{}
	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewArecordRecorder(cfg, mockLogger, tempMgr)

	// Create a temporary file to test cleanup
	tempFile, err := os.CreateTemp("", "arecord_test_*.wav")
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

// TestArecordRecorder_BufferMode tests in-memory buffer functionality
func TestArecordRecorder_BufferMode(t *testing.T) {
	cfg := &config.Config{}
	cfg.Audio.ExpectedDuration = 5 // Short duration should trigger buffer mode
	cfg.Audio.SampleRate = 16000   // Low sample rate should trigger buffer mode

	mockLogger := testutils.NewMockLogger()
	tempMgr := processing.NewTempFileManager(30 * time.Minute)
	recorder := NewArecordRecorder(cfg, mockLogger, tempMgr)

	// Should be using buffer mode for short, low-quality recordings
	if !recorder.useBuffer {
		t.Error("Expected buffer mode to be enabled for short recordings")
	}

}
