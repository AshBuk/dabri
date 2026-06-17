// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package providers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testMinSize = 1024 // 1 KB for tests

func TestModelDownloader_Download_Success(t *testing.T) {
	// Create mock server with fake model data
	modelData := strings.Repeat("x", testMinSize+1000) // Slightly larger than minimum
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(modelData))
	}))
	defer server.Close()

	// Create downloader with mock URL
	downloader := NewModelDownloaderForURL(server.URL, testMinSize)
	// Download to temp directory
	destPath := filepath.Join(t.TempDir(), "models", "test_model.bin")
	err := downloader.Download(context.Background(), destPath)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	// Verify file exists and has correct size
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Downloaded file not found: %v", err)
	}
	if info.Size() != int64(len(modelData)) {
		t.Errorf("File size mismatch: expected %d, got %d", len(modelData), info.Size())
	}
}

func TestModelDownloader_Download_ReportsProgress(t *testing.T) {
	modelData := strings.Repeat("x", testMinSize+1000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(modelData))
	}))
	defer server.Close()

	var lastDownloaded, lastTotal int64
	calls := 0
	downloader := NewModelDownloaderForURL(server.URL, testMinSize).
		WithProgress(func(downloaded, total int64) {
			calls++
			lastDownloaded, lastTotal = downloaded, total
		})

	destPath := filepath.Join(t.TempDir(), "models", "test_model.bin")
	if err := downloader.Download(context.Background(), destPath); err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	// The final flush always fires, so we expect at least one report ending at 100%.
	if calls == 0 {
		t.Fatal("progress callback was never invoked")
	}
	if lastDownloaded != int64(len(modelData)) {
		t.Errorf("final progress: expected %d bytes, got %d", len(modelData), lastDownloaded)
	}
	if lastTotal != int64(len(modelData)) {
		t.Errorf("expected total %d from Content-Length, got %d", len(modelData), lastTotal)
	}
}

func TestModelDownloader_Download_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	downloader := NewModelDownloaderForURL(server.URL, testMinSize)
	destPath := filepath.Join(t.TempDir(), "test_model.bin")
	err := downloader.Download(context.Background(), destPath)
	if err == nil {
		t.Fatal("Expected error for HTTP 404, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("Error should mention HTTP status: %v", err)
	}
}

func TestModelDownloader_Download_TooSmall(t *testing.T) {
	// Return data smaller than minimum
	smallData := "too small"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(smallData))
	}))
	defer server.Close()

	downloader := NewModelDownloaderForURL(server.URL, testMinSize)
	destPath := filepath.Join(t.TempDir(), "test_model.bin")
	err := downloader.Download(context.Background(), destPath)
	if err == nil {
		t.Fatal("Expected error for too small file, got nil")
	}
	if !strings.Contains(err.Error(), "too small") {
		t.Errorf("Error should mention size: %v", err)
	}
	// Verify temp file was cleaned up
	tmpPath := destPath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("Temporary file should be cleaned up on error")
	}
}

func TestModelDownloader_Download_CreatesDirectories(t *testing.T) {
	modelData := strings.Repeat("x", testMinSize+1000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(modelData))
	}))
	defer server.Close()

	downloader := NewModelDownloaderForURL(server.URL, testMinSize)
	// Use deeply nested path that doesn't exist
	destPath := filepath.Join(t.TempDir(), "a", "b", "c", "model.bin")
	err := downloader.Download(context.Background(), destPath)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	// Verify directories were created
	if _, err := os.Stat(filepath.Dir(destPath)); os.IsNotExist(err) {
		t.Error("Parent directories should be created")
	}
}

func TestModelDownloader_Download_AtomicWrite(t *testing.T) {
	modelData := strings.Repeat("x", testMinSize+1000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(modelData))
	}))
	defer server.Close()

	downloader := NewModelDownloaderForURL(server.URL, testMinSize)
	destPath := filepath.Join(t.TempDir(), "model.bin")
	err := downloader.Download(context.Background(), destPath)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	// Verify no .tmp file remains
	tmpPath := destPath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("Temporary file should not remain after successful download")
	}
}

func TestModelDownloader_GetModelURL(t *testing.T) {
	url := "https://example.com/model.bin"
	downloader := NewModelDownloaderForURL(url, testMinSize)

	if downloader.GetModelURL() != url {
		t.Errorf("GetModelURL() = %q, want %q", downloader.GetModelURL(), url)
	}
}

func TestModelDownloader_Download_NetworkError(t *testing.T) {
	// Use invalid URL to simulate network error
	downloader := NewModelDownloaderForURL("http://invalid.invalid.invalid:99999", testMinSize)
	destPath := filepath.Join(t.TempDir(), "model.bin")

	err := downloader.Download(context.Background(), destPath)
	if err == nil {
		t.Fatal("Expected error for network failure, got nil")
	}
}

func TestModelDownloader_Download_ContextCancelled(t *testing.T) {
	// Server that blocks until request context is done
	requestReceived := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestReceived)
		// Block until client cancels
		<-r.Context().Done()
	}))
	defer server.Close()

	// Clean up default client connection pool to avoid goroutine leaks
	defer http.DefaultClient.CloseIdleConnections()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	downloader := NewModelDownloaderForURL(server.URL, testMinSize)
	destPath := filepath.Join(t.TempDir(), "model.bin")

	// Run download in goroutine and cancel context
	errCh := make(chan error, 1)
	go func() {
		errCh <- downloader.Download(ctx, destPath)
	}()

	// Wait for server to receive request, then cancel
	<-requestReceived
	cancel()

	err := <-errCh
	if err == nil {
		t.Fatal("Expected error from cancelled context, got nil")
	}

	// Verify .tmp file was cleaned up
	tmpPath := destPath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("Temporary file should be cleaned up on cancellation")
	}
}
