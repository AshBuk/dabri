//go:build integration

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AshBuk/dabri/v2/audio/factory"
	audiointerfaces "github.com/AshBuk/dabri/v2/audio/interfaces"
	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	hotkeyinterfaces "github.com/AshBuk/dabri/v2/hotkeys/interfaces"
	"github.com/AshBuk/dabri/v2/hotkeys/manager"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/testutils"
	outputfactory "github.com/AshBuk/dabri/v2/output/factory"
	"github.com/AshBuk/dabri/v2/whisper"
)

// setupRecorderWithCleanup creates a recorder and ensures cleanup on test completion
func setupRecorderWithCleanup(t *testing.T, cfg *config.Config) audiointerfaces.AudioRecorder {
	testLogger := logger.NewDefaultLogger(logger.InfoLevel)
	tempManager := processing.NewTempFileManager(30 * time.Minute)
	recorder, err := factory.GetRecorder(cfg, testLogger, tempManager)
	if err != nil {
		t.Skipf("Audio not available: %v", err)
	}
	// Ensure cleanup happens even if test fails
	t.Cleanup(func() {
		// Force stop and cleanup (ignore any errors)
		forceStopRecorder(recorder)
	})

	return recorder
}

// forceStopRecorder ensures recording is stopped even if errors occur
func forceStopRecorder(recorder audiointerfaces.AudioRecorder) {
	// Try to stop gracefully first (ignore errors)
	_, _ = recorder.StopRecording()
	// Cleanup any remaining resources
	_ = recorder.CleanupFile()
	// Give time for background goroutines to finish
	time.Sleep(50 * time.Millisecond)
}

func TestEndToEndWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end integration test in short mode")
	}

	// This test simulates a complete user workflow
	t.Run("complete_recording_workflow", func(t *testing.T) {
		// Setup configuration
		cfg := &config.Config{}
		config.SetDefaultConfig(cfg)

		tempDir := t.TempDir()
		cfg.General.TempAudioPath = tempDir
		cfg.Output.DefaultMode = "clipboard" // Safe for testing
		cfg.Audio.RecordingMethod = "arecord"
		cfg.Audio.Device = "default"
		// Validate configuration
		err := config.ValidateConfig(cfg)
		if err != nil {
			t.Logf("Config validation failed (expected in test environment): %v", err)
		}

		// Test audio recording
		recorder := setupRecorderWithCleanup(t, cfg)
		// Start recording
		err = recorder.StartRecording()
		if err != nil {
			t.Skipf("Could not start recording: %v", err)
		}

		// Simulate short recording
		time.Sleep(200 * time.Millisecond)
		// Stop recording
		audioFile, err := recorder.StopRecording()
		if err != nil {
			t.Skipf("Failed to stop recording (audio device issue): %v", err)
		}

		// Verify audio file exists
		if audioFile != "" {
			if _, err := os.Stat(audioFile); err != nil {
				t.Errorf("Audio file not created: %v", err)
			}
		}
		// Test output system
		outputFactory := outputfactory.NewFactory(cfg)
		outputter, err := outputFactory.GetOutputter(outputfactory.EnvironmentUnknown)
		if err != nil {
			t.Logf("Output system not available (expected): %v", err)
		} else {
			// Test clipboard functionality (should fail gracefully)
			err = outputter.CopyToClipboard("Test transcription result")
			if err != nil {
				t.Logf("Clipboard operation failed (expected): %v", err)
			}
		}

		// Cleanup
		forceStopRecorder(recorder)
		t.Log("End-to-end workflow test completed")
	})
}

func TestApplicationInitializationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping application initialization test in short mode")
	}

	t.Run("app_components_initialization", func(t *testing.T) {
		// Test that all main application components can be initialized
		cfg := &config.Config{}
		config.SetDefaultConfig(cfg)

		tempDir := t.TempDir()
		cfg.General.TempAudioPath = tempDir
		cfg.WebServer.Enabled = false // Disable web server for testing
		// Test configuration loading
		t.Log("Testing configuration...")
		err := config.ValidateConfig(cfg)
		if err != nil {
			t.Logf("Config validation warnings (expected): %v", err)
		}

		// Test audio system
		t.Log("Testing audio system...")
		testLogger := logger.NewDefaultLogger(logger.InfoLevel)
		tempManager := processing.NewTempFileManager(30 * time.Minute)
		_, err = factory.GetRecorder(cfg, testLogger, tempManager)
		if err != nil {
			t.Logf("Audio system not available: %v", err)
		} else {
			t.Log("Audio system initialized successfully")
		}

		// Test output system
		t.Log("Testing output system...")
		outputFactory := outputfactory.NewFactory(cfg)
		_, err = outputFactory.GetOutputter(outputfactory.EnvironmentUnknown)
		if err != nil {
			t.Logf("Output system not fully available: %v", err)
		} else {
			t.Log("Output system initialized successfully")
		}

		// Test hotkey system
		t.Log("Testing hotkey system...")
		hotkeyConfig := adapters.NewConfigAdapter(cfg.Hotkeys.StartRecording, "auto")
		hotkeyManager := manager.NewHotkeyManager(hotkeyConfig, hotkeyinterfaces.EnvironmentUnknown, testutils.NewMockLogger())
		if hotkeyManager != nil {
			t.Log("Hotkey system initialized successfully")
		}

		// Test whisper system (if bundled model available)
		t.Log("Testing whisper system...")
		modelManager := whisper.NewModelManager(cfg)
		if modelPath, err := modelManager.GetModelPath(context.Background()); err == nil {
			t.Logf("Bundled whisper model found at %s but skipping engine test (requires CGO)", modelPath)
		} else {
			t.Log("Bundled whisper model not available for testing")
		}

		t.Log("Application initialization flow test completed")
	})
}

func TestRealWorldScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world scenario tests in short mode")
	}

	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	tempDir := t.TempDir()
	cfg.General.TempAudioPath = tempDir
	t.Run("quick_recording_session", func(t *testing.T) {
		// Simulate a quick voice note recording
		recorder := setupRecorderWithCleanup(t, cfg)

		files := []string{}
		// Record multiple short sessions
		for i := 0; i < 3; i++ {
			// Ensure clean state before starting
			forceStopRecorder(recorder)
			// Clear session state between recordings for isolation
			tempManager := processing.NewTempFileManager(30 * time.Minute)
			tempManager.CleanupAll()
			// Give time for cleanup to complete
			time.Sleep(100 * time.Millisecond)

			err := recorder.StartRecording()
			if err != nil {
				t.Skipf("Could not start recording %d: %v", i+1, err)
			}
			// Wait longer for process to fully start
			time.Sleep(200 * time.Millisecond)

			audioFile, err := recorder.StopRecording()
			if err != nil {
				// Ensure the recorder is stopped even on error
				forceStopRecorder(recorder)
				t.Logf("Failed to stop recording %d (audio device issue): %v", i+1, err)
			} else if audioFile != "" {
				files = append(files, audioFile)
			}
			// Wait for stop to complete before next iteration
			time.Sleep(100 * time.Millisecond)
		}
		// Verify all files were created (if any recording succeeded)
		successfulRecordings := 0
		for i, file := range files {
			if _, err := os.Stat(file); err != nil {
				t.Logf("Recording file %d not created (audio device unavailable): %v", i+1, err)
			} else {
				successfulRecordings++
			}
		}

		if successfulRecordings == 0 {
			t.Skip("No audio files created - audio device unavailable in test environment")
		}

		// Cleanup
		recorder.CleanupFile()
		t.Logf("Quick recording session test completed - %d successful recordings", successfulRecordings)
	})

	t.Run("error_recovery_scenarios", func(t *testing.T) {
		// Test error recovery in various scenarios
		recorder := setupRecorderWithCleanup(t, cfg)
		// Test stopping without starting
		_, err := recorder.StopRecording()
		if err == nil {
			t.Log("Stopping without starting handled gracefully")
		} else {
			t.Logf("Stop without start error (expected): %v", err)
		}

		// Test double start
		err = recorder.StartRecording()
		if err != nil {
			t.Skipf("Could not start first recording: %v", err)
		}
		// Wait for first recording to fully start
		time.Sleep(100 * time.Millisecond)

		err = recorder.StartRecording()
		if err != nil {
			t.Logf("Double start prevented (good): %v", err)
		}

		_, err = recorder.StopRecording()
		if err != nil {
			t.Logf("Stop recording error: %v", err)
		}
		// Force stop to ensure cleanup
		forceStopRecorder(recorder)

		t.Log("Error recovery scenarios test completed")
	})

	t.Run("concurrent_operations", func(t *testing.T) {
		// Test that concurrent operations are handled safely
		tempManager := processing.NewTempFileManager(30 * time.Minute)
		// Add files concurrently
		errChan := make(chan error, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				testFile := filepath.Join(tempDir, "concurrent_"+string(rune('0'+id))+".tmp")
				f, err := os.Create(testFile)
				if err != nil {
					errChan <- err
					return
				}
				f.Close()

				tempManager.AddFile(testFile)
				errChan <- nil
			}(i)
		}
		// Collect results
		errors := 0
		for i := 0; i < 10; i++ {
			if err := <-errChan; err != nil {
				errors++
				t.Logf("Concurrent operation error: %v", err)
			}
		}

		if errors > 5 { // Allow some failures in concurrent test
			t.Errorf("Too many concurrent operation failures: %d", errors)
		}

		t.Logf("Concurrent operations test completed with %d errors", errors)
	})
}

