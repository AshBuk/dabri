# Dabri Architecture Documentation

The application follows a **dual-mode daemon architecture** with clear separation of concerns:
- **Daemon mode**: Background service with system tray and main window, hotkeys, IPC server
- **CLI mode**: Control commands (start/stop/status/transcript) via Unix socket IPC

```
┌─────────────────────────────────────────────────────────────────┐
│                        Application Layer                        │
├─────────────────────────────────────────────────────────────────┤
│  cmd/dabri/    │  Entry point, CLI parsing, environment   │
└─────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│                        Business Logic                           │
├─────────────────────────────────────────────────────────────────┤
│  internal/app/       │  Core application orchestration          │
└─────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│                       Service Modules                           │
├─────────────────────────────────────────────────────────────────┤
│  audio/     │  hotkeys/   │  whisper/   │  output/    │  config/│
│  websocket/ │                                                   │
└─────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│                      Internal Utilities                         │
├─────────────────────────────────────────────────────────────────┤
│  internal/logger/ │ internal/notify/ │ internal/platform/       │
│  internal/ui/     │ internal/utils/  │ internal/constants/      │
│  internal/services/ │ internal/ipc/                             │
└─────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────┐
│                       System Integration                        │
├─────────────────────────────────────────────────────────────────┤
│  X11/Wayland │ DBus Portals │ System Tray │ Audio System │ FS   │
│  Unix Sockets (IPC)                                             │
└─────────────────────────────────────────────────────────────────┘
```

---

## Service Pipelines

```
// Audio pipeline
  AudioService -> AudioRecorder -> (arecord/ffmpeg)
  AudioService -> TempFileManager -> cleanup logic
  AudioService -> WhisperEngine -> transcription

// Config pipeline  
  ConfigService -> YAMLLoader -> validation
  ConfigService -> StandardValidator -> sanitization

// UI pipeline
  UIService -> TrayManager -> systray integration
  UIService -> WindowManager -> GTK main window (no-op when built without the gtk tag)
  UIService -> NotificationManager -> system notifications

// Hotkey pipeline
  HotkeyService -> HotkeyManager -> (evdev/dbus providers)
  HotkeyService -> ConfigAdapter -> hotkey mapping
  // Start/stop is a single toggle resolved against AudioService.IsRecording(),
  // shared by hotkey, window, tray and CLI (one source of truth, no per-surface state).

// Output pipeline
  IOService -> OutputManager -> (clipboard/typing)
  IOService -> WebSocketServer -> external integrations
  
// Project Pattern
  Service -> Manager -> Provider/Adapter
```

---

## Module Breakdown

### 📁 **Core Application**

