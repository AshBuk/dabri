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

// ModelChoice is one selectable whisper model: a stable ID and a display name.
type ModelChoice struct {
	ID   string
	Name string
}

// Options is the initial display state passed to a backend at construction.
type Options struct {
	HasTray      bool          // whether a system tray is available
	StartVisible bool          // show the window as soon as the loop starts
	Models       []ModelChoice // selectable models
	ActiveModel  string        // active model ID
	OutputMode   string        // active output mode (config value)
	Hotkey       string        // start-recording hotkey, for display only
}

// Actions are the callbacks the window invokes on user interaction; the window
// itself holds no business logic. They are wired after construction via
// SetActions, mirroring the tray's callback setters.
type Actions struct {
	OnToggleRecording func() error
	OnSelectModel     func(ctx context.Context, modelID string) error
	OnSelectOutput    func(mode string) error
	OnQuit            func()
}

// Manager drives the main window. The backend is chosen at build time (gotk3 or
// no-op). Visibility and update methods are safe to call from any goroutine;
// Run owns the calling thread and blocks until Quit.
type Manager interface {
	Run() error
	Quit()
	Available() bool

	SetActions(actions Actions)

	Show()
	Hide()
	Present()
	Toggle()

	SetState(state State)
	SetModel(modelID string)
	SetOutput(mode string)
}
