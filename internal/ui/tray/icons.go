// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package tray

import _ "embed"

//go:embed resources/icon.png
var iconDefault []byte

//go:embed resources/icon-recording.png
var iconRecording []byte

// IconDefault returns the icon for the idle state.
func IconDefault() []byte { return iconDefault }

// IconRecording returns the icon for the recording state.
func IconRecording() []byte { return iconRecording }
