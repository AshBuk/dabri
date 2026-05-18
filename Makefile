# Build targets
.PHONY: all build build-systray deps whisper-libs internal-whisper-libs clean

# Test targets
.PHONY: test test-integration test-integration-full

# Code quality
.PHONY: fmt lint gosec

# Packaging
.PHONY: appimage appimage-host

# Docker targets
.PHONY: docker-build docker-up docker-down docker-stop docker-shell
.PHONY: docker-ci docker-logs docker-ps docker-clean docker-clean-all

# Utility
.PHONY: help

# Variables
GO_VERSION := 1.25.3
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X github.com/AshBuk/dabri/v2/internal/version.Version=$(VERSION)
BINARY_NAME := dabri
BUILD_DIR := build
LIB_DIR := lib
DIST_DIR := dist
# Optional: set to a tag or commit hash to pin whisper.cpp version for reproducible builds
# Example (CI recommended): make WHISPER_CPP_REF=v1.8.4
WHISPER_CPP_REF ?= v1.8.4

# Docker helpers
DOCKER_RUN := docker compose run --rm dev

# ============================================================================
# Formatting & Lint
# ============================================================================

# Format source code (full: go fmt + goimports)
fmt:
	@echo "=== Running go fmt and goimports in Docker ==="
	$(DOCKER_RUN) bash -c 'go fmt ./... && go install golang.org/x/tools/cmd/goimports@latest && goimports -w .'

# Lint source code
lint: deps whisper-libs
	@echo "=== Running linter in Docker ==="
	$(DOCKER_RUN) bash -c 'go build -v github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper && golangci-lint run --verbose --timeout=5m && go install golang.org/x/tools/cmd/goimports@latest && goimports -l . | grep -v "^build/" | tee /dev/stderr | (! read)'

# Security scan with gosec
gosec:
	@echo "=== Running gosec security scanner in Docker ==="
	$(DOCKER_RUN) bash -c 'go install github.com/securego/gosec/v2/cmd/gosec@latest && gosec -fmt=json -out=gosec-report.json ./... || true && gosec ./...'

# -----------------------------------------------------------------------------
# Test targets
# -----------------------------------------------------------------------------

# Run tests via Docker (reuses dev image with CGO + whisper.cpp)
test:
	@echo "=== Running tests via Docker ==="
	$(DOCKER_RUN) go test -v -cover ./...

test-race:
	@echo "=== Running tests with race detector via Docker ==="
	$(DOCKER_RUN) go test -race -count=1 ./...

test-integration:
	@echo "=== Running integration tests (fast mode) via Docker ==="
	docker compose run --rm -e CGO_ENABLED=0 dev go test -tags=integration ./tests/integration/... -short -v

test-integration-full:
	@echo "=== Running full integration tests via Docker ==="
	$(DOCKER_RUN) go test -tags=integration ./tests/integration/... -v

# ============================================================================
# Build & Dependencies
# ============================================================================

# Default target (with systray support for desktop usage)
all: deps whisper-libs build-systray

# Download Go dependencies
deps:
	@echo "=== Downloading Go dependencies (Docker) ==="
	$(DOCKER_RUN) bash -c 'go mod download && go mod tidy && go mod verify'

# Build whisper.cpp libraries (via Docker dev)
whisper-libs:
	@echo "=== Building whisper.cpp libraries in Docker (dev) ==="
	$(DOCKER_RUN) bash -c 'make internal-whisper-libs WHISPER_CPP_REF="$(WHISPER_CPP_REF)"'

# Internal target executed inside the dev container
internal-whisper-libs: $(LIB_DIR)/whisper.h

