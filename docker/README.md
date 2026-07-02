# Docker Development Environment

This directory contains Docker infrastructure for the dabri project, providing isolated development and build environments.

## Architecture

The project uses a **single-service Docker Compose setup** with one `dev` container that handles all development workflows.

### Service: `dev`
- **Image**: `docker/Dockerfile.dev` (Go 1.25 + Debian Bookworm)
- **Purpose**: Full development environment with GUI dependencies
- **Includes**:
  - Go 1.25.3
  - golangci-lint (for code quality)
  - GUI libraries (libayatana-appindicator3, libgtk-3, etc.)
  - Build tools (cmake, gcc, pkg-config)
  - CLI utilities (xsel, wl-clipboard, xdotool, ydotool, ffmpeg)
  - Note: wtype is also supported by the application for Wayland typing (auto-detected if available on target system)
- **Usage**: All `make` commands run inside this container

## Docker Volumes

The Docker Compose setup uses **named volumes** for caching:

### Active Volumes
- **`go-cache`**: Go module cache (`/go/pkg/mod`)
- **`build-cache`**: Build artifacts and whisper.cpp compilation cache (`/app/build`)

### Bind Mounts
- **Source code**: `.` mounted to `/app` (read-write for development)

## Available Make Commands

### 1. Setup & Build Docker Environment
```bash
make docker-build           # Build/rebuild Docker images from Dockerfiles
make docker-up              # Start dev container in background (docker-compose up -d)
make docker-ps              # Show running containers status
```

### 2. Development Environment
```bash
make docker-shell           # Open interactive bash shell in dev container
make docker-logs            # Show and follow container logs (Ctrl+C to exit)
make docker-stop            # Stop containers (without removing them)
make docker-down            # Stop and remove containers (preserves volumes)
```

### 3. Dependencies & Libraries
```bash
make deps                   # Download Go dependencies (go mod download + tidy + verify)
make whisper-libs           # Build whisper.cpp libraries (runs inside dev container)
                           # Output: lib/libwhisper.so, lib/whisper.h, lib/libggml*.so
```

### 4. Code Quality (runs in Docker)
```bash
make fmt                    # Format code with go fmt + goimports (writes changes)
make lint                   # Run golangci-lint with project configuration
```

### 5. Building Application
```bash
make build                  # Build Go binary without systray (CGO_ENABLED=1)
                           # Output: ./dabri
make build-systray          # Build with system tray support (production build)
                           # Output: ./dabri (with -tags systray)
make all                    # Full build: deps + whisper-libs + build-systray
```

### 6. Testing (runs in Docker)
```bash
make test                   # Unit tests with coverage (CGO_ENABLED=1)
                           # Includes whisper.cpp integration
make test-integration       # Integration tests (fast mode, CGO_ENABLED=0)
                           # Skips long-running tests (-short flag)
make test-integration-full  # Full integration tests (CGO_ENABLED=1)
                           # Runs all tests including slow ones
```

### 7. Packaging
```bash
make appimage              # Build AppImage using Docker (Ubuntu 22.04, recommended)
                           # Output: dist/dabri-<version>.AppImage
                           # Includes: binary, libraries, models, dependencies

make appimage-host        # Build AppImage locally without Docker (requires tools on host)
                           # Requires: linuxdeploy, appimagetool installed
```

### 8. CI/CD Pipeline
```bash
make docker-ci              # Run full CI pipeline (simulates GitHub Actions):
                           # 1. Lint (golangci-lint + goimports check)
                           # 2. Test (unit tests with coverage)
                           # 3. Build AppImage package
```

### 9. Cleanup
```bash
make clean                  # Clean local build artifacts (binary, build/, lib/, dist/)
                           # Runs: rm -rf dabri build/ lib/ dist/ + go clean -cache
make docker-clean           # Remove containers and volumes
                           # Runs: docker-compose down -v + docker system prune -f
make docker-clean-all       # Remove everything including Docker images
                           # Runs: docker-compose down -v --rmi all + docker system prune -af
```

## Common Workflows

### 1. Interactive Development
```bash
# Start container in background
make docker-up

# Open shell
make docker-shell

# Inside container:
make build-systray
./dabri --help
make test
```

### 2. Quick Format + Lint + Test
```bash
make fmt
make lint
make test
```

### 3. Build AppImage Package
```bash
# Docker-based (recommended, Ubuntu 22.04)
make appimage

# Output: dist/dabri-<version>.AppImage

# Or locally without Docker (requires tools installed)
make appimage-host
```

### 4. Full CI Pipeline Locally
```bash
# Simulates GitHub Actions CI
make docker-ci
```

## Environment Variables

All Docker containers have CGO environment pre-configured for whisper.cpp:

```bash
CGO_ENABLED=1
C_INCLUDE_PATH=/app/lib
LIBRARY_PATH=/app/lib
CGO_CFLAGS=-I/app/lib
CGO_LDFLAGS=-L/app/lib -lwhisper -lggml-cpu -lggml
LD_LIBRARY_PATH=/app/lib
PKG_CONFIG_PATH=/app/lib
```

### Build-time Variables

#### Whisper.cpp Version Pinning
```bash
# Pin whisper.cpp to specific version (Makefile default: v1.9.1)
make whisper-libs WHISPER_CPP_REF=v1.9.1

# Or set in environment
export WHISPER_CPP_REF=v1.9.1
make whisper-libs
```

#### AppImage Version
```bash
# Set version for AppImage build
make appimage VERSION=v1.2.3
```

## Dockerfiles

### `Dockerfile.dev` (Development)
- **Base**: `golang:1.25-bookworm`
- **Purpose**: Development and testing
- **Size**: ~2GB (includes all dev dependencies)
- **Entrypoint**: `bash` (interactive shell)

### `Dockerfile.appimage` (Packaging)
- **Base**: `ubuntu:22.04`
- **Purpose**: Build AppImage packages
- **Multi-stage**:
  - Stage 1 (`builder`): Builds AppImage with all dependencies
  - Stage 2 (`artifacts`): Exports only the `.AppImage` file
- **Usage**: `make appimage`
- **Output**: `dist/dabri-<version>.AppImage`

## Benefits

1. **No System Dependencies**: No need to install GUI libraries, CGO, or whisper.cpp on host
2. **Consistent Environment**: Same Go version, linter, and tools across all developers
3. **Isolated Builds**: Package builds (AppImage) run in clean Ubuntu 22.04 container
4. **Fast Iteration**: Docker layer caching + named volumes = quick rebuilds
5. **CI Parity**: Local `make docker-ci` mirrors GitHub Actions workflow
