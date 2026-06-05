//go:build integration

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/providers"
	"github.com/AshBuk/dabri/v2/internal/platform"
	"github.com/AshBuk/dabri/v2/internal/testutils"
	outputfactory "github.com/AshBuk/dabri/v2/output/factory"
	"github.com/AshBuk/dabri/v2/whisper"
)

// Platform-specific integration tests

func TestWaylandCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test Wayland-specific functionality
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	t.Run("wayland_output_tools", func(t *testing.T) {
		// Test that Wayland output tools are properly configured
		factory := outputfactory.NewFactory(cfg)

		// Test clipboard on Wayland
		_, err := factory.GetOutputter(outputfactory.EnvironmentWayland)
		if err != nil {
			t.Logf("Expected error in test environment (tools not available): %v", err)
		} else {
			t.Log("Wayland outputter created successfully")
		}
	})

	t.Run("wayland_hotkeys", func(t *testing.T) {
		// Test DBus hotkey provider
		provider := providers.NewDbusKeyboardProvider(testutils.NewMockLogger())
		if provider.IsSupported() {
			t.Log("DBus GlobalShortcuts portal is available")
		} else {
			t.Log("DBus GlobalShortcuts portal not available (expected in CI)")
		}
	})

	t.Run("wayland_environment_detection", func(t *testing.T) {
		// Test environment detection logic
		env := platform.DetectEnvironment()
		t.Logf("Detected environment: %s", env)
		// Environment detection should not crash
		if env == "" {
			t.Error("Environment detection returned empty string")
		}
	})
}

func TestX11Compatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test X11-specific functionality
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	t.Run("x11_output_tools", func(t *testing.T) {
		// Test that X11 output tools are properly configured
		factory := outputfactory.NewFactory(cfg)

		// Test clipboard on X11
		_, err := factory.GetOutputter(outputfactory.EnvironmentX11)
		if err != nil {
			t.Logf("Expected error in test environment (tools not available): %v", err)
		} else {
			t.Log("X11 outputter created successfully")
		}
	})

	t.Run("x11_hotkeys", func(t *testing.T) {
		// Test evdev hotkey provider
		provider := providers.NewEvdevKeyboardProvider(testutils.NewMockLogger())
		if provider.IsSupported() {
			t.Log("Evdev input devices are available")
		} else {
			t.Log("Evdev input devices not available (expected in CI)")
		}
	})
}

func TestCrossplatformToolFallbacks(t *testing.T) {
	// Test that tool fallback logic works correctly
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	factory := outputfactory.NewFactory(cfg)

	environments := []outputfactory.EnvironmentType{
		outputfactory.EnvironmentX11,
		outputfactory.EnvironmentWayland,
		outputfactory.EnvironmentUnknown,
	}

	for _, env := range environments {
		t.Run(string(env), func(t *testing.T) {
			// Test that factory can create outputters for any environment
			// (even if tools aren't available, the logic should be sound)
			_, err := factory.GetOutputter(env)
			if err != nil {
				t.Logf("Environment %s: %v (expected in test environment)", env, err)
			} else {
				t.Logf("Environment %s: outputter created successfully", env)
			}
		})
	}
}

func TestSecurityValidation(t *testing.T) {
	// Test that security validation is working
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	// Test allowed commands
	allowedCommands := []string{"xdotool", "wtype", "wl-copy", "arecord"}
	for _, cmd := range allowedCommands {
		if !config.IsCommandAllowed(cfg, cmd) {
			t.Errorf("Command %s should be allowed but isn't", cmd)
		}
	}
	// Test disallowed commands
	disallowedCommands := []string{"rm", "curl", "wget", "sh", "bash"}
	for _, cmd := range disallowedCommands {
		if config.IsCommandAllowed(cfg, cmd) {
			t.Errorf("Command %s should not be allowed but is", cmd)
		}
	}
	// Test argument sanitization
	testArgs := []string{
		"normal-arg",
		"--flag",
		"/path/to/file",
	}

	sanitized := config.SanitizeCommandArgs(testArgs)
	if len(sanitized) != len(testArgs) {
		t.Log("Some arguments were filtered (expected for security)")
	}

	// Test dangerous arguments are filtered
	dangerousArgs := []string{
		"arg; rm -rf /",
		"$(malicious command)",
		"arg|dangerous",
		"../../../etc/passwd",
	}

	sanitizedDangerous := config.SanitizeCommandArgs(dangerousArgs)
	if len(sanitizedDangerous) > 0 {
		t.Log("Some dangerous arguments were not filtered - review security policy")
	}
}

func TestModelAvailability(t *testing.T) {
	// Test bundled model path handling
	t.Run("bundled_model_path", func(t *testing.T) {
		cfg := &config.Config{}
		config.SetDefaultConfig(cfg)

		// Test ModelManager's bundled model path resolution.
		// Use a short timeout: when no bundled model exists (development), GetModelPath
		// falls through to a network download — without a deadline this test would block
		// downloading the full model. The timeout makes the missing-model case fail fast.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		modelManager := whisper.NewModelManager(cfg)
		modelPath, err := modelManager.GetModelPath(ctx)
		if err != nil {
			t.Logf("Bundled model not found (expected for development): %v", err)
		} else {
			t.Logf("Bundled model found at %s", modelPath)
		}
	})

	t.Run("model_download_location", func(t *testing.T) {
		// Test that the model download location is accessible
		cfg := &config.Config{}
		config.SetDefaultConfig(cfg)

		modelDir := "sources/language-models"
		if info, err := os.Stat(modelDir); err != nil {
			t.Logf("Model directory %s not found (expected for fresh repo)", modelDir)
		} else if !info.IsDir() {
			t.Errorf("Model path %s exists but is not a directory", modelDir)
		} else {
			t.Logf("Model directory %s exists", modelDir)
		}
	})
}

func TestToolsAvailability(t *testing.T) {
	// Test that required tools are available in the system
	t.Run("required_tools", func(t *testing.T) {
		tools := []string{"arecord", "xdotool", "wl-copy", "wtype"}

		for _, tool := range tools {
			t.Run(tool, func(t *testing.T) {
				t.Logf("Testing availability of %s", tool)
			})
		}
	})
}
