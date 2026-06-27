// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
)

func TestAutoPasteOutputter_TypeToActiveWindow_NonASCIIUsesClipboardPaste(t *testing.T) {
	captureFile := installFakePasteTool(t)
	typing := NewMockOutputter()
	clipboard := NewMockOutputter()
	cfg := &config.Config{}
	cfg.Security.AllowedCommands = []string{"ydotool"}

	outputter := NewAutoPasteOutputter(typing, clipboard, "ydotool", cfg).(*AutoPasteOutputter)
	outputter.pasteDelay = 0

	if err := outputter.TypeToActiveWindow("Привет мир"); err != nil {
		t.Fatalf("expected paste fallback to succeed, got %v", err)
	}
	if typing.WasTypeCalled() {
		t.Fatal("expected typing backend not to be used for non-ASCII text")
	}
	if got := clipboard.GetLastClipboardCall(); got != "Привет мир" {
		t.Fatalf("expected clipboard text %q, got %q", "Привет мир", got)
	}

	args := readCapturedArgs(t, captureFile)
	expected := []string{"key", "29:1", "47:1", "47:0", "29:0"}
	if strings.Join(args, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("expected args %q, got %q", expected, args)
	}
}

func TestAutoPasteOutputter_TypeToActiveWindow_ASCIIUsesTyping(t *testing.T) {
	typing := NewMockOutputter()
	clipboard := NewMockOutputter()
	cfg := &config.Config{}
	cfg.Security.AllowedCommands = []string{"ydotool"}

	outputter := NewAutoPasteOutputter(typing, clipboard, "ydotool", cfg)
	if err := outputter.TypeToActiveWindow("hello world"); err != nil {
		t.Fatalf("expected typing backend to succeed, got %v", err)
	}
	if got := typing.GetLastTypeCall(); got != "hello world" {
		t.Fatalf("expected typed text %q, got %q", "hello world", got)
	}
	if clipboard.WasClipboardCalled() {
		t.Fatal("expected clipboard backend not to be used for ASCII text")
	}
}

func installFakePasteTool(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	captureFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "ydotool")
	content := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CAPTURE_FILE\"\n"
	if err := os.WriteFile(script, []byte(content), 0700); err != nil {
		t.Fatalf("write fake ydotool: %v", err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("CAPTURE_FILE", captureFile)
	return captureFile
}
