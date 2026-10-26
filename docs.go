// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

// Package dabri provides a high-level overview of the Dabri project.
//
// Dabri is a minimalist, privacy-focused desktop application written in Go
// that converts speech to text offline using local Whisper models.
//
// Dual-mode architecture:
//   - Daemon mode: Background service with system tray integration for GUI usage
//   - CLI mode: Command-line interface for scripting and tiling window managers
//
// Core responsibilities:
//   - Global hotkeys using DBus GlobalShortcuts portal (primary) or evdev (fallback)
//   - Audio recording via arecord/ffmpeg backends
//   - Local transcription using go-whisper (whisper.cpp)
//   - Text output routing: clipboard, active window typing, or combined
//   - X11 and Wayland support with smart tool selection (xdotool, wtype, ydotool)
//   - IPC communication via Unix socket for low-latency CLI operations
//
// Optional WebSocket API:
//   - Real-time speech-to-text API for external clients
//   - Enabled via config: web_server.enabled: true (default: false)
//   - Endpoint: ws://localhost:8080/ws (or /api/v1/ws)
//   - Supports authentication, CORS, and connection limits
//
// Packaging:
//   - AppImage package with first-run configuration and model copy
//   - Flatpak package published on Flathub (io.github.ashbuk.dabri)
//
// Testing strategy:
//   - Unit tests colocated with packages (default go test ./...)
//   - Integration tests in tests/integration (run with -tags=integration)
//
// For more details, see docs/
package dabri