#### `cmd/dabri/`
- **Purpose**: Application entry point with dual-mode support (daemon + CLI), powered by [cobra](https://github.com/spf13/cobra)
- **Structure**:
  - `main.go`: Entry point, root cobra.Command definition, version output
  - `daemon.go`: Daemon mode initialization and flag parsing (`--config`, `--debug`)
  - `commands.go`: IPC-based CLI subcommands (start/stop/toggle/status/transcript) via factory pattern
  - `model.go`: Model management command tree (model list/set/delete)
- **Responsibilities**:
  - Dual-mode routing via cobra: root command → daemon, subcommands → IPC client
  - Built-in `--help`, `--version` via cobra (no custom usage/version code)
  - Command-line flag processing (`--config`, `--debug` for daemon; `--socket`, `--json`, `--timeout` for CLI)
  - Environment detection and path adjustment for portable packages
  - IPC client for CLI commands communicating with daemon

#### `internal/app/` - Application Core
- **`app.go`**: Application orchestrator with ServiceContainer and RuntimeContext; the window backend owns the main loop (GTK loop, or blocking no-op)
- **`handlers.go`**: Hotkey handlers that delegate to services; recording is a single start/stop toggle decided from AudioService state
- **`ipc.go`**: IPC server initialization and command handlers

### **Service Layer** (`internal/services/`)
- **`interfaces.go`**: Service contracts and ServiceContainer definition
- **`factory_components.go`**: Components initialization (recorder, whisper, output, hotkeys, tray, window, notify, websocket)
- **`factory_assembler.go`**: Services assembly and cross-dependencies
- **`factory_wirer.go`**: Tray and window callback wiring (shared recording toggle, model/language/output selection)
- **`factory.go`**: Facade delegating to FactoryComponents, FactoryAssembler, FactoryWirer
- **`audio_service.go`**: Audio recording, Whisper transcription
- **`ui_service.go`**: System tray, main window, notifications, UI state (fans state/config changes out to both tray and window)
- **`io_service.go`**: Text output, WebSocket server
- **`config_service.go`**: Configuration file operations
- **`hotkey_service.go`**: Hotkey registration and the recording toggle

Related constants:
- **`internal/constants/ui.go`**: UI icons/messages/titles centralization

### **Audio Processing Module** (`audio/`)

#### Core Audio Components
- **`interfaces/recorder.go`**: AudioRecorder interface definition
- **`recorders/`**: Recorder implementations
  - `base_recorder.go`: Common functionality for all audio recorders
  - `arecord_recorder.go`: ALSA-based recording implementation
  - `ffmpeg_recorder.go`: FFmpeg-based recording implementation
- **`factory/factory.go`**: Factory pattern for creating appropriate recorders

#### Processing (`processing/`)
- **`tempfile_manager.go`**: Temporary audio file lifecycle management

#### Testing & Mocking
- **`mocks/mock_recorder.go`**: Mock implementation for testing
- **`interfaces/interface_validation_test.go`**: Interface compliance tests
- **`*_test.go`**: Comprehensive unit tests for all components

### **Hotkey Management** (`hotkeys/`)

#### Core Hotkey System
- **`interfaces/provider.go`**: KeyboardEventProvider interface and types
- **`manager/manager.go`**: Cross-platform hotkey manager
- **`manager/manager_linux.go`**: Linux-specific hotkey implementation
- **`manager/manager_stub.go`**: Stub implementation for unsupported platforms
- **`adapters/adapter.go`**: Abstraction layer for different providers

#### Provider Implementations
- **`providers/dbus_provider.go`**: DBus GlobalShortcuts portal (preferred for GNOME/KDE), via the [go-wlportal](https://github.com/AshBuk/go-wlportal) shortcuts adapter
- **`providers/evdev_provider.go`**: Direct evdev input handling (primary for AppImage)
- **`manager/provider_fallback.go`**: Fallback logic and hotkey re-registration
- **`providers/dummy_provider.go`**: Dummy provider for testing

#### Provider Selection & Fallback
- **Override** (config): `hotkeys.provider: auto | dbus | evdev` (default: `auto`)
- **System (GNOME/KDE)**: D-Bus primary → evdev fallback
- **AppImage**: evdev primary → D-Bus fallback
- **Failover**: Seamless re-registration on provider switching

### **Speech Recognition** (`whisper/`)

#### Engine Components
- **`engine.go`**: Main Whisper engine using CGO bindings
- **`engine_stub.go`**: Stub implementation when CGO is disabled
- **`manager/model_manager.go`**: Whisper model lifecycle management
- **`providers/model_path_resolver.go`**: Bundled model path resolution for different environments
- **`whisper.go`**: Public facade and type re-exports for external use

### **Output Management** (`output/`)

#### Output Implementations
- **`interfaces/outputter.go`**: Outputter interface definition
- **`factory/factory.go`**: Factory for creating appropriate output handlers with automatic tool selection
- **`outputters/`**: Output implementations
  - `clipboard_outputter.go`: System clipboard integration (wl-copy/wl-paste for Wayland, xsel for X11)
  - `portal_outputter.go`: RemoteDesktop portal typing on GNOME/KDE Wayland (via [go-wlportal](https://github.com/AshBuk/go-wlportal)); sandbox-clean, no extra permissions. Types through the compositor's active keyboard layout; non-ASCII text is copied to the clipboard and auto-pasted with `Shift+Insert` sent through the same portal session, keeping active-window mode.
  - `type_outputter.go`: Active window typing simulation
    - **X11**: Uses `xdotool` (works out-of-the-box)
    - **Wayland (non-GNOME)**: Prefers `wtype` → falls back to `ydotool` if available
    - **Wayland (GNOME)**: Uses `ydotool` → falls back to `wtype` if available
    - Automatic runtime fallback: if primary tool fails, tries alternative typing tool
  - `input_daemon.go`: Owns a `ydotoold` process inside Flatpak on wlroots/Hyprland, where the host daemon's socket is invisible to the sandbox; returns nil (→ clipboard fallback) when uinput is inaccessible
  - `mock_outputter.go`: Mock implementation for testing

### **WebSocket API** (`websocket/`)

- **`server.go`**: WebSocket server for external integrations
- **`message_handler.go`**: WebSocket message processing
- **`authentication.go`**: API authentication and authorization
- **`retry_manager.go`**: Connection retry logic

### **Configuration System** (`config/`)

#### Configuration Management
- **`models/config.go`**: Configuration data structures and constants
- **`loaders/yaml_loader.go`**: YAML configuration loading, parsing and defaults
- **`validators/standard_validator.go`**: Configuration validation and sanitization
- **`security/utils.go`**: Configuration security and integrity checks

### **Internal Utilities** (`internal/`)

#### System Integration
- **`logger/logger.go`**: Structured logging with levels
- **`notify/notification.go`**: Desktop notification system
- **`platform/environment.go`**: Platform detection (X11/Wayland)
- **`ui/`**: User-facing surfaces (grouped under one package)
  - `tray/`: System tray integration
    - `interface.go`: TrayManager interface
    - `tray.go` / `default_systray.go`: systray library integration (built under the `systray` tag)
    - `settings_menu.go`, `icons.go`: menu construction and icon assets
    - `mock_tray.go`: Mock implementation for testing / headless builds
  - `window/`: Optional main control window
    - `window.go`: Manager interface, Options/Actions, shared types
    - `gtk.go`: gotk3 backend (built under the `gtk` tag); owns the GTK main loop
    - `noop.go`: headless fallback that still drives the app lifecycle
- **`utils/`**: General utilities
  - `files.go`: File system utilities
  - `lockfile.go`: Single-instance protection (PID lock file)
  - `ipc_paths.go`: IPC socket path management
  - `disk_linux.go`: Disk space checking (Linux)
  - `disk_stub.go`: Stub implementation
  - `sanitize.go`: Transcript sanitization (token cleanup, whitespace)
  - `async.go`: Goroutine tracking and graceful shutdown coordination
- **`ipc/`**: Inter-process communication (CLI ↔ daemon via Unix sockets)
  - `server.go`: IPC server (socket listener, handler registry)
  - `client.go`: IPC client (request sender with timeout)
  - `types.go`: Request/Response protocol types
- **`testutils/`**: Testing utilities (mock logger)
- **`assets/`**: Embedded resources (about.html)

### **Build & Packaging**

#### Packaging (`packaging/`)
Distribution-specific packaging scripts and configurations:
- `appimage/` - AppImage build scripts
- `fedora/` - RPM spec and SRPM creation for Fedora/RHEL

#### Bash Scripts (`bash-scripts/`)
- **`check-license-headers.sh`**: License compliance validation
- **`dev-env.sh`**: Development environment setup

#### Docker Configuration (`docker/`)
- **`Dockerfile.dev`**: Development environment
- **`Dockerfile.appimage`**: AppImage build environment
- **`docker-compose.yml`**: Multi-service development setup

#### Package Metadata
- **`io.github.ashbuk.dabri.desktop`**: Desktop entry
- **`io.github.ashbuk.dabri.appdata.xml`**: AppStream metadata

###  **Testing Infrastructure** (`tests/`)

#### Integration Tests (`tests/integration/`)
- **`integration_test.go`**: Main integration test suite
- **`end_to_end_test.go`**: Full workflow testing
- **`audio_integration_test.go`**: Audio system integration
- **`platform_integration_test.go`**: Platform-specific testing
- **`goroutine_lifecycle_test.go`**: Goroutine leak detection and graceful shutdown
- **`notification_integration_test.go`**: Desktop notification system integration

#### Test Utilities
- **`internal/testutils/mock_logger.go`**: Shared mock logger implementation
- **`tests/mocks/`**: Shared service mocks for integration-style tests
- **Build tags**: Tests use `-tags=integration` for selective execution

---

*This architecture documentation is maintained alongside the codebase. Last updated: 2026-07-02*
