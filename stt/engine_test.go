//go:build cgo

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package stt

import (
	"testing"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/testutils"
)

func TestNewEngine(t *testing.T) {
	config := &config.Config{}
	modelPath := "/non/existent/model.bin"
	// Test that NewEngine returns error for non-existent model
	engine, err := NewEngine(testutils.StaticConfig(config), modelPath)
	if err == nil {
		t.Fatalf("Expected error for non-existent model, got nil")
	}

	if engine != nil {
		t.Errorf("Expected nil engine when model doesn't exist")
	}
	// Test error message
	expectedError := "whisper model not found: /non/existent/model.bin"
	if err.Error() != expectedError {
		t.Errorf("Expected error message %q, got %q", expectedError, err.Error())
	}
}
