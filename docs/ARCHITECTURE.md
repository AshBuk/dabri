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
│  audio/     │  hotkeys/   │  stt/       │  output/    │  config/│
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
  AudioService -> Engine -> transcription

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
  // It lives with the other user actions in internal/services/actions.go.

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
- **`ipc.go`**: IPC server initialization and command handlers

### **Service Layer** (`internal/services/`)
- **`interfaces.go`**: Service contracts and ServiceContainer definition
- **`factory.go`**: `NewContainer` builds the components (recorder, stt engine, output, hotkeys, tray, window, notify, websocket), assembles the services and wires tray and window callbacks
- **`actions.go`**: User actions shared by hotkeys, tray, window and IPC (recording toggle, reset to defaults, model switch with rollback, language/output/recorder selection, hotkey rebinding)
- **`audio_service.go`**: Audio recording, speech transcription
- **`ui_service.go`**: System tray, main window, notifications, UI state (fans state/config changes out to both tray and window)
- **`io_service.go`**: Text output, WebSocket server
- **`config_service.go`**: Configuration file operations. A change is saved as a new copy and published only after the save succeeds, so a config returned by `GetConfig` is never mutated. Components that apply settings live (notifications, the stt engine and model manager, the audio, IO and UI services) read through `GetConfig`. The rest keep the startup config.
- **`hotkey_service.go`**: Hotkey registration, reload and capture for rebinding

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

#### Testing
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
- **`providers/evdev_provider.go`**: Direct evdev input handling (fallback, push-to-talk)
- **`manager/provider_fallback.go`**: Fallback logic and hotkey re-registration
- **`providers/dummy_provider.go`**: Dummy provider for testing

#### Provider Selection & Fallback
- **Override** (config): `hotkeys.provider: auto | dbus | evdev` (default: `auto`)
- **System and AppImage**: D-Bus primary → evdev fallback
- **Flatpak**: D-Bus only, the sandbox has no `/dev/input` access
- **Push-to-talk** (`hotkeys.mode: ptt`): evdev when accessible, since the portal does not report key release reliably
- **Failover**: Seamless re-registration on provider switching

### **Speech Recognition** (`stt/`)

#### Engine Components
- **`engine.go`**: `Engine` validates and decodes the WAV file and passes the samples to the backend named by the model's `Engine` field in `internal/constants/stt_models.go`
- **`whisper.go`**: Whisper backend using the whisper.cpp Go bindings
- **`parakeet.go`**: Parakeet backend using libparakeet from whisper.cpp through CGO; it detects the language itself, so the language pickers are disabled for it
- **`engine_stub.go`**: Stub implementation when CGO is disabled
- **`manager/model_manager.go`**: Model lifecycle management
- **`providers/model_path_resolver.go`**: Bundled model path resolution for different environments
- **`stt.go`**: Public facade and type re-exports for external use

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
- `arch/` - PKGBUILD for the AUR
- `fedora/` - RPM spec and SRPM creation for Fedora/RHEL
- `flatpak/` - Vendored Go modules and sandbox wrappers for the Flatpak build

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
- **`io.github.ashbuk.dabri.yml`**: Flatpak manifest, published on [Flathub](https://flathub.org/apps/io.github.ashbuk.dabri)

###  **Testing Infrastructure** (`tests/`)

#### Integration Tests (`tests/integration/`)
- **`integration_test.go`**: Main integration test suite
- **`audio_integration_test.go`**: Audio system integration
- **`goroutine_lifecycle_test.go`**: App lifecycle from a temp config, goroutine leak detection and graceful shutdown
- **`notification_integration_test.go`**: Desktop notification system integration

#### Test Utilities
- **`internal/testutils/mock_logger.go`**: Shared mock logger implementation
- **`tests/mocks/`**: Shared service mocks; they record calls in one ordered log so tests can check cross-service sequencing
- **Build tags**: Tests use `-tags=integration` for selective execution

---

*This architecture documentation is maintained alongside the codebase. Last updated: 2026-10-26*