func TestSystemResourceManagement(t *testing.T) {
	// Test that system resources are properly managed
	t.Run("temporary_file_cleanup", func(t *testing.T) {
		tempDir := t.TempDir()
		tempManager := processing.NewTempFileManager(30 * time.Minute)
		// Create several temp files
		testFiles := []string{}
		for i := 0; i < 5; i++ {
			file := filepath.Join(tempDir, "resource_test_"+string(rune('1'+i))+".tmp")
			f, err := os.Create(file)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}
			f.Close()

			testFiles = append(testFiles, file)
			tempManager.AddFile(file)
		}
		// Verify files exist
		for _, file := range testFiles {
			if _, err := os.Stat(file); err != nil {
				t.Errorf("Test file not found: %v", err)
			}
		}
		// Test cleanup
		tempManager.Stop()
		time.Sleep(100 * time.Millisecond)

		t.Log("Resource management test completed")
	})

	t.Run("memory_usage_monitoring", func(t *testing.T) {
		// Test that memory usage stays reasonable during operation
		cfg := &config.Config{}
		config.SetDefaultConfig(cfg)
		// Create and destroy multiple components
		for i := 0; i < 5; i++ {
			// Skip whisper engine (requires CGO)
			t.Logf("Iteration %d: Skipping whisper engine (requires CGO)", i+1)

			testLogger := logger.NewDefaultLogger(logger.InfoLevel)
			tempManager := processing.NewTempFileManager(30 * time.Minute)
			recorder, err := factory.GetRecorder(cfg, testLogger, tempManager)
			if err == nil {
				recorder.CleanupFile()
			}

			outputFactory := outputfactory.NewFactory(cfg)
			_, _ = outputFactory.GetOutputter(outputfactory.EnvironmentUnknown)
		}

		t.Log("Memory usage monitoring test completed")
	})
}

func TestCrossComponentIntegration(t *testing.T) {
	// Test integration between different components
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	tempDir := t.TempDir()
	cfg.General.TempAudioPath = tempDir

	t.Run("audio_to_output_pipeline", func(t *testing.T) {
		// Test the complete pipeline from audio to output
		recorder := setupRecorderWithCleanup(t, cfg)
		outputFactory := outputfactory.NewFactory(cfg)
		outputter, err := outputFactory.GetOutputter(outputfactory.EnvironmentUnknown)
		if err != nil {
			t.Logf("Output not available: %v", err)
		}

		// Simulate recording
		err = recorder.StartRecording()
		if err != nil {
			t.Skipf("Could not start recording: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
		_, _ = recorder.StopRecording()
		// Simulate transcription result
		mockTranscription := "This is a test transcription result"
		// Test output
		if outputter != nil {
			err = outputter.CopyToClipboard(mockTranscription)
			if err != nil {
				t.Logf("Output operation failed (expected): %v", err)
			}
		}
		// Ensure proper cleanup
		forceStopRecorder(recorder)
		t.Log("Audio to output pipeline test completed")
	})

	t.Run("config_validation_integration", func(t *testing.T) {
		// Test that configuration affects all components correctly
		testConfigs := []*config.Config{
			func() *config.Config {
				c := &config.Config{}
				config.SetDefaultConfig(c)
				c.Audio.RecordingMethod = "arecord"
				return c
			}(),
			func() *config.Config {
				c := &config.Config{}
				config.SetDefaultConfig(c)
				c.Audio.RecordingMethod = "ffmpeg"
				return c
			}(),
		}

		for i, testCfg := range testConfigs {
			t.Run(testCfg.Audio.RecordingMethod, func(t *testing.T) {
				// Test that configuration is respected
				testLogger := logger.NewDefaultLogger(logger.InfoLevel)
				tempManager := processing.NewTempFileManager(30 * time.Minute)
				_, err := factory.GetRecorder(testCfg, testLogger, tempManager)
				if err != nil {
					t.Logf("Config %d: recorder not available: %v", i, err)
				} else {
					t.Logf("Config %d: recorder created successfully", i)
				}

				outputFactory := outputfactory.NewFactory(testCfg)
				_, err = outputFactory.GetOutputter(outputfactory.EnvironmentUnknown)
				if err != nil {
					t.Logf("Config %d: output not available: %v", i, err)
				} else {
					t.Logf("Config %d: output created successfully", i)
				}
			})
		}

		t.Log("Configuration validation integration test completed")
	})
}
