//go:build linux

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"testing"

	"github.com/AshBuk/dabri/v2/internal/testutils"
)

func TestConvertHotkeyToAccelerator(t *testing.T) {
	tests := []struct {
		name     string
		hotkey   string
		expected string
	}{
		{
			name:     "AltGr+comma should work correctly",
			hotkey:   "altgr+comma",
			expected: "<AltGr>comma",
		},
		{
			name:     "Ctrl+Shift+a",
			hotkey:   "ctrl+shift+a",
			expected: "<Ctrl><Shift>a",
		},
		{
			name:     "Simple key without modifiers",
			hotkey:   "a",
			expected: "a",
		},
		{
			name:     "Super key modifier",
			hotkey:   "super+space",
			expected: "<Super>space",
		},
		{
			name:     "Special keys mapping",
			hotkey:   "ctrl+enter",
			expected: "<Ctrl>Return",
		},
		{
			name:     "Multiple modifiers",
			hotkey:   "ctrl+alt+shift+f1",
			expected: "<Ctrl><Alt><Shift>f1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertHotkeyToAccelerator(tt.hotkey)
			if result != tt.expected {
				t.Errorf("convertHotkeyToAccelerator(%s) = %s, want %s",
					tt.hotkey, result, tt.expected)
			}
		})
	}
}

func TestDbusKeyboardProvider_IsSupported(t *testing.T) {

	provider := NewDbusKeyboardProvider(testutils.NewMockLogger())

	// Test that IsSupported doesn't panic
	// The result depends on system environment, so we just check it doesn't crash
	supported := provider.IsSupported()
	t.Logf("DBus GlobalShortcuts supported: %v", supported)
}

func TestDbusProvider_RegisterHotkey(t *testing.T) {

	provider := NewDbusKeyboardProvider(testutils.NewMockLogger())

	// Test callback registration
	callback := func() error {
		return nil
	}

	err := provider.RegisterHotkey("test+key", "test+key", callback)
	if err != nil {
		t.Logf("Expected error in test environment (no D-Bus portal): %v", err)
	}

	// Verify callback was registered even if D-Bus failed
	if _, exists := provider.callbacks["test+key"]; !exists {
		t.Error("Callback should be registered even if D-Bus setup fails")
	}
}
