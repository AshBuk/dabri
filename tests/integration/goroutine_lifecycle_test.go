//go:build integration

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/providers"
	"github.com/AshBuk/dabri/v2/internal/app"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/testutils"
	sttproviders "github.com/AshBuk/dabri/v2/stt/providers"
	"go.uber.org/goleak"
)

// Shared goleak ignore rules for known long-lived goroutines from dependencies.
// The shared D-Bus session connection lives for the whole process: its reader
// blocks in a socket read (inWorker below the top frame) and newConn parks a
// goroutine until the connection context ends.
// HTTP transport pool goroutines linger after model download attempts;
// IgnoreAnyFunction is needed because the top of the stack is crypto/tls.(*Conn).Read,
// not the HTTP function itself.
var goLeakIgnores = []goleak.Option{
	goleak.IgnoreAnyFunction("github.com/godbus/dbus/v5.(*Conn).inWorker"),
	goleak.IgnoreTopFunction("github.com/godbus/dbus/v5.newConn.func1"),
	goleak.IgnoreAnyFunction("net/http.(*persistConn).writeLoop"),
	goleak.IgnoreAnyFunction("net/http.(*persistConn).readLoop"),
	goleak.IgnoreAnyFunction("net/http.(*http2ClientConn).readLoop"),
}

// TestMain enables goroutine leak detection for integration tests
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goLeakIgnores...)
}

// TestAppLifecycle builds the full app (real whisper engine, mock tray, no-op
// window) and checks that shutdown is quick and leaves no goroutines behind.
func TestAppLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	var defaults config.Config
	config.SetDefaultConfig(&defaults)
	model := constants.ModelByID(defaults.General.WhisperModel)
	if _, err := os.Stat(sttproviders.NewModelPathResolver(&defaults, model.FileName).GetBundledModelPath()); err != nil {
		t.Skipf("No local %s model; Initialize would download it", model.ID)
	}
	if _, err := exec.LookPath(defaults.Audio.RecordingMethod); err != nil {
		t.Skipf("Recorder %s not installed", defaults.Audio.RecordingMethod)
	}

	defer goleak.VerifyNone(t, goLeakIgnores...)

	a := app.NewApp(testutils.NewMockLogger())
	// A missing config file is created with defaults
	if err := a.Initialize(filepath.Join(t.TempDir(), "config.yaml"), false); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	start := time.Now()
	err := a.Shutdown()
	duration := time.Since(start)
	http.DefaultClient.CloseIdleConnections()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	if duration > 6*time.Second {
		t.Errorf("Shutdown took too long: %v (expected < 6s)", duration)
	}
}

// TestEvdevProviderLifecycle verifies evdev provider goroutine cleanup
func TestEvdevProviderLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	defer goleak.VerifyNone(t, goLeakIgnores...)

	mockLogger := testutils.NewMockLogger()
	provider := providers.NewEvdevKeyboardProvider(mockLogger)
	// Check if evdev is supported
	if !provider.IsSupported() {
		t.Skip("Evdev not supported in this environment")
	}

	// Start provider
	err := provider.Start()
	if err != nil {
		t.Skipf("Could not start evdev provider: %v", err)
	}
	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)
	// Stop should wait for all device listener goroutines
	start := time.Now()
	provider.Stop()
	duration := time.Since(start)
	// Should complete quickly (< 1 second)
	if duration > 1*time.Second {
		t.Errorf("Stop() took too long: %v (expected < 1s)", duration)
	}
	// Give goroutines time to fully exit
	time.Sleep(100 * time.Millisecond)

	t.Logf("Evdev provider stopped in %v", duration)
}

// TestDbusProviderLifecycle verifies dbus provider goroutine cleanup
func TestDbusProviderLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	defer goleak.VerifyNone(t, goLeakIgnores...)

	mockLogger := testutils.NewMockLogger()
	provider := providers.NewDbusKeyboardProvider(mockLogger)
	// Check if dbus is supported
	if !provider.IsSupported() {
		t.Skip("DBus not supported in this environment")
	}

	// Start provider
	err := provider.Start()
	if err != nil {
		t.Skipf("Could not start dbus provider: %v", err)
	}

	// Give listener time to start
	time.Sleep(50 * time.Millisecond)

	// Stop should wait for listener goroutine
	start := time.Now()
	provider.Stop()
	duration := time.Since(start)

	// Should complete quickly
	if duration > 1*time.Second {
		t.Errorf("Stop() took too long: %v (expected < 1s)", duration)
	}

	// Give goroutine time to exit
	time.Sleep(100 * time.Millisecond)

	t.Logf("DBus provider stopped in %v", duration)
}
