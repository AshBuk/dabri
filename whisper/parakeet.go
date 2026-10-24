//go:build cgo

// Copyright (c) 2026 Asher Buk
// SPDX-License-Identifier: MIT

package whisper

// #cgo LDFLAGS: -lparakeet
// #include <stdlib.h>
// #include <parakeet.h>
import "C"

import (
	"errors"
	"path/filepath"
	"strings"
	"unsafe"
)

// parakeetModel wraps an NVIDIA Parakeet model loaded through whisper.cpp's libparakeet
type parakeetModel struct {
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
	params := C.parakeet_full_default_params(C.PARAKEET_SAMPLING_GREEDY)
	if C.parakeet_full(p.ctx, params, (*C.float)(&samples[0]), C.int(len(samples))) != 0 {
		return "", errors.New("parakeet failed to process audio")
	}
	var transcript strings.Builder
	for i := range int(C.parakeet_full_n_segments(p.ctx)) {
		transcript.WriteString(C.GoString(C.parakeet_full_get_segment_text(p.ctx, C.int(i))))
		transcript.WriteString(" ")
	}
	return transcript.String(), nil
}
