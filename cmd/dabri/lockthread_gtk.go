// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

//go:build gtk

package main

import "runtime"

// GTK requires its main loop to run on the thread that called gtk.Init. Pinning
// the main goroutine to the OS thread here (init runs on it before main) ensures
// gtk.Main, reached later via App.RunAndWait, stays on that thread.
func init() { runtime.LockOSThread() }
