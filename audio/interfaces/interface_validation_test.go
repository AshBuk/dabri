// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package interfaces_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/AshBuk/dabri/v2/audio/interfaces"
	"github.com/AshBuk/dabri/v2/audio/mocks"
)

func TestAudioRecorderInterface_MockCompliance(t *testing.T) {
	// Test that MockAudioRecorder implements AudioRecorder interface
	mock := mocks.NewMockAudioRecorder()

	// Verify interface compliance at compile time
	var _ interfaces.AudioRecorder = mock

	// Test all interface methods
	t.Run("StartRecording", func(t *testing.T) {
		err := mock.StartRecording()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !mock.IsRecording() {
			t.Error("expected recording to be started")
		}
	})

	t.Run("StopRecording", func(t *testing.T) {
		result, err := mock.StopRecording()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == "" {
			t.Error("expected non-empty result")
		}
		if mock.IsRecording() {
			t.Error("expected recording to be stopped")
		}
	})

	t.Run("GetOutputFile", func(t *testing.T) {
		file := mock.GetOutputFile()
		if file == "" {
			t.Error("expected non-empty output file")
		}
	})

	t.Run("CleanupFile", func(t *testing.T) {
		err := mock.CleanupFile()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !mock.WasCleanupCalled() {
			t.Error("expected cleanup to be called")
		}
	})

	t.Run("AudioLevelCallback", func(t *testing.T) {
		var receivedLevel float64
		callback := func(level float64) {
			receivedLevel = level
		}

		mock.SetAudioLevelCallback(callback)
		mock.SetAudioLevel(0.5)

		if receivedLevel != 0.5 {
			t.Errorf("expected level 0.5, got %f", receivedLevel)
		}
	})

	t.Run("GetAudioLevel", func(t *testing.T) {
		mock.SetAudioLevel(0.7)
		level := mock.GetAudioLevel()
		if level != 0.7 {
			t.Errorf("expected level 0.7, got %f", level)
		}
	})
}

func TestAudioRecorderInterface_ErrorHandling(t *testing.T) {
	mock := mocks.NewMockAudioRecorder()

	t.Run("StartRecording_Error", func(t *testing.T) {
		expectedError := errors.New("start recording failed")
		mock.SetStartError(expectedError)

		err := mock.StartRecording()
		if err == nil {
			t.Error("expected error but got none")
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("expected error %q, got %q", expectedError.Error(), err.Error())
		}
	})

	t.Run("StopRecording_Error", func(t *testing.T) {
		mock.Reset()
		expectedError := errors.New("stop recording failed")
		mock.SetStopError(expectedError)

		// Start recording first
		mock.StartRecording()

		result, err := mock.StopRecording()
		if err == nil {
			t.Error("expected error but got none")
		}
		if result != "" {
			t.Errorf("expected empty result on error, got %q", result)
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("expected error %q, got %q", expectedError.Error(), err.Error())
		}
	})

	t.Run("CleanupFile_Error", func(t *testing.T) {
		mock.Reset()
		expectedError := errors.New("cleanup failed")
		mock.SetCleanupError(expectedError)

		err := mock.CleanupFile()
		if err == nil {
			t.Error("expected error but got none")
		}
		if err.Error() != expectedError.Error() {
			t.Errorf("expected error %q, got %q", expectedError.Error(), err.Error())
		}
	})

}

func TestAudioRecorderInterface_StateValidation(t *testing.T) {
	mock := mocks.NewMockAudioRecorder()

	t.Run("DoubleStart", func(t *testing.T) {
		err := mock.StartRecording()
		if err != nil {
			t.Fatalf("first start failed: %v", err)
		}

		err = mock.StartRecording()
		if err == nil {
			t.Error("expected error when starting recording twice")
		}
	})

	t.Run("StopWithoutStart", func(t *testing.T) {
		mock.Reset()

		result, err := mock.StopRecording()
		if err == nil {
			t.Error("expected error when stopping without starting")
		}
		if result != "" {
			t.Error("expected empty result when stopping without starting")
		}
	})

	t.Run("StateTransitions", func(t *testing.T) {
		mock.Reset()

		// Initial state
		if mock.IsRecording() {
			t.Error("expected recording to be false initially")
		}

		// Start recording
		err := mock.StartRecording()
		if err != nil {
			t.Fatalf("start recording failed: %v", err)
		}
		if !mock.IsRecording() {
			t.Error("expected recording to be true after start")
		}

		// Stop recording
		_, err = mock.StopRecording()
		if err != nil {
			t.Fatalf("stop recording failed: %v", err)
		}
		if mock.IsRecording() {
			t.Error("expected recording to be false after stop")
		}
	})
}

