// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

// Package window provides Dabri's optional main control window — a small panel
// (status, model/output selectors, record button) shown when the system tray is
// unavailable or on first launch. The gotk3 backend is selected via the "gtk"
// build tag, mirroring the tray's "systray" tag; without it a no-op backend
// keeps headless and CI builds GTK-free.
package window

import "context"

// State is the high-level status surfaced by the window indicator.
type State int

const (
	StateReady State = iota
	StateRecording
	StateTranscribing
	StateError
)

// Actions are the callbacks the window invokes on user interaction; the window
// itself holds no business logic.
type Actions struct {
	OnToggleRecording func() error
	OnSelectModel     func(ctx context.Context, modelID string) error
	OnSelectOutput    func(mode string) error
	OnRunInBackground func()
	OnQuit            func()
}

// Manager drives the main window. The backend is chosen at build time (gotk3 or
// no-op). Visibility and update methods are safe to call from any goroutine;
// Run owns the calling thread and blocks until Quit.
type Manager interface {
	Run() error
	Quit()
	Available() bool

	Show()
	Hide()
	Present()
	Toggle()

	SetState(state State)
	SetModel(modelID string)
	SetOutput(mode string)
}
