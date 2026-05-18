//go:build linux

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"testing"
	"time"

	"github.com/AshBuk/dabri/v2/internal/testutils"
)

// TestEvdevStopStartRace verifies no race between Stop() and Start()
func TestEvdevStopStartRace(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())

	if !provider.IsSupported() {
		t.Skip("Evdev not supported (permissions or no devices)")
	}

	// Start provider
	if err := provider.Start(); err != nil {
		t.Skipf("Cannot start evdev: %v", err)
	}

	// Immediate Stop/Start cycle (simulates CaptureOnce flow)
	provider.Stop()

	// No sleep needed - Stop() waits synchronously
	if err := provider.Start(); err != nil {
		t.Fatalf("Start after Stop failed: %v", err)
	}

	// Verify devices are accessible
	if provider.devices == nil {
		t.Fatal("devices is nil after Start()")
	}

	if len(provider.devices) == 0 {
		t.Fatal("no devices after Start()")
	}

	// Cleanup
	provider.Stop()
}

// TestEvdevStopIdempotent verifies Stop() can be called multiple times
func TestEvdevStopIdempotent(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())

	if !provider.IsSupported() {
		t.Skip("Evdev not supported")
	}

	if err := provider.Start(); err != nil {
		t.Skipf("Cannot start evdev: %v", err)
	}

	// Multiple Stop() calls should not panic
	provider.Stop()
	provider.Stop()
	provider.Stop()
}

// TestEvdevRapidRestarts simulates config reload scenario
func TestEvdevRapidRestarts(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())

	if !provider.IsSupported() {
		t.Skip("Evdev not supported")
	}

	// Simulate 5 rapid config reloads
	for i := 0; i < 5; i++ {
		if err := provider.Start(); err != nil {
			t.Fatalf("Start #%d failed: %v", i, err)
		}

		// Immediate stop (no delay)
		provider.Stop()
	}

	// Final start should work
	if err := provider.Start(); err != nil {
		t.Fatalf("Final start failed: %v", err)
	}

	if provider.devices == nil {
		t.Fatal("devices is nil after rapid restarts")
	}

	provider.Stop()
}

// TestEvdevStopTimeout verifies timeout behavior
func TestEvdevStopTimeout(t *testing.T) {
	provider := NewEvdevKeyboardProvider(testutils.NewMockLogger())

	if !provider.IsSupported() {
		t.Skip("Evdev not supported")
	}

	if err := provider.Start(); err != nil {
		t.Skipf("Cannot start evdev: %v", err)
	}

	// Stop should complete within reasonable time (2s timeout + overhead)
	start := time.Now()
	provider.Stop()
	duration := time.Since(start)

	if duration > 3*time.Second {
		t.Errorf("Stop() took too long: %v (expected <3s)", duration)
	}

	t.Logf("Stop() completed in %v", duration)
}
