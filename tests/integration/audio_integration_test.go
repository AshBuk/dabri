//go:build integration

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AshBuk/dabri/v2/audio/factory"
	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

func TestAudioRecordingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test real audio recording functionality
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	tempDir := t.TempDir()
	cfg.General.TempAudioPath = tempDir
	t.Run("arecord_real_device", func(t *testing.T) {
		// Test with real audio device if available
		cfg.Audio.RecordingMethod = "arecord"
		cfg.Audio.Device = "default"
		cfg.Audio.SampleRate = 16000
		// channels removed (mono enforced internally)

		testLogger := logger.NewDefaultLogger(logger.InfoLevel)
		tempManager := processing.NewTempFileManager(30 * time.Minute)
		recorder, err := factory.GetRecorder(cfg, testLogger, tempManager)
		if err != nil {
			t.Skipf("Audio recorder not available: %v", err)
		}

		err = recorder.StartRecording()
		if err != nil {
			t.Skipf("Could not start recording (no audio device?): %v", err)
		}

		// Record for a short time
		time.Sleep(100 * time.Millisecond)
		// Ensure cleanup happens even if test fails
		defer func() {
			recorder.StopRecording()
			recorder.CleanupFile()
		}()

		outputFile, err := recorder.StopRecording()
		if err != nil {
			t.Skipf("Failed to stop recording (audio device issue): %v", err)
		}
		// Check that file was created
		if outputFile != "" {
			if _, err := os.Stat(outputFile); err != nil {
				t.Errorf("Recording file not created: %v", err)
			}
		}
	})

	t.Run("ffmpeg_real_device", func(t *testing.T) {
		// Test with FFmpeg if available
		cfg.Audio.RecordingMethod = "ffmpeg"
		cfg.Audio.Device = "default"

		testLogger := logger.NewDefaultLogger(logger.InfoLevel)
		tempManager := processing.NewTempFileManager(30 * time.Minute)
		recorder, err := factory.GetRecorder(cfg, testLogger, tempManager)
		if err != nil {
			t.Skipf("FFmpeg recorder not available: %v", err)
		}

		err = recorder.StartRecording()
		if err != nil {
			t.Skipf("Could not start FFmpeg recording: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		// Ensure cleanup happens even if test fails
		defer func() {
			recorder.StopRecording()
			recorder.CleanupFile()
		}()

		outputFile, err := recorder.StopRecording()
		if err != nil {
			t.Skipf("Failed to stop FFmpeg recording (audio device issue): %v", err)
		}

		if outputFile != "" {
			if _, err := os.Stat(outputFile); err != nil {
				t.Errorf("FFmpeg recording file not created: %v", err)
			}
		}
	})
}

func TestAudioDeviceDetection(t *testing.T) {
	// Test that audio devices can be detected and configured
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	t.Run("default_device_available", func(t *testing.T) {
		cfg.Audio.Device = "default"
		cfg.Audio.RecordingMethod = "arecord"

		testLogger := logger.NewDefaultLogger(logger.InfoLevel)
		tempManager := processing.NewTempFileManager(30 * time.Minute)
		recorder, err := factory.GetRecorder(cfg, testLogger, tempManager)
		if err != nil {
			t.Skipf("Default audio device not available: %v", err)
		}

		// Try to initialize recording (but don't actually record)
		err = recorder.StartRecording()
		if err != nil {
			t.Logf("Default device not functional: %v", err)
		} else {
			// Ensure cleanup happens even if test fails
			defer func() {
				recorder.StopRecording()
				recorder.CleanupFile()
			}()
			recorder.StopRecording()
			t.Log("Default audio device is functional")
		}
	})

	t.Run("multiple_recording_methods", func(t *testing.T) {
		methods := []string{"arecord", "ffmpeg"}
		workingMethods := 0

		for _, method := range methods {
			cfg.Audio.RecordingMethod = method
			testLogger := logger.NewDefaultLogger(logger.InfoLevel)
			tempManager := processing.NewTempFileManager(30 * time.Minute)
			_, err := factory.GetRecorder(cfg, testLogger, tempManager)
			if err == nil {
				workingMethods++
				t.Logf("Recording method %s is available", method)
			} else {
				t.Logf("Recording method %s not available: %v", method, err)
			}
		}

		if workingMethods == 0 {
			t.Skip("No audio recording methods available")
		}

		t.Logf("Found %d working recording methods", workingMethods)
	})
}

func TestTemporaryFileManagement(t *testing.T) {
	// Test that temporary audio files are properly managed
	tempDir := t.TempDir()

	manager := processing.NewTempFileManager(30 * time.Minute)

	// Add several test files
	testFiles := []string{
		filepath.Join(tempDir, "audio1.wav"),
		filepath.Join(tempDir, "audio2.wav"),
		filepath.Join(tempDir, "audio3.wav"),
	}

	// Create test files
	for _, file := range testFiles {
		f, err := os.Create(file)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		f.Close()

		manager.AddFile(file)
	}

	// Verify files are tracked
	for _, file := range testFiles {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("Test file not found: %v", err)
		}
	}

	// Test immediate cleanup with CleanupAll
	manager.CleanupAll()

	// Verify files are cleaned up immediately
	for _, file := range testFiles {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("Test file should be cleaned up: %s", file)
		}
	}

	// Test background cleanup
	manager.Stop()

	// Give cleanup time to run
	time.Sleep(100 * time.Millisecond)

	t.Log("Temporary file management integration test completed")
}
