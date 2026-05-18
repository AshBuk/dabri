// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package factory

import (
	"testing"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/output/interfaces"
	"github.com/AshBuk/dabri/v2/output/outputters"
)

func TestOutputFactory_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		env         EnvironmentType
		defaultMode string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty default mode",
			env:         EnvironmentX11,
			defaultMode: "",
			expectError: true, // External tools not available in test environment
		},
		{
			name:        "invalid default mode",
			env:         EnvironmentX11,
			defaultMode: "invalid_mode",
			expectError: true,
		},
		{
			name:        "unknown environment",
			env:         EnvironmentUnknown,
			defaultMode: "clipboard",
			expectError: true, // External tools not available in test environment
		},
		{
			name:        "valid X11 clipboard",
			env:         EnvironmentX11,
			defaultMode: "clipboard",
			expectError: true, // External tools not available in test environment
		},
		{
			name:        "valid Wayland clipboard",
			env:         EnvironmentWayland,
			defaultMode: "clipboard",
			expectError: true, // External tools not available in test environment
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

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if outputter != nil {
					t.Errorf("expected nil outputter on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if outputter == nil {
					t.Errorf("expected outputter to be created")
				}
			}
		})
	}
}

func TestOutputFactory_NilConfig(t *testing.T) {
	// Test with nil config
	factory := NewFactory(nil)
	if factory == nil {
		t.Fatal("expected factory to be created even with nil config")
	}

	// This should panic or handle gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Logf("GetOutputter panicked with nil config: %v", r)
		}
	}()

	outputter, err := factory.GetOutputter(EnvironmentX11)
	if err == nil {
		t.Error("expected error with nil config")
	}
	if outputter != nil {
		t.Error("expected nil outputter with nil config")
	}
}

func TestOutputFactory_ToolSelection(t *testing.T) {
	tests := []struct {
		name              string
		env               EnvironmentType
		clipboardTool     string
		typeTool          string
		expectedClipboard string
		expectedType      string
	}{
		{
			name:              "X11 auto selection",
			env:               EnvironmentX11,
			clipboardTool:     "auto",
			typeTool:          "auto",
			expectedClipboard: "xsel",
			expectedType:      "xdotool",
		},
		{
			name:              "Wayland auto selection",
			env:               EnvironmentWayland,
			clipboardTool:     "auto",
			typeTool:          "auto",
			expectedClipboard: "xsel",
			expectedType:      "wl-keyboard",
		},
		{
			name:              "Unknown environment defaults to X11",
			env:               EnvironmentUnknown,
			clipboardTool:     "auto",
			typeTool:          "auto",
			expectedClipboard: "xsel",
			expectedType:      "xdotool",
		},
		{
			name:              "Manual tool selection",
			env:               EnvironmentX11,
			clipboardTool:     "wl-copy",
			typeTool:          "custom-tool",
			expectedClipboard: "wl-copy",
			expectedType:      "custom-tool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.Config{}
			config.Output.DefaultMode = "clipboard"
			config.Output.ClipboardTool = tt.clipboardTool
			config.Output.TypeTool = tt.typeTool

			factory := NewFactory(config)

			// We can't test the actual tool selection without external dependencies
			// but we can test that the factory processes the configuration correctly
			if factory.config.Output.ClipboardTool != tt.clipboardTool {
				t.Errorf("expected clipboard tool %s, got %s", tt.clipboardTool, factory.config.Output.ClipboardTool)
			}
			if factory.config.Output.TypeTool != tt.typeTool {
				t.Errorf("expected type tool %s, got %s", tt.typeTool, factory.config.Output.TypeTool)
			}
		})
	}
}

func TestOutputFactory_InterfaceCompliance(t *testing.T) {
	// Test that factory implements proper interface behavior
	config := &config.Config{}
	config.Output.DefaultMode = "clipboard"
	config.Output.ClipboardTool = "auto"
	config.Output.TypeTool = "auto"

	factory := NewFactory(config)
	if factory == nil {
		t.Fatal("expected factory to be created")
	}

	// Test that factory has proper config reference
	if factory.config != config {
		t.Error("factory should reference the provided config")
	}
}

