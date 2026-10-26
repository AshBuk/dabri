// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package constants

// DefaultModelID is the default whisper model used when none is configured
const DefaultModelID = "small-q5_1"

// WhisperModelDef describes an available whisper model variant
type WhisperModelDef struct {
	ID       string // Unique identifier (matches config value)
	Name     string // Human-readable name for UI
	FileName string // File name on disk
	URL      string // HuggingFace download URL
	MinSize  int64  // Minimum valid file size in bytes
}

// WhisperModels contains curated quantized whisper models for machines with varying GPU compute power
var WhisperModels = []WhisperModelDef{
	{
		ID:       "base-q5_1",
		Name:     "Base (Q5_1)",
		FileName: "ggml-base-q5_1.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base-q5_1.bin",
		MinSize:  50 * 1024 * 1024, // ~57 MB
	},
	{
		ID:       "small-q5_1",
		Name:     "Small (Q5_1)",
		FileName: "small-q5_1.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small-q5_1.bin",
		MinSize:  150 * 1024 * 1024, // ~181 MB
	},
	{
		ID:       "medium-q5_0",
		Name:     "Medium (Q5_0)",
		FileName: "ggml-medium-q5_0.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium-q5_0.bin",
		MinSize:  500 * 1024 * 1024, // ~539 MB
	},
	{
		ID:       "large-v3-turbo-q5_0",
		Name:     "Large v3 Turbo (Q5_0)",
		FileName: "ggml-large-v3-turbo-q5_0.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo-q5_0.bin",
		MinSize:  500 * 1024 * 1024, // ~820 MB
	},
	{
		ID:       "large-v3-q5_0",
		Name:     "Large v3 (Q5_0)",
		FileName: "ggml-large-v3-q5_0.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-q5_0.bin",
		MinSize:  1000 * 1024 * 1024, // ~1.1 GB
	},
	{
		// Runs through libparakeet, detected by the ggml-parakeet file name prefix; ignores the language setting
		ID:       "parakeet-tdt-0.6b-v3-q8_0",
		Name:     "Parakeet TDT 0.6B v3 (Q8_0)",
		FileName: "ggml-parakeet-tdt-0.6b-v3-q8_0.bin",
		URL:      "https://huggingface.co/ggml-org/parakeet-GGUF/resolve/main/ggml-parakeet-tdt-0.6b-v3-q8_0.bin",
		MinSize:  600 * 1024 * 1024, // ~669 MB
	},
}

// ModelByID returns model definition by its ID, or nil if not found
func ModelByID(id string) *WhisperModelDef {
	for _, m := range WhisperModels {
		if m.ID == id {
			return &m
		}
	}
	return nil
}
