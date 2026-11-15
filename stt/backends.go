//go:build cgo

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package stt

// // Declared from ggml-backend.h; libggml is already linked by the whisper.cpp Go bindings
// void ggml_backend_load_all(void);
import "C"

import (
	"io/fs"
	"math"
	"os"
	"runtime"
	"sync"
)

// loadBackends registers ggml backends built as runtime modules (GGML_BACKEND_DL),
// picking the best CPU variant for this machine. A no-op when backends are linked statically
var loadBackends = sync.OnceFunc(func() { C.ggml_backend_load_all() })

// inferenceThreads returns one thread per physical core, capped by the CPUs this process may use; 0 if unknown
func inferenceThreads() int32 {
	if threads := min(physicalCores(), runtime.NumCPU()); threads > 0 && threads <= math.MaxInt32 {
		return int32(threads)
	}
	return 0
}

// physicalCores counts CPU cores without their SMT siblings, which slow ggml down; 0 if sysfs has no topology
func physicalCores() int {
	sys := os.DirFS("/sys/devices/system/cpu")
	siblings, _ := fs.Glob(sys, "cpu[0-9]*/topology/thread_siblings_list")
	cores := map[string]struct{}{}
	for _, name := range siblings {
		if list, err := fs.ReadFile(sys, name); err == nil {
			cores[string(list)] = struct{}{}
		}
	}
	return len(cores)
}