$(LIB_DIR)/whisper.h:
	@echo "=== Building whisper.cpp libraries ==="
	mkdir -p $(BUILD_DIR)
	cd $(BUILD_DIR) && \
	if [ ! -d "whisper.cpp" ]; then \
		echo "Cloning whisper.cpp..."; \
		git clone https://github.com/ggml-org/whisper.cpp.git; \
	fi
	cd $(BUILD_DIR)/whisper.cpp && \
		if [ -n "$(WHISPER_CPP_REF)" ]; then \
			echo "Checking out whisper.cpp ref $(WHISPER_CPP_REF)"; \
			git fetch --tags; \
			git checkout $(WHISPER_CPP_REF); \
		fi; \
		rm -rf build && \
		cmake -B build -DGGML_VULKAN=ON && \
		cmake --build build --config Release
	mkdir -p $(LIB_DIR)
	cp $(BUILD_DIR)/whisper.cpp/build/src/libwhisper.so* $(LIB_DIR)/ || true
	cp $(BUILD_DIR)/whisper.cpp/build/src/libwhisper.a $(LIB_DIR)/ || true
	cp $(BUILD_DIR)/whisper.cpp/include/whisper.h $(LIB_DIR)/
	cp $(BUILD_DIR)/whisper.cpp/ggml/include/*.h $(LIB_DIR)/ || true
	find $(BUILD_DIR)/whisper.cpp/build/ggml/src -name 'libggml*' \( -name '*.so*' -o -name '*.a' \) -exec cp {} $(LIB_DIR)/ \; 2>/dev/null || true
	@echo "Library files:"
	@ls -la $(LIB_DIR)/

# Build the main binary
build: deps whisper-libs
	@echo "=== Building $(BINARY_NAME) (Docker) ==="
	$(DOCKER_RUN) bash -c 'go build -v -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/dabri && ls -lh $(BINARY_NAME)'

# Build with systray support
build-systray: deps whisper-libs
	@echo "=== Building $(BINARY_NAME) with systray support (Docker) ==="
	$(DOCKER_RUN) bash -c 'go build -tags systray -v -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/dabri && ls -lh $(BINARY_NAME)'

# ============================================================================
# Packaging
# ============================================================================

# Build AppImage (Docker-based, recommended)
appimage:
	@echo "=== Building AppImage via Docker (Ubuntu 22.04) ==="
	docker build -f docker/Dockerfile.appimage --build-arg APP_VERSION=$(VERSION) --build-arg WHISPER_CPP_REF=$(WHISPER_CPP_REF) --target artifacts --output type=local,dest=$(DIST_DIR) .

# Build AppImage locally (without Docker, requires linuxdeploy/appimagetool on host)
appimage-host: build
	@echo "=== Building AppImage locally (host environment) ==="
	@echo "⚠️  Warning: This requires linuxdeploy and appimagetool installed on host"
	bash packaging/appimage/build-appimage.sh

# Clean build artifacts
clean:
	@echo "=== Cleaning build artifacts ==="
	rm -f $(BINARY_NAME)
	@if [ -d "$(LIB_DIR)" ] && [ -n "$$(find $(LIB_DIR) -type f ! -writable 2>/dev/null)" ]; then \
		echo "Removing lib/ and build/ with Docker (files owned by root)..."; \
		docker compose run --rm dev sh -c 'rm -rf $(BUILD_DIR)/* $(LIB_DIR)/* 2>/dev/null || true'; \
		rm -rf $(BUILD_DIR) $(LIB_DIR) 2>/dev/null || true; \
	else \
		rm -rf $(BUILD_DIR) $(LIB_DIR); \
	fi
	rm -rf $(DIST_DIR)
	go clean -cache 2>/dev/null || true
	@echo "Clean completed"

# ============================================================================
# Docker Development Commands
# ============================================================================

# Docker build commands
docker-build:
	@echo "=== Building all Docker images ==="
	docker compose build


# Docker development environment
docker-up:
	@echo "=== Starting Docker development services ==="
	docker compose up -d

docker-stop:
	@echo "=== Stopping Docker development environment ==="
	docker compose down

docker-down:
	@echo "=== Stopping all Docker services ==="
	docker compose down -v --remove-orphans

# Docker CI pipeline
docker-ci:
	@echo "=== Running full CI pipeline in Docker ==="
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) appimage
	@echo "=== CI pipeline completed successfully ==="

# Docker cleanup
docker-clean:
	@echo "=== Cleaning Docker resources ==="
	docker compose down --volumes --remove-orphans
	docker system prune -f

docker-clean-all:
	@echo "=== Cleaning all Docker resources including images ==="
	docker compose down --volumes --remove-orphans --rmi all
	docker system prune -af

# Docker utility commands
docker-logs:
	@echo "=== Showing Docker logs ==="
	docker compose logs -f

docker-ps:
	@echo "=== Showing Docker containers ==="
	docker compose ps

docker-shell:
	@echo "=== Opening shell in development container ==="
	docker compose exec dev bash

# ============================================================================
# Utilities
# ============================================================================

# Check if required tools are available
check-tools:
	@echo "=== Checking required tools ==="
	@command -v go >/dev/null 2>&1 || { echo "Go is required but not installed"; exit 1; }
	@command -v cmake >/dev/null 2>&1 || { echo "CMake is required but not installed"; exit 1; }
	@command -v git >/dev/null 2>&1 || { echo "Git is required but not installed"; exit 1; }
	@echo "All required tools are available"

# ============================================================================
# Help
# ============================================================================

help:
	@echo "Available targets:"
	@echo ""
	@echo "General:"
	@echo "  all                   - Build everything (deps + whisper + binary)"
	@echo "  build                 - Build Go binary only"
	@echo "  build-systray         - Build with systray support"
	@echo "  deps                  - Download Go dependencies"
	@echo "  whisper-libs          - Build whisper.cpp libraries"
	@echo "  fmt                   - Format Go code (go fmt + goimports)"
	@echo "  lint                  - Run linter and code quality checks"
	@echo "  gosec                 - Run security scanner (SAST)"
	@echo "  clean                 - Clean build artifacts"
	@echo ""
	@echo "Tests:"
	@echo "  test                  - Run unit tests"
	@echo "  test-integration      - Run integration tests (fast mode)"
	@echo "  test-integration-full - Run full integration tests (with CGO)"
	@echo ""
	@echo "Packaging:"
	@echo "  appimage              - Build AppImage (Docker-based, recommended)"
	@echo "  appimage-host         - Build AppImage locally (requires tools on host)"
	@echo ""
	@echo "Docker:"
	@echo "  docker-up             - Start development services (docker compose up -d)"
	@echo "  docker-down           - Stop all services (docker compose down)"
	@echo "  docker-stop           - Stop development environment"
	@echo "  docker-shell          - Open shell in dev container"
	@echo "  docker-build          - Build all Docker images"
	@echo "  docker-ci             - Run full CI pipeline (lint + test + appimage)"
	@echo "  docker-logs           - Show Docker logs"
	@echo "  docker-ps             - Show Docker containers"
	@echo "  docker-clean          - Clean Docker resources"
	@echo "  docker-clean-all      - Clean everything including images"
