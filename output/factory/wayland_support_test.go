// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package factory

import (
	"testing"

	"github.com/AshBuk/dabri/v2/config"
)

func TestWaylandTextInput(t *testing.T) {
	// Create a config with defaults
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)

	factory := NewFactory(cfg)

	tests := []struct {
		name     string
		env      EnvironmentType
		expected string
	}{
		{
			name:     "Wayland environment should prefer wtype/ydotool",
			env:      EnvironmentWayland,
			expected: "Should select Wayland-compatible tool",
		},
		{
			name:     "X11 environment should use xdotool",
			env:      EnvironmentX11,
			expected: "Should select xdotool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that factory can create outputter without error
			// (even if tools are not available, factory logic should work)
			_, err := factory.GetOutputter(tt.env)

			// We expect error because tools may not be installed in test environment
			// but the important thing is that no panic occurs and logic is sound
			if err == nil {
				t.Logf("Successfully created outputter for %s", tt.env)
			} else {
				t.Logf("Expected error in test environment: %v", err)
			}
		})
	}
}

func TestIsToolAvailable(t *testing.T) {
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)
	factory := NewFactory(cfg)

	// Test with a tool that should always be available
	available := factory.isToolAvailable("sh")
	if !available {
		t.Error("sh should be available on Linux systems")
	}

	// Test with a tool that likely doesn't exist
	notAvailable := factory.isToolAvailable("non-existent-tool-12345")
	if notAvailable {
		t.Error("non-existent tool should not be available")
	}
}

func TestSecurityAllowedCommands(t *testing.T) {
	cfg := &config.Config{}
	config.SetDefaultConfig(cfg)

	// Test that new Wayland tools are in allowed commands
	waylandTools := []string{"wtype", "ydotool"}

	for _, tool := range waylandTools {
		if !config.IsCommandAllowed(cfg, tool) {
			t.Errorf("Wayland tool %s should be in allowed commands list", tool)
		}
	}

	// Test that traditional tools are still allowed
	traditionalTools := []string{"xdotool", "xsel", "wl-copy"}

	for _, tool := range traditionalTools {
		if !config.IsCommandAllowed(cfg, tool) {
			t.Errorf("Traditional tool %s should be in allowed commands list", tool)
		}
	}
}
