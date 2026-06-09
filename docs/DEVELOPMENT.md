# Development Guide

Concise commands for local development and CI-compatible builds.

## Build Architecture

**Docker First Approach:** All builds run in Docker by default for consistency and reproducibility.
- Consistent environment (Ubuntu 22.04 for AppImage compatibility)
- No system dependencies required on host
- Identical builds locally and in CI/CD
- Isolated whisper.cpp compilation

```
Makefile → packaging/ → docker/ → CI/CD
   ↑           ↑            ↑        ↑
simple     orchestration containers production
command    dependencies
```

### 1. Makefile - Simple Commands via Docker
Entry point for developers (all commands run inside Docker dev container):
```bash
make all                   # Build everything (deps + whisper + binary)
make build                 # Build binary with whisper.cpp integration
make build-systray         # Build with system tray support (production)
make test                  # Run unit tests (CGO + whisper.cpp)
make test-race             # Run unit tests with the Go race detector
make test-integration      # Run integration tests (fast mode, no CGO)
make test-integration-full # Run full integration tests (with audio/CGO)
make deps                  # Download Go dependencies
make whisper-libs          # Build whisper.cpp libraries into ./lib
make appimage              # Build AppImage (Docker-based, recommended)
make appimage-host         # Build AppImage on host (requires tools installed)
make clean                 # Clean build artifacts
make fmt                   # Format Go code (go fmt + goimports)
make lint                  # Run linter and code quality checks
make gosec                 # Run security scanner (SAST)
make check-tools           # Verify required tools (local check)
```

### 2. Packaging Scripts
Handle complex build logic and package creation:
```bash
packaging/
├── appimage/
│   ├── AppRun                    # AppImage entrypoint
│   └── build-appimage.sh         # AppImage creation with linuxdeploy
├── arch/
│   └── PKGBUILD                  # Arch Linux AUR package
└── fedora/
    ├── dabri.spec          # RPM spec for Fedora/RHEL
    ├── create-srpm.sh            # SRPM creation script
    └── .rpmlintrc                # rpmlint configuration

bash-scripts/dev-env.sh           # CGO environment configuration
```

### 3. Docker - Containers
Reproducible builds across different environments (single dev service):
```bash
make docker-build      # Build/rebuild Docker images
make docker-up         # Start dev container in background
make docker-stop       # Stop containers (preserve state)
make docker-down       # Stop and remove containers (keep volumes)
make docker-shell      # Open interactive shell in dev container
make docker-ci         # Run full CI pipeline (lint + test + appimage)
make docker-logs       # Show container logs
make docker-ps         # Show running containers
make docker-clean      # Clean containers and volumes
make docker-clean-all  # Clean everything including images
```

### 4. CI/CD - Production
GitHub Actions handle complex builds, releases, and distribution:
```
.github/workflows/
├── ci.yml                  # Main CI (format, lint, gosec, test)
├── lint-arch.yml           # PKGBUILD validation (namcap)
├── lint-rpm.yml            # Spec validation (rpmlint)
├── pre-release.yml         # Pre-release validation
├── release-appimage.yml    # AppImage build & GitHub Release
├── release-arch.yml        # AUR publish (on tag)
└── release-fedora.yml      # COPR submit (on tag)
```

### Test Types & Current Coverage 

| Test Type | Command | Duration | Dependencies | Use Case |
|-----------|---------|----------|--------------|----------|
| **Unit Tests** | `make test` | ~3-8s | Docker dev image | Core functionality |
| **Race Tests** | `make test-race` | slower | Docker dev image | Concurrency checks |
| **Integration Fast** | `make test-integration` | ~2-5s | Docker dev image | Development, CI |
| **Integration Full** | `make test-integration-full` | ~8-20s | Docker dev image + audio | QA, Production |


| Module | Test Coverage | Test Files | Integration Tests |
|--------|---------------|------------|-------------------|
| **audio** | Comprehensive | `factory/`, `recorders/`, `interfaces/` | Audio devices, recording |
| **output** | Comprehensive | `factory/`, `outputters/` | Clipboard, typing tools |
| **hotkeys** | Comprehensive | `manager/`, `providers/`, `adapters/` | D-Bus, evdev fallback providers |
| **config** | Comprehensive | `loaders/`, `validators/`, `security/` | File loading, validation |
| **internal/** | Broad | `services/`, `platform/`, `utils` | Platform detection, service contracts |

## Hotkeys Architecture

### Provider Selection & Fallback Strategy ([manager_linux.go](../hotkeys/manager/manager_linux.go))
```
System (GNOME/KDE):  DBus (primary) → Evdev (fallback)
                       ↑                 ↑
                    (portal)       (direct input)

AppImage:            Evdev (primary) → DBus (fallback)
                       ↑                 ↑
                 (direct input)       (portal)
```

### Module Structure
```
hotkeys/
├── adapters/            # Hotkey adapters and utilities
├── interfaces/          # Provider interface definitions  
├── manager/             # Manager with fallback logic
│   ├── manager.go       # Main orchestrator
│   └── provider_fallback.go # Fallback registration
├── providers/           # Hotkey providers implementation
│   ├── dbus_provider.go # GlobalShortcuts portal (GNOME/KDE)
│   ├── evdev_provider.go # Direct input devices (i3/XFCE/AppImage)
│   └── dummy_provider.go # Testing provider
├── utils/               # Parser utilities
└── hotkeys.go          # Main entry point
```

### Configuration Reference

See [`config/loaders/default_config.yaml`](../config/loaders/default_config.yaml) for the complete configuration file with all available options and detailed comments. It is embedded in the binary and written to `~/.config/dabri/config.yaml` on first run.

*Last updated: 2026-06-10*
