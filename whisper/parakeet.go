//go:build cgo

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package whisper

// #cgo LDFLAGS: -lparakeet
// #include <stdlib.h>
// #include <parakeet.h>
import "C"

import (
	"errors"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

// parakeetModel wraps an NVIDIA Parakeet model loaded through whisper.cpp's libparakeet
type parakeetModel struct {
	mu  sync.Mutex // parakeet_full keeps results in the context's default state, so calls must not overlap
	ctx *C.struct_parakeet_context
}

// isParakeetModel reports whether the file is a Parakeet model, following the ggml-org file naming
func isParakeetModel(modelPath string) bool {
	return strings.HasPrefix(filepath.Base(modelPath), "ggml-parakeet")
}

// newParakeetModel loads the model on the first GPU device, falling back to CPU
func newParakeetModel(modelPath string) (*parakeetModel, error) {
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))
	ctx := C.parakeet_init_from_file_with_params(cPath, C.parakeet_context_default_params())
	if ctx == nil {
		return nil, errors.New("failed to initialize parakeet context")
	}
	return &parakeetModel{ctx: ctx}, nil
}

// Close releases the parakeet context
func (p *parakeetModel) Close() error {
	C.parakeet_free(p.ctx)
	return nil
}

// transcribe decodes 16 kHz mono samples; Parakeet detects the language itself
func (p *parakeetModel) transcribe(samples []float32) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	params := C.parakeet_full_default_params(C.PARAKEET_SAMPLING_GREEDY)
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
