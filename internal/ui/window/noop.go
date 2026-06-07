// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

//go:build !gtk

package window

import (
	"sync"

	"github.com/AshBuk/dabri/v2/internal/logger"
)

// noopManager is the fallback backend when no GUI is compiled in: the window is
// absent, but Run still acts as the application's main loop so the lifecycle is
// identical to the GTK build.
type noopManager struct {
	stop     chan struct{}
	stopOnce sync.Once
}

// New returns the no-op window manager. Options and logger are unused.
func New(_ logger.Logger, _ Options) Manager {
	return &noopManager{stop: make(chan struct{})}
}

func (m *noopManager) Run() error { <-m.stop; return nil }
func (m *noopManager) Quit()      { m.stopOnce.Do(func() { close(m.stop) }) }

func (m *noopManager) Available() bool      { return false }
func (m *noopManager) SetActions(_ Actions) {}
func (m *noopManager) Show()                {}
func (m *noopManager) Hide()                {}
func (m *noopManager) Present()             {}
func (m *noopManager) Toggle()              {}
func (m *noopManager) SetState(State)       {}
func (m *noopManager) SetModel(string)      {}
func (m *noopManager) SetOutput(string)     {}
