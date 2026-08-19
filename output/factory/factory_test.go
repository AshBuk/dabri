// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package factory

import (
	"testing"

	"github.com/AshBuk/dabri/v2/config"
)

// pinPortal fixes the RemoteDesktop portal probe for one test. Without it the
// Wayland active-window path depends on whether the session's portal implements
// RemoteDesktop: GNOME does, wlroots does not.
func pinPortal(t *testing.T, available bool) {
	t.Helper()
	prev := portalAvailable
	portalAvailable = func() bool { return available }
	t.Cleanup(func() { portalAvailable = prev })
}

func TestNewFactory(t *testing.T) {
	config := &config.Config{}
	factory := NewFactory(config)
	if factory.config != config {
		t.Errorf("expected config to be set correctly")
	}
}

func TestFactory_GetOutputter(t *testing.T) {
	// The empty allowlist below is what makes every case fail; keep the portal
	// out of it, since it is not a command and would bypass the allowlist.
	pinPortal(t, false)

	tests := []struct {
		name        string
		env         EnvironmentType
		defaultMode string
		expectError bool
	}{
		{
			name:        "X11 clipboard mode",
			env:         EnvironmentX11,
			defaultMode: "clipboard",
			expectError: true, // Empty allowlist rejects every tool
		},
		{
			name:        "Wayland clipboard mode",
			env:         EnvironmentWayland,
			defaultMode: "clipboard",
			expectError: true, // Empty allowlist rejects every tool
		},
		{
			name:        "X11 active window mode",
			env:         EnvironmentX11,
			defaultMode: "active_window",
			expectError: true, // Empty allowlist rejects every tool
		},
		{
			name:        "Wayland active window mode",
			env:         EnvironmentWayland,
			defaultMode: "active_window",
			expectError: true, // Empty allowlist rejects every tool
		},
		{
			name:        "Unknown environment default mode",
			env:         EnvironmentUnknown,
			defaultMode: "clipboard",
			expectError: true, // Empty allowlist rejects every tool
		},
		{
			name:        "empty default mode falls back to clipboard",
			env:         EnvironmentX11,
			defaultMode: "",
			expectError: true, // Empty allowlist rejects every tool
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.Config{}
			config.Output.DefaultMode = tt.defaultMode
			config.Output.ClipboardTool = "auto"
			config.Output.TypeTool = "auto"

			factory := NewFactory(config)
			outputter, err := factory.GetOutputter(tt.env)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && outputter == nil {
				t.Errorf("expected outputter to be created")
			}
		})
	}
}

func TestFactory_GetOutputter_ToolSelection(t *testing.T) {
	tests := []struct {
		name              string
		env               EnvironmentType
		clipboardTool     string
		typeTool          string
		expectedClipboard string
		expectedType      string
		expectError       bool
	}{
		{
			name:              "X11 auto selection",
			env:               EnvironmentX11,
			clipboardTool:     "auto",
			typeTool:          "auto",
			expectedClipboard: "xsel",
			expectedType:      "xdotool",
			expectError:       true, // Empty allowlist rejects every tool
		},
		{
			name:              "Wayland auto selection",
			env:               EnvironmentWayland,
			clipboardTool:     "auto",
			typeTool:          "auto",
			expectedClipboard: "wl-copy",
			expectedType:      "wl-keyboard",
			expectError:       true, // Empty allowlist rejects every tool
		},
		{
			name:              "Unknown environment auto selection",
			env:               EnvironmentUnknown,
			clipboardTool:     "auto",
			typeTool:          "auto",
			expectedClipboard: "xsel",
			expectedType:      "xdotool",
			expectError:       true, // Empty allowlist rejects every tool
		},
		{
			name:              "manual tool selection",
			env:               EnvironmentX11,
			clipboardTool:     "wl-copy",
			typeTool:          "custom-tool",
			expectedClipboard: "wl-copy",
			expectedType:      "custom-tool",
			expectError:       true, // Empty allowlist rejects every tool
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.Config{}
			config.Output.DefaultMode = "clipboard"
			config.Output.ClipboardTool = tt.clipboardTool
			config.Output.TypeTool = tt.typeTool

			factory := NewFactory(config)
			outputter, err := factory.GetOutputter(tt.env)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && outputter == nil {
				t.Errorf("expected outputter to be created")
			}

			// In a real test environment, we would test tool selection logic
			// by mocking the external dependencies or using dependency injection
		})
	}
}

func TestGetOutputterFromConfig(t *testing.T) {
	// The empty allowlist below is what makes every case fail; keep the portal
	// out of it, since it is not a command and would bypass the allowlist.
	pinPortal(t, false)

	tests := []struct {
		name        string
		env         EnvironmentType
		defaultMode string
		expectError bool
	}{
		{
			name:        "valid X11 config",
			env:         EnvironmentX11,
			defaultMode: "clipboard",
			expectError: true, // Empty allowlist rejects every tool
		},
		{
			name:        "valid Wayland config",
			env:         EnvironmentWayland,
			defaultMode: "active_window",
			expectError: true, // Empty allowlist rejects every tool
		},
		{
			name:        "unknown environment",
			env:         EnvironmentUnknown,
			defaultMode: "clipboard",
			expectError: true, // Empty allowlist rejects every tool
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.Config{}
			config.Output.DefaultMode = tt.defaultMode
			config.Output.ClipboardTool = "auto"
			config.Output.TypeTool = "auto"

			outputter, err := GetOutputterFromConfig(config, tt.env)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.expectError && outputter == nil {
				t.Errorf("expected outputter to be created")
			}
		})
	}
}

func TestEnvironmentType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		env      EnvironmentType
		expected string
	}{
		{
			name:     "X11 environment",
			env:      EnvironmentX11,
			expected: "X11",
		},
		{
			name:     "Wayland environment",
			env:      EnvironmentWayland,
			expected: "Wayland",
		},
		{
			name:     "Unknown environment",
			env:      EnvironmentUnknown,
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.env) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.env))
			}
		})
	}
}
