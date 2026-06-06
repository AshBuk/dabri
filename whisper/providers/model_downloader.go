// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// httpClient is used for model downloads. It sets connection-establishment and
// response-header timeouts so a stalled server fails fast, but deliberately has
// no total Timeout: model files are large and may legitimately take a while on
// slow links. Total cancellation is handled via the request context.
var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	},
}

// ModelDownloader handles downloading the whisper model from Hugging Face
type ModelDownloader struct {
	url     string
	minSize int64
}

// NewModelDownloaderForURL creates a downloader for the given URL and minimum size
func NewModelDownloaderForURL(url string, minSize int64) *ModelDownloader {
	return &ModelDownloader{url: url, minSize: minSize}
}

// Download downloads the model to the specified path.
// The context allows cancellation of in-progress downloads (e.g. on app shutdown).
// Creates parent directories if they don't exist.
func (d *ModelDownloader) Download(ctx context.Context, destPath string) error {
	// Create parent directories
	dir := filepath.Dir(destPath)
	// #nosec G301 -- Model directory needs to be readable by the application
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create model directory %s: %w", dir, err)
	}

	// Create temporary file for atomic download
	tmpPath := destPath + ".tmp"

	// Download to temporary file
	if err := d.downloadToFile(ctx, tmpPath); err != nil {
		_ = os.Remove(tmpPath) // Clean up on error
		return err
	}

	// Verify download size
	info, err := os.Stat(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to stat downloaded file: %w", err)
	}
	if info.Size() < d.minSize {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("downloaded model is too small (%d bytes), expected at least %d bytes", info.Size(), d.minSize)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to move model to final location: %w", err)
	}
	return nil
}

// downloadToFile downloads the model URL to the specified file
func (d *ModelDownloader) downloadToFile(ctx context.Context, path string) error {
	// Create HTTP request with context for cancellation support
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil) // #nosec G107 -- URL comes from hardcoded model definitions, not user input
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download model: HTTP %d", resp.StatusCode)
	}

	// Create output file
	// #nosec G304 -- path is constructed internally, not from user input
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create model file: %w", err)
	}
	defer func() { _ = out.Close() }()

	// Copy response body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}
	return nil
}

// GetModelURL returns the download URL
func (d *ModelDownloader) GetModelURL() string {
	return d.url
}
