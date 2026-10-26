// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package constants

// DefaultModelID is the default model used when none is configured
const DefaultModelID = "small-q5_1"

// Inference engines that run the models
const (
	EngineWhisper  = "whisper"  // whisper.cpp
	EngineParakeet = "parakeet" // libparakeet from whisper.cpp; detects the language itself
)

// ModelDef describes an available speech model variant
type ModelDef struct {
	ID       string // Unique identifier (matches config value)
	Engine   string // EngineWhisper or EngineParakeet
	Name     string // Human-readable name for UI
	FileName string // File name on disk
	URL      string // HuggingFace download URL
	MinSize  int64  // Minimum valid file size in bytes
}

// Models contains curated quantized speech models for machines with varying GPU compute power
var Models = []ModelDef{
	{
		ID:       "base-q5_1",
		Engine:   EngineWhisper,
		Name:     "Base (Q5_1)",
		FileName: "ggml-base-q5_1.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base-q5_1.bin",
		MinSize:  50 * 1024 * 1024, // ~57 MB
	},
	{
		ID:       "small-q5_1",
		Engine:   EngineWhisper,
		Name:     "Small (Q5_1)",
		FileName: "small-q5_1.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small-q5_1.bin",
		MinSize:  150 * 1024 * 1024, // ~181 MB
	},
	{
		ID:       "medium-q5_0",
		Engine:   EngineWhisper,
		Name:     "Medium (Q5_0)",
		FileName: "ggml-medium-q5_0.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium-q5_0.bin",
		MinSize:  500 * 1024 * 1024, // ~539 MB
	},
	{
		ID:       "large-v3-turbo-q5_0",
		Engine:   EngineWhisper,
		Name:     "Large v3 Turbo (Q5_0)",
		FileName: "ggml-large-v3-turbo-q5_0.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo-q5_0.bin",
		MinSize:  500 * 1024 * 1024, // ~820 MB
	},
	{
		ID:       "large-v3-q5_0",
		Engine:   EngineWhisper,
		Name:     "Large v3 (Q5_0)",
		FileName: "ggml-large-v3-q5_0.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-q5_0.bin",
		MinSize:  1000 * 1024 * 1024, // ~1.1 GB
	},
	{
		ID:       "parakeet-tdt-0.6b-v3-q8_0",
		Engine:   EngineParakeet,
		Name:     "Parakeet TDT 0.6B v3 (Q8_0)",
		FileName: "ggml-parakeet-tdt-0.6b-v3-q8_0.bin",
		URL:      "https://huggingface.co/ggml-org/parakeet-GGUF/resolve/main/ggml-parakeet-tdt-0.6b-v3-q8_0.bin",
		MinSize:  600 * 1024 * 1024, // ~669 MB
	},
}

// DetectsLanguage reports whether the model ignores the language setting
func (m ModelDef) DetectsLanguage() bool {
	return m.Engine == EngineParakeet
}

// ModelByID returns model definition by its ID, or nil if not found
func ModelByID(id string) *ModelDef {
	for _, m := range Models {
		if m.ID == id {
			return &m
		}
	}
	return nil
}

// ModelByFileName returns model definition by its file name, or nil if not found
func ModelByFileName(name string) *ModelDef {
	for _, m := range Models {
		if m.FileName == name {
			return &m
		}
	}
	return nil
}
