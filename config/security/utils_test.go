// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package security

import (
	"testing"

	"github.com/AshBuk/dabri/v2/config/models"
)

func TestIsCommandAllowed(t *testing.T) {
	config := &models.Config{}
	config.Security.AllowedCommands = []string{"echo", "ls", "cat"}

	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{"allowed command", "echo", true},
		{"allowed command with path", "/bin/echo", true},
		{"disallowed command", "rm", false},
		{"empty command", "", false},
		{"malicious command", "rm -rf /", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCommandAllowed(config, tt.command)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSanitizeCommandArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "clean args",
			args:     []string{"echo", "hello", "world"},
			expected: []string{"echo", "hello", "world"},
		},
		{
			name:     "args with path traversal",
			args:     []string{"echo", "../passwd", "hello"},
			expected: []string{"echo", "hello"},
		},
		{
			name:     "args with dangerous chars",
			args:     []string{"echo", "hello;rm -rf /", "world"},
			expected: []string{"echo", "world"},
		},
		{
			name:     "empty args",
			args:     []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeCommandArgs(tt.args)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d args, got %d", len(tt.expected), len(result))
				return
			}
			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("expected arg %d to be %s, got %s", i, tt.expected[i], arg)
				}
			}
		})
	}
}
