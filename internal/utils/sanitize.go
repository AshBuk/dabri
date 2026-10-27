// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package utils

import (
	"regexp"
	"strings"
)

// Bracketed placeholders like [music], [BLANK_AUDIO], etc. (Unicode letters supported)
var tokenPattern = regexp.MustCompile(`(?i)\[[\p{L}0-9_\-]+\]`)

// SanitizeTranscript removes placeholder tokens and normalizes whitespace.
// This package is broadly used across app and stt layers
func SanitizeTranscript(input string) string {
	if input == "" {
		return ""
	}

	cleaned := tokenPattern.ReplaceAllString(input, " ")
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}