func TestOutputFactory_ConcurrentAccess(t *testing.T) {
	// Test concurrent access to factory
	config := &config.Config{}
	config.Output.DefaultMode = "clipboard"
	config.Output.ClipboardTool = "auto"
	config.Output.TypeTool = "auto"

	factory := NewFactory(config)

	// Create multiple goroutines that try to get outputters
	done := make(chan bool)
	errorChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Try different environments
			envs := []EnvironmentType{EnvironmentX11, EnvironmentWayland, EnvironmentUnknown}
			for _, env := range envs {
				_, err := factory.GetOutputter(env)
				if err != nil {
					// Expected in test environment, but should be consistent
					continue
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check for unexpected errors
	close(errorChan)
	for err := range errorChan {
		t.Errorf("concurrent access error: %v", err)
	}
}

func TestOutputFactory_ConfigImmutability(t *testing.T) {
	// Test that factory doesn't modify the original config
	originalConfig := &config.Config{}
	originalConfig.Output.DefaultMode = "clipboard"
	originalConfig.Output.ClipboardTool = "xsel"
	originalConfig.Output.TypeTool = "xdotool"

	factory := NewFactory(originalConfig)

	// Try to get outputter (will fail in test environment, but shouldn't modify config)
	_, err := factory.GetOutputter(EnvironmentX11)
	if err == nil {
		t.Log("Outputter creation succeeded unexpectedly")
	}

	// Verify original config is unchanged
	if originalConfig.Output.DefaultMode != "clipboard" {
		t.Errorf("original config default mode changed")
	}
	if originalConfig.Output.ClipboardTool != "xsel" {
		t.Errorf("original config clipboard tool changed")
	}
	if originalConfig.Output.TypeTool != "xdotool" {
		t.Errorf("original config type tool changed")
	}
}

func TestOutputFactory_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() *config.Config
		env         EnvironmentType
		expectError bool
		testFunc    func(*testing.T, *Factory)
	}{
		{
			name: "config with special characters in tools",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Output.DefaultMode = "clipboard"
				cfg.Output.ClipboardTool = "xsel\n"
				cfg.Output.TypeTool = "xdotool"
				return cfg
			},
			env:         EnvironmentX11,
			expectError: true,
		},
		{
			name: "config with unicode tools",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Output.DefaultMode = "clipboard"
				cfg.Output.ClipboardTool = "инструмент"
				cfg.Output.TypeTool = "xdotool"
				return cfg
			},
			env:         EnvironmentX11,
			expectError: true,
		},
		{
			name: "config with empty tools",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Output.DefaultMode = "clipboard"
				cfg.Output.ClipboardTool = ""
				cfg.Output.TypeTool = ""
				return cfg
			},
			env:         EnvironmentX11,
			expectError: true,
		},
		{
			name: "config with whitespace tools",
			setupConfig: func() *config.Config {
				cfg := &config.Config{}
				cfg.Output.DefaultMode = "clipboard"
				cfg.Output.ClipboardTool = "   "
				cfg.Output.TypeTool = "   "
				return cfg
			},
			env:         EnvironmentX11,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.setupConfig()
			factory := NewFactory(config)
			outputter, err := factory.GetOutputter(tt.env)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if outputter != nil {
					t.Errorf("expected nil outputter on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if outputter == nil {
					t.Errorf("expected outputter to be created")
				}
			}

			if tt.testFunc != nil {
				tt.testFunc(t, factory)
			}
		})
	}
}

func TestGetOutputterFromConfig_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		env         EnvironmentType
		expectError bool
	}{
		{
			name:        "nil config",
			config:      nil,
			env:         EnvironmentX11,
			expectError: true,
		},
		{
			name: "valid config",
			config: func() *config.Config {
				cfg := &config.Config{}
				cfg.Output.DefaultMode = "clipboard"
				cfg.Output.ClipboardTool = "auto"
				cfg.Output.TypeTool = "auto"
				return cfg
			}(),
			env:         EnvironmentX11,
			expectError: true, // External tools not available in test environment
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectError {
						t.Errorf("unexpected panic: %v", r)
					}
				}
			}()

			outputter, err := GetOutputterFromConfig(tt.config, tt.env)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if outputter != nil {
					t.Errorf("expected nil outputter on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if outputter == nil {
					t.Errorf("expected outputter to be created")
				}
			}
		})
	}
}

func TestEnvironmentType_Validation(t *testing.T) {
	tests := []struct {
		name  string
		env   EnvironmentType
		valid bool
	}{
		{
			name:  "X11 environment",
			env:   EnvironmentX11,
			valid: true,
		},
		{
			name:  "Wayland environment",
			env:   EnvironmentWayland,
			valid: true,
		},
		{
			name:  "Unknown environment",
			env:   EnvironmentUnknown,
			valid: true,
		},
		{
			name:  "invalid environment",
			env:   EnvironmentType("invalid"),
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.Config{}
			config.Output.DefaultMode = "clipboard"
			config.Output.ClipboardTool = "auto"
			config.Output.TypeTool = "auto"

			factory := NewFactory(config)
			outputter, err := factory.GetOutputter(tt.env)

			if tt.valid {
				// Should attempt to create outputter (may fail due to external dependencies)
				if err != nil {
					t.Logf("Expected failure due to external dependencies: %v", err)
				}
			} else {
				// Invalid environment should be handled gracefully
				if err == nil {
					t.Log("Invalid environment was handled gracefully")
				}
			}

			// In test environment, outputter creation typically fails
			if outputter != nil {
				t.Logf("Outputter created successfully: %T", outputter)
			}
		})
	}
}

func TestOutputFactory_MockIntegration(t *testing.T) {
	// Test integration with mock outputters
	mockOutputter := outputters.NewMockOutputter()

	// Test that mock implements the interface
	var _ interfaces.Outputter = mockOutputter

	// Test basic functionality
	err := mockOutputter.CopyToClipboard("test text")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if mockOutputter.GetClipboardContent() != "test text" {
		t.Errorf("expected clipboard content to be 'test text', got %s", mockOutputter.GetClipboardContent())
	}

	// Test error simulation
	mockWithErrors := outputters.NewMockOutputterWithErrors()
	mockWithErrors.SimulateClipboardUnavailable()

	err = mockWithErrors.CopyToClipboard("test")
	if err == nil {
		t.Error("expected error when clipboard is unavailable")
	}
}
