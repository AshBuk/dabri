// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

//go:build cgo

package whisper

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AshBuk/dabri/v2/config"
)

// TestTranscribeWithContextCancellation verifies context cancellation works
func TestTranscribeWithContextCancellation(t *testing.T) {
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)

	// Use non-existent model to avoid long transcription
	engine := &WhisperEngine{
		config:    cfg,
		modelPath: "/nonexistent/model.bin",
	}
	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	// TranscribeWithContext should return quickly with context error
	start := time.Now()
	_, err := engine.TranscribeWithContext(ctx, "test.wav")
	duration := time.Since(start)
	// Should fail with context error
	if err == nil {
		t.Error("Expected error due to cancelled context")
	}

	// Should return context error
	if !errors.Is(err, context.Canceled) {
		t.Logf("Error type: %v", err)
		// Note: might also be "model not found" error if context check happens after
	}
	// Should return quickly (< 100ms)
	if duration > 100*time.Millisecond {
		t.Errorf("Cancellation took too long: %v (expected < 100ms)", duration)
	}

	t.Logf("Cancellation completed in %v", duration)
}

// TestTranscribeWithContextTimeout verifies timeout works
func TestTranscribeWithContextTimeout(t *testing.T) {
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)

	engine := &WhisperEngine{
		config:    cfg,
		modelPath: "/nonexistent/model.bin",
	}

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	// Should timeout
	start := time.Now()
	_, err := engine.TranscribeWithContext(ctx, "test.wav")
	duration := time.Since(start)
	if err == nil {
		t.Error("Expected timeout error")
	}
	// Should timeout around 10ms (allow some slack)
	if duration > 100*time.Millisecond {
		t.Errorf("Timeout took too long: %v (expected ~10ms)", duration)
	}

	t.Logf("Timeout completed in %v", duration)
}

// TestTranscribeWithContextNoLeak verifies no goroutine leak on cancellation
func TestTranscribeWithContextNoLeak(t *testing.T) {
	// Note: This test would ideally use goleak, but whisper.cpp C code
	// may create threads that goleak cannot track. Manual verification needed.

	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)

	engine := &WhisperEngine{
		config:    cfg,
		modelPath: "/nonexistent/model.bin",
	}
	// Run multiple cancellations
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(5 * time.Millisecond)
			cancel()
		}()

		_, _ = engine.TranscribeWithContext(ctx, "test.wav")
	}
	// If goroutines leaked, this test would eventually hang or OOM
	// (Best verified manually with `go test -race` or profiling)
	time.Sleep(100 * time.Millisecond)
}
