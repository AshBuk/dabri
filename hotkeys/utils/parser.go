// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"strings"

	"github.com/AshBuk/dabri/v2/hotkeys/interfaces"
)

// Parse a hotkey string into a KeyCombination struct
// Treat the last part as the main key and all preceding parts as modifiers
func ParseHotkey(hotkeyStr string) interfaces.KeyCombination {
	combo := interfaces.KeyCombination{}
	parts := strings.Split(hotkeyStr, "+")
	// If there is only one part, it is a key with no modifiers
	if len(parts) == 1 {
		combo.Key = strings.TrimSpace(parts[0])
		return combo
	}
	// The last part is the key; the rest are modifiers
	combo.Key = strings.TrimSpace(parts[len(parts)-1])
	for i := 0; i < len(parts)-1; i++ {
		modifier := strings.ToLower(strings.TrimSpace(parts[i]))
		combo.Modifiers = append(combo.Modifiers, modifier)
	}
	return combo
}

// Check if a key name represents a modifier key
func IsModifier(keyName string) bool {
	modifiers := map[string]bool{
		"ctrl":       true,
		"alt":        true,
		"shift":      true,
		"super":      true,
		"meta":       true,
		"win":        true,
		"leftmeta":   true,
		"rightmeta":  true,
		"altgr":      true, // AltGr modifier for international keyboards
		"hyper":      true, // Hyper modifier
		"leftctrl":   true,
		"rightctrl":  true,
		"leftalt":    true,
		"rightalt":   true,
		"leftshift":  true,
		"rightshift": true,
	}
	return modifiers[strings.ToLower(keyName)]
}

// Convert a common modifier name to its evdev key name equivalent
func ConvertModifierToEvdev(modifier string) string {
	modifierMap := map[string]string{
		"ctrl":  "leftctrl",
		"alt":   "leftalt",
		"shift": "leftshift",
		"super": "leftmeta",
		"meta":  "leftmeta",
		"win":   "leftmeta",
		"altgr": "rightalt",
	}

	if evdevName, ok := modifierMap[strings.ToLower(modifier)]; ok {
		return evdevName
	}

	return strings.ToLower(modifier)
}

// Normalize a hotkey string to its canonical form
// This involves making it lowercase, trimming whitespace, ordering modifiers,
// and using canonical names for modifiers
func NormalizeHotkey(hotkeyStr string) string {
	s := strings.ToLower(strings.TrimSpace(hotkeyStr))
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "+")
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}

	key := cleaned[len(cleaned)-1]
	mods := cleaned[:len(cleaned)-1]

	// Map modifier synonyms to their canonical names
	mapMod := func(m string) string {
		switch m {
		case "win", "meta", "leftmeta", "rightmeta":
			return "super"
		case "rightalt", "altgr":
			return "altgr"
		case "leftalt":
			return "alt"
		case "leftctrl", "rightctrl":
			return "ctrl"
		case "leftshift", "rightshift":
			return "shift"
		default:
			return m
		}
	}

	seen := map[string]bool{}
	for i := range mods {
		mods[i] = mapMod(mods[i])
		if !seen[mods[i]] {
			seen[mods[i]] = true
		}
	}

	order := []string{"ctrl", "shift", "alt", "altgr", "super"}
	ordered := make([]string, 0, len(order))
	for _, o := range order {
		if seen[o] {
			ordered = append(ordered, o)
		}
	}

	if len(ordered) == 0 {
		return key
	}
	return strings.Join(ordered, "+") + "+" + key
}

// Perform basic validation on a hotkey string
// Return an error if the string is empty or if the main key is a modifier
func ValidateHotkey(hotkeyStr string) error {
	s := NormalizeHotkey(hotkeyStr)
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("empty hotkey")
	}
	combo := ParseHotkey(s)
	if strings.TrimSpace(combo.Key) == "" {
		return fmt.Errorf("missing key")
	}
	if IsModifier(combo.Key) {
		return fmt.Errorf("invalid hotkey: key cannot be a modifier")
	}
	return nil
}
