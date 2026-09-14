# Dabri Agents Context

## Core Principles
- **Follow idiomatic Go and its best practices**
- **THE FEWER LINES OF CODE THE BETTER**
- **Prefer reusing existing logic, utilities, and components over writing new code**
- Abstract and generalize **only if it need**
- Apply **minimally necessary changes** only
- Preserve existing code style and formatting
- Do not delete existing comments
- **SOLID**, **SRP**, **DRY**, **YAGNI**

## Documentation map (for reference)
- [Architecture](docs/ARCHITECTURE.md) — system design, module boundaries, data flow
- [Audio Pipeline Diagram](docs/AUDIO_PIPELINE_DIAGRAM.txt) — visual pipeline overview
- [CLI Usage](docs/CLI_USAGE.md) — command-line interface reference
- [Contributing](docs/CONTRIBUTING.md) — contribution guidelines
- [Development](docs/DEVELOPMENT.md) — local dev setup, build, debug
- [Desktop Environment Support](docs/Desktop_Environment_Support.md) — DE compatibility notes
- [Stats](docs/STATS.md) — project metrics

## Command Reference

### Build & Dependencies
```bash
# Full build (deps + whisper + binary)
make all

# Build the main binary
make build

# Build with system tray support
make build-systray

# Download and verify Go dependencies
make deps

# Build whisper.cpp libraries
make whisper-libs

# Clean everything
make clean
```

### Quality & Tests
```bash
# Format Go code
make fmt

# Run linter and code quality checks
make lint

# Run security scanner
make gosec

# Run unit tests
make test

# Run race detector tests
make test-race

# Run integration tests
make test-integration

# Run full integration tests with CGO
make test-integration-full
```

### Packaging
```bash
# Build AppImage via Docker
make appimage

# Build AppImage locally on host
make appimage-host
```

### Docker
```bash
# Start dev environment
make docker-up

# Open shell in dev container
make docker-shell

# Stop services
make docker-stop       # Stop without removing volumes
make docker-down       # Stop and remove volumes

# Build Docker images
make docker-build

# Docker utility
make docker-logs       # Show logs
make docker-ps         # Show containers

# Docker cleanup
make docker-clean      # Clean resources
make docker-clean-all  # Clean everything including images

# Run full CI pipeline (lint + test + appimage)
make docker-ci
```

### Utilities
```bash
# Check required host tools
make check-tools

# Show available targets
make help
```

## Project Structure
- `cmd/dabri/` - Main entry point (main.go, daemon.go, commands.go, model.go, environment.go) — cobra-based CLI
- `hotkeys/` - Hotkey handling
- `audio/` - Audio recording
- `whisper/` - Speech recognition
- `output/` - Text output (clipboard/typing)
- `config/` - Configuration management
- `lib/` - Whisper.cpp libraries (built)
- `build/` - Build artifacts

## Configuration
Config location: `~/.config/dabri/config.yaml`

## Dependencies
- Go 1.26.8
- whisper.cpp v1.8.4
- cobra, systray, websockets, yaml, evdev, dbus
- CGO enabled for whisper bindings
