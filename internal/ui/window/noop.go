// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package window

// noopManager is the fallback backend when no GUI is compiled in: every method
// is a no-op so the rest of the app stays backend-agnostic.
type noopManager struct{}

// New returns a window manager. Until the gotk3 backend lands it yields the
// no-op manager regardless of Actions.
func New(_ Actions) Manager { return noopManager{} }

func (noopManager) Run() error      { return nil }
func (noopManager) Quit()           {}
func (noopManager) Available() bool { return false }
func (noopManager) Show()           {}
func (noopManager) Hide()           {}
func (noopManager) Present()        {}
func (noopManager) Toggle()         {}
func (noopManager) SetState(State)  {}
func (noopManager) SetModel(string) {}
func (noopManager) SetOutput(string) {}