func TestAudioRecorderInterface_AudioLevelMonitoring(t *testing.T) {
	mock := mocks.NewMockAudioRecorder()

	t.Run("DefaultAudioLevel", func(t *testing.T) {
		level := mock.GetAudioLevel()
		if level != 0.0 {
			t.Errorf("expected default audio level to be 0.0, got %f", level)
		}
	})

	t.Run("SetAudioLevel", func(t *testing.T) {
		mock.SetAudioLevel(0.5)
		level := mock.GetAudioLevel()
		if level != 0.5 {
			t.Errorf("expected audio level to be 0.5, got %f", level)
		}
	})

	t.Run("AudioLevelCallback", func(t *testing.T) {
		var receivedLevels []float64
		callback := func(level float64) {
			receivedLevels = append(receivedLevels, level)
		}

		mock.SetAudioLevelCallback(callback)

		testLevels := []float64{0.1, 0.3, 0.5, 0.7, 0.4}
		for _, level := range testLevels {
			mock.SetAudioLevel(level)
		}

		if len(receivedLevels) != len(testLevels) {
			t.Errorf("expected %d callbacks, got %d", len(testLevels), len(receivedLevels))
		}

		for i, expected := range testLevels {
			if i < len(receivedLevels) && receivedLevels[i] != expected {
				t.Errorf("expected level %f at index %d, got %f", expected, i, receivedLevels[i])
			}
		}
	})

	t.Run("AudioLevelSequence", func(t *testing.T) {
		mock.Reset()
		testSequence := []float64{0.1, 0.3, 0.5, 0.7, 0.4, 0.2}
		mock.SetAudioLevelSequence(testSequence)
		mock.EnableAudioLevelSimulation()

		var mu sync.Mutex
		var receivedLevels []float64
		callback := func(level float64) {
			mu.Lock()
			receivedLevels = append(receivedLevels, level)
			mu.Unlock()
		}
		mock.SetAudioLevelCallback(callback)

		// Start recording to trigger simulation
		err := mock.StartRecording()
		if err != nil {
			t.Fatalf("start recording failed: %v", err)
		}

		// Wait for some simulation
		time.Sleep(200 * time.Millisecond)

		// Stop recording
		mock.StopRecording()
		// Allow simulation goroutine to observe isRecording=false and exit
		time.Sleep(150 * time.Millisecond)

		mu.Lock()
		count := len(receivedLevels)
		mu.Unlock()
		if count == 0 {
			t.Error("expected to receive audio level updates")
		}
	})
}

func TestAudioRecorderInterface_EdgeCases(t *testing.T) {
	mock := mocks.NewMockAudioRecorder()

	t.Run("NilCallback", func(t *testing.T) {
		// Should not panic
		mock.SetAudioLevelCallback(nil)
		mock.SetAudioLevel(0.5)

		// Test that it doesn't crash
		level := mock.GetAudioLevel()
		if level != 0.5 {
			t.Errorf("expected level 0.5, got %f", level)
		}
	})

	t.Run("EmptyOutputFile", func(t *testing.T) {
		mock.SetOutputFile("")
		file := mock.GetOutputFile()
		if file != "" {
			t.Errorf("expected empty output file, got %q", file)
		}
	})

	t.Run("EmptyRecordingResult", func(t *testing.T) {
		mock.Reset()
		mock.SetRecordingResult("")

		err := mock.StartRecording()
		if err != nil {
			t.Fatalf("start recording failed: %v", err)
		}

		result, err := mock.StopRecording()
		if err != nil {
			t.Fatalf("stop recording failed: %v", err)
		}

		if result != "" {
			t.Errorf("expected empty result, got %q", result)
		}
	})

	t.Run("ExtremeAudioLevels", func(t *testing.T) {
		testLevels := []float64{-1.0, 0.0, 1.0, 2.0, -0.5}

		for _, level := range testLevels {
			mock.SetAudioLevel(level)
			retrieved := mock.GetAudioLevel()
			if retrieved != level {
				t.Errorf("expected level %f, got %f", level, retrieved)
			}
		}
	})
}

func TestAudioRecorderInterface_ConcurrentAccess(t *testing.T) {
	mock := mocks.NewMockAudioRecorder()

	t.Run("ConcurrentLevelUpdates", func(t *testing.T) {
		var mu sync.Mutex
		var receivedLevels []float64
		callback := func(level float64) {
			mu.Lock()
			receivedLevels = append(receivedLevels, level)
			mu.Unlock()
		}
		mock.SetAudioLevelCallback(callback)

		// Start multiple goroutines updating levels
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(level float64) {
				defer func() { done <- true }()
				mock.SetAudioLevel(level)
			}(float64(i) * 0.1)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should not crash and should have received some updates
		mu.Lock()
		count := len(receivedLevels)
		mu.Unlock()
		if count == 0 {
			t.Error("expected to receive some audio level updates")
		}
	})

	t.Run("ConcurrentStartStop", func(t *testing.T) {
		mock.Reset()

		// Start multiple goroutines trying to start/stop recording
		done := make(chan bool)
		errors := make(chan error, 20)

		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				err := mock.StartRecording()
				if err != nil {
					errors <- err
				}
			}()

			go func() {
				defer func() { done <- true }()
				_, err := mock.StopRecording()
				if err != nil {
					errors <- err
				}
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 20; i++ {
			<-done
		}

		// Should handle concurrent access gracefully
		close(errors)
		errorCount := 0
		for range errors {
			errorCount++
		}

		// Some errors are expected due to invalid state transitions
		if errorCount == 0 {
			t.Log("No errors occurred during concurrent access")
		} else {
			t.Logf("Handled %d errors during concurrent access", errorCount)
		}
	})
}
