// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package constants

import "testing"

// whisper.cpp picks a CPU variant at runtime, so its models must never be blocked
func TestCPUSupported_WhisperCppModels(t *testing.T) {
	for _, m := range Models {
		if m.Engine != EngineTranscribe && !m.CPUSupported() {
			t.Errorf("%s is blocked on this CPU", m.ID)
		}
	}
}
