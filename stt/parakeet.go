//go:build cgo

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package stt

// #cgo LDFLAGS: -lparakeet
// #include <stdlib.h>
// #include <parakeet.h>
import "C"

import (
	"errors"
	"io/fs"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/AshBuk/dabri/v2/internal/logger"
)

// parakeetModel wraps an NVIDIA Parakeet model loaded through whisper.cpp's libparakeet
type parakeetModel struct {
	mu  sync.Mutex                 // parakeet_full keeps results in the context's default state, so calls must not overlap; Close waits for them too
	ctx *C.struct_parakeet_context // nil after Close
}

// newParakeetModel loads the model on the first GPU device, falling back to CPU, and logs system info
func newParakeetModel(modelPath string, log logger.Logger) (*parakeetModel, error) {
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))
	ctx := C.parakeet_init_from_file_with_params(cPath, C.parakeet_context_default_params())
	if ctx == nil {
		return nil, errors.New("failed to load parakeet model")
	}
	log.Info("Parakeet system_info: %s", strings.TrimPrefix(C.GoString(C.parakeet_print_system_info()), "PARAKEET : "))
	return &parakeetModel{ctx: ctx}, nil
}

// Close releases the parakeet context once no transcription is running
func (p *parakeetModel) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctx != nil {
		C.parakeet_free(p.ctx)
		p.ctx = nil
	}
	return nil
}

// transcribe decodes 16 kHz mono samples; Parakeet detects the language itself
func (p *parakeetModel) transcribe(samples []float32) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctx == nil {
		return "", errors.New("parakeet model is closed")
	}
	params := C.parakeet_full_default_params(C.PARAKEET_SAMPLING_GREEDY)
	// The library defaults to 4 threads; one per physical core is the fastest on CPU
	if threads := min(physicalCores(), runtime.NumCPU()); threads > 0 && threads <= math.MaxInt32 {
		params.n_threads = C.int(threads)
	}
	// gosec needs both bounds and the conversion outside the cgo call; cgo output drops #nosec comments
	n := len(samples)
	if n < 0 || n > math.MaxInt32 {
		return "", errors.New("audio too long for parakeet")
	}
	nSamples := C.int(n)
	if C.parakeet_full(p.ctx, params, (*C.float)(&samples[0]), nSamples) != 0 {
		return "", errors.New("parakeet failed to process audio")
	}
	var transcript strings.Builder
	for i := range C.parakeet_full_n_segments(p.ctx) {
		transcript.WriteString(C.GoString(C.parakeet_full_get_segment_text(p.ctx, i)))
		transcript.WriteString(" ")
	}
	return transcript.String(), nil
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
