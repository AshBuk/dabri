// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package security

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/AshBuk/dabri/v2/config/models"
)

// TestVerifyConfigIntegrity tests config integrity verification
func TestVerifyConfigIntegrity(t *testing.T) {
	// Create a temporary config file
	tempFile, err := os.CreateTemp("", "security_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	testContent := "test: value\nother: setting\n"
	if _, err := tempFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tempFile.Close()

	// Calculate correct hash
	correctHash, err := CalculateFileHash(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to calculate hash: %v", err)
	}

	tests := []struct {
		name          string
		config        *models.Config
		expectedError bool
		description   string
	}{
		{
			name: "integrity check disabled",
			config: func() *models.Config {
				c := &models.Config{}
				c.Security.CheckIntegrity = false
				c.Security.ConfigHash = "any_hash"
				return c
			}(),
			expectedError: false,
			description:   "Should pass when integrity check is disabled",
		},
		{
			name: "no hash set",
			config: func() *models.Config {
				c := &models.Config{}
				c.Security.CheckIntegrity = true
				c.Security.ConfigHash = ""
				return c
			}(),
			expectedError: false,
			description:   "Should pass when no hash is set",
		},
		{
			name: "correct hash",
			config: func() *models.Config {
				c := &models.Config{}
				c.Security.CheckIntegrity = true
				c.Security.ConfigHash = correctHash
				return c
			}(),
			expectedError: false,
			description:   "Should pass with correct hash",
		},
		{
			name: "incorrect hash",
			config: func() *models.Config {
				c := &models.Config{}
				c.Security.CheckIntegrity = true
				c.Security.ConfigHash = "incorrect_hash"
				return c
			}(),
			expectedError: true,
			description:   "Should fail with incorrect hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyConfigIntegrity(tempFile.Name(), tt.config)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none: %s", tt.description)
			}

			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v (%s)", err, tt.description)
			}

			if tt.expectedError && err != nil {
				if !strings.Contains(err.Error(), "integrity check failed") {
					t.Errorf("Expected integrity check error, got: %v", err)
				}
			}
		})
	}
}

// TestUpdateConfigHash tests hash updating functionality
func TestUpdateConfigHash(t *testing.T) {
	// Create a temporary config file
	tempFile, err := os.CreateTemp("", "hash_update_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	testContent := "test: value\nother: setting\n"
	if _, err := tempFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tempFile.Close()

	config := &models.Config{}
	config.Security.CheckIntegrity = true
	config.Security.ConfigHash = ""

	// Update hash
	err = UpdateConfigHash(tempFile.Name(), config)
	if err != nil {
		t.Fatalf("Failed to update config hash: %v", err)
	}

	// Verify hash was set
	if config.Security.ConfigHash == "" {
		t.Error("Expected config hash to be set")
	}

	// Verify hash is correct
	expectedHash, err := CalculateFileHash(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to calculate expected hash: %v", err)
	}

	if config.Security.ConfigHash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, config.Security.ConfigHash)
	}

	// Verify the hash is actually valid for integrity check
	err = VerifyConfigIntegrity(tempFile.Name(), config)
	if err != nil {
		t.Errorf("Integrity check failed after hash update: %v", err)
	}
}

// TestCalculateFileHash tests file hash calculation
func TestCalculateFileHash(t *testing.T) {
	// Create test file with known content
	tempFile, err := os.CreateTemp("", "hash_calc_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	testContent := "Hello, World!"
	if _, err := tempFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tempFile.Close()

	// Calculate hash
	hash, err := CalculateFileHash(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to calculate hash: %v", err)
	}

	// Verify hash format (should be 64 character hex string)
	if len(hash) != 64 {
		t.Errorf("Expected hash length 64, got %d", len(hash))
	}

	// Verify it's valid hex
	if _, err := hex.DecodeString(hash); err != nil {
		t.Errorf("Hash is not valid hex: %v", err)
	}

	// Calculate expected hash manually
	hasher := sha256.New()
	hasher.Write([]byte(testContent))
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	if hash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, hash)
	}
}

// TestEnforceFileSizeLimit tests file size limit enforcement
func TestEnforceFileSizeLimit(t *testing.T) {
	// Create test file with known size
	tempFile, err := os.CreateTemp("", "size_limit_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	testContent := strings.Repeat("A", 1000) // 1000 bytes
	if _, err := tempFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tempFile.Close()

	tests := []struct {
		name          string
		maxSize       int64
		expectedError bool
		description   string
	}{
		{
			name:          "file within limit",
			maxSize:       2000,
			expectedError: false,
			description:   "Should pass when file is within size limit",
		},
		{
			name:          "file at exact limit",
			maxSize:       1000,
			expectedError: false,
			description:   "Should pass when file is at exact size limit",
		},
		{
			name:          "file exceeds limit",
			maxSize:       500,
			expectedError: true,
			description:   "Should fail when file exceeds size limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &models.Config{}
			config.Security.MaxTempFileSize = tt.maxSize

			err := EnforceFileSizeLimit(tempFile.Name(), config)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none: %s", tt.description)
			}

			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v (%s)", err, tt.description)
			}
		})
	}
}
