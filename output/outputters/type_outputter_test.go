// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/output/interfaces"
)

func TestNewTypeOutputter(t *testing.T) {
	tests := []struct {
		name        string
		typeTool    string
		expectError bool
	}{
		{
			name:        "nonexistent tool",
			typeTool:    "nonexistent-type-tool",
			expectError: true,
		},
		{
			name:        "empty tool name",
			typeTool:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			outputter, err := NewTypeOutputter(tt.typeTool, cfg)
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

func TestNewTypeOutputter_WithExistingTool(t *testing.T) {
	// Find an existing command for testing
	var existingTool string
	for _, tool := range []string{"cat", "echo", "true"} {
		if _, err := exec.LookPath(tool); err == nil {
			existingTool = tool
			break
		}
	}

	if existingTool == "" {
		t.Skip("No basic tools available for testing")
	}

	cfg := &config.Config{}
	outputter, err := NewTypeOutputter(existingTool, cfg)
	if err != nil {
		t.Errorf("unexpected error with existing tool: %v", err)
	}
	if outputter == nil {
		t.Error("expected outputter to be created with existing tool")
	}

	typeOutputter, ok := outputter.(*TypeOutputter)
	if !ok {
		t.Error("expected TypeOutputter type")
	}
	if typeOutputter.typeTool != existingTool {
		t.Errorf("expected typeTool %s, got %s", existingTool, typeOutputter.typeTool)
	}
	if typeOutputter.config != cfg {
		t.Error("expected config to be set correctly")
	}
}

func TestTypeOutputter_TypeToActiveWindow_CommandNotAllowed(t *testing.T) {
	cfg := &config.Config{}
	// Create a mock config that doesn't allow any commands
	cfg.Security.AllowedCommands = []string{} // Empty list means no commands allowed

	outputter := &TypeOutputter{
		typeTool: "xdotool",
		config:   cfg,
	}
	err := outputter.TypeToActiveWindow("test text")
	if err == nil {
		t.Error("expected error for disallowed command")
	}
	if err != nil && err.Error() != "typing tool not allowed: xdotool" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTypeOutputter_TypeToActiveWindow_UnsupportedTool(t *testing.T) {
	cfg := &config.Config{}
	cfg.Security.AllowedCommands = []string{"unsupported-tool"} // Allow the tool

	outputter := &TypeOutputter{
		typeTool: "unsupported-tool",
		config:   cfg,
	}
	err := outputter.TypeToActiveWindow("test text")
	if err == nil {
		t.Error("expected error for unsupported tool")
	}
	if err != nil && err.Error() != "unsupported typing tool: unsupported-tool" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTypeOutputter_TypeToActiveWindow_SupportedTools(t *testing.T) {
	tests := []struct {
		name         string
		tool         string
		expectedArgs []string
	}{
		{
			name: "xdotool",
			tool: "xdotool",
			expectedArgs: []string{
				"type",
				"--clearmodifiers",
				"--",
				"test text",
			},
		},
		{
			name:         "wtype",
			tool:         "wtype",
			expectedArgs: []string{"--", "test text"},
		},
		{
			name:         "ydotool",
			tool:         "ydotool",
			expectedArgs: []string{"type", "test text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureFile := installFakeTool(t, tt.tool)
			cfg := &config.Config{}
			cfg.Security.AllowedCommands = []string{tt.tool}

			outputter := &TypeOutputter{
				typeTool: tt.tool,
				config:   cfg,
			}

			err := outputter.TypeToActiveWindow("test text")
			if err == nil {
				args := readCapturedArgs(t, captureFile)
				if strings.Join(args, "\n") != strings.Join(tt.expectedArgs, "\n") {
					t.Fatalf("expected args %q, got %q", tt.expectedArgs, args)
				}
				return
			}
			t.Fatalf("expected fake %s to run successfully, got %v", tt.tool, err)
		})
	}
}

func installFakeTool(t *testing.T, tool string) string {
	t.Helper()

	dir := t.TempDir()
	captureFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, tool)
	content := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CAPTURE_FILE\"\n"
	if err := os.WriteFile(script, []byte(content), 0700); err != nil {
		t.Fatalf("write fake tool: %v", err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("CAPTURE_FILE", captureFile)
	return captureFile
}

func readCapturedArgs(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read captured args: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func TestTypeOutputter_CopyToClipboard(t *testing.T) {
	cfg := &config.Config{}
	outputter := &TypeOutputter{
		typeTool: "xdotool",
		config:   cfg,
	}

	err := outputter.CopyToClipboard("test text")
	if err == nil {
		t.Error("expected error for unsupported operation")
	}
	expectedError := "copying to clipboard not supported by type outputter"
	if err.Error() != expectedError {
		t.Errorf("expected error message %q, got %q", expectedError, err.Error())
	}
}

func TestTypeOutputter_Interface(t *testing.T) {
	// Verify it implements the interfaces.Outputter interface
	var _ interfaces.Outputter = (*TypeOutputter)(nil)
}
