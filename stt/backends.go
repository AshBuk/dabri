//go:build cgo

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package stt

// // Declared from ggml-backend.h; libggml is already linked by the whisper.cpp Go bindings
// void ggml_backend_load_all(void);
import "C"

import "sync"

// loadBackends registers ggml backends built as runtime modules (GGML_BACKEND_DL),
// picking the best CPU variant for this machine. A no-op when backends are linked statically
var loadBackends = sync.OnceFunc(func() { C.ggml_backend_load_all() })
