//go:build cgo

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package stt

// #cgo LDFLAGS: -ldabri-transcribe
// #include <stdlib.h>
// #include <transcribe.h>
// // Takes the enum itself, so Go needs no unsigned-to-int conversion
// static const char *status_string(transcribe_status status) { return transcribe_status_string(status); }
import "C"

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/AshBuk/dabri/v2/internal/logger"
)

// transcribeModel wraps a model loaded through transcribe.cpp, which runs the families whisper.cpp has no support for
type transcribeModel struct {
	mu      sync.Mutex                   // transcribe_run keeps the result on the session, so calls must not overlap; Close waits for them too
	session *C.struct_transcribe_session // nil after Close
}

// silenceTranscribeLog drops the library's stderr output, which includes a line per transcription; errors come back as statuses
var silenceTranscribeLog = sync.OnceFunc(func() { C.transcribe_log_set(nil, nil) })

// newTranscribeModel loads the model on the best GPU device, falling back to CPU, and logs the backend
func newTranscribeModel(modelPath string, log logger.Logger) (*transcribeModel, error) {
	silenceTranscribeLog()
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))
	var params C.struct_transcribe_session_params
	C.transcribe_session_params_init(&params)
	// The library defaults to at most 8 threads
	params.n_threads = C.int(inferenceThreads())
	var session *C.struct_transcribe_session
	if status := C.transcribe_open(cPath, nil, &params, &session); status != C.TRANSCRIBE_OK {
		return nil, fmt.Errorf("failed to load model: %s", C.GoString(C.status_string(status)))
	}
	log.Info("transcribe.cpp %s, backend: %s", C.GoString(C.transcribe_version()), C.GoString(C.transcribe_model_backend(C.transcribe_get_model(session))))
	return &transcribeModel{session: session}, nil
}

// Close releases the session and its model once no transcription is running
func (t *transcribeModel) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	C.transcribe_session_free(t.session) // NULL is a no-op
	t.session = nil
	return nil
}

// transcribe decodes 16 kHz mono samples; the catalog's transcribe.cpp models detect the language themselves
func (t *transcribeModel) transcribe(samples []float32) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.session == nil {
		return "", errors.New("model is closed")
	}
	// gosec needs both bounds and the conversion outside the cgo call; cgo output drops #nosec comments
	n := len(samples)
	if n < 0 || n > math.MaxInt32 {
		return "", errors.New("audio too long for transcribe.cpp")
	}
	nSamples := C.int(n)
	if status := C.transcribe_run(t.session, (*C.float)(&samples[0]), nSamples, nil); status != C.TRANSCRIBE_OK {
		return "", fmt.Errorf("failed to process audio: %s", C.GoString(C.status_string(status)))
	}
	return C.GoString(C.transcribe_full_text(t.session)), nil
}
