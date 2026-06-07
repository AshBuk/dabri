// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/ipc"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/platform"
	"github.com/AshBuk/dabri/v2/internal/services"
)

// Manages application lifecycle and context
type RuntimeContext struct {
	Ctx        context.Context    // Main application context for cancellation propagation
	Cancel     context.CancelFunc // Function to cancel the context and trigger shutdown
	ShutdownCh chan os.Signal     // Channel receiving OS signals (SIGINT, SIGTERM)
	Logger     logger.Logger      // Application-wide logger instance
}

// Create a new runtime context
func NewRuntimeContext(logger logger.Logger) *RuntimeContext {
	ctx, cancel := context.WithCancel(context.Background())
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	return &RuntimeContext{
		Ctx:        ctx,
		Cancel:     cancel,
		ShutdownCh: shutdownCh,
		Logger:     logger,
	}
}

// Represents the main application and its services
type App struct {
	Services  *services.ServiceContainer // DI Pattern. Container for: Audio, UI, IO, Config, Hotkeys, TempFileManager
	Runtime   *RuntimeContext            // Application lifecycle management (context, cancel, shutdown channel, logger)
	ipcServer *ipc.Server                // Inter-process communication server for CLI commands
}

// NewApp Constructor - creates a new application instance
// Initializes empty service container and runtime context
func NewApp(logger logger.Logger) *App {
	return &App{
		Services: services.NewServiceContainer(),
		Runtime:  NewRuntimeContext(logger),
	}
}

// Initialize all application services and dependencies
// Multi-step initialization: Config → Services → Dependencies
func (a *App) Initialize(configFile string, debug bool) error {
	a.Runtime.Logger.Info("Initializing application...")
	config, err := a.initializeConfig(configFile, debug)
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}
	if err := a.initializeServices(config, configFile); err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}
	a.setupServiceDependencies()
	a.Runtime.Logger.Info("Application initialization complete")
	return nil
}

// Load and validate the application configuration
func (a *App) initializeConfig(configFile string, debug bool) (*config.Config, error) {
	a.Runtime.Logger.Info("Loading configuration from: %s", configFile)
	cfg, err := config.LoadConfig(configFile, a.Runtime.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	// Override the debug flag from the command line if specified
	if debug {
		cfg.General.Debug = true
	}

	a.Runtime.Logger.Info("Configuration loaded successfully")
	return cfg, nil
}

// Factory Pattern - creates and configures all services via factory with DI
func (a *App) initializeServices(cfg *config.Config, cfgFilePath string) error {
	a.Runtime.Logger.Info("Initializing services with dependency injection...")
	environment := platform.DetectEnvironment()
	a.Runtime.Logger.Info("Detected environment: %s", environment)

	// Factory Constructor: NewServiceFactory(config) → *ServiceFactory (stores configuration)
	factory := services.NewServiceFactory(services.ServiceFactoryConfig{
		Ctx:         a.Runtime.Ctx,    // Application context for cancellation propagation
		Logger:      a.Runtime.Logger, // Application-wide logger for all services
		Config:      cfg,              // Loaded YAML configuration
		ConfigFile:  cfgFilePath,      // Path to config file for reloading
		Environment: environment,      // Detected runtime environment (Native/AppImage/etc)
	})
	// Factory Method: factory.CreateServices() → *ServiceContainer (all services ready)
	serviceContainer, err := factory.CreateServices()
	if err != nil {
		return fmt.Errorf("failed to create services: %w", err)
	}

	// Replace empty container with fully initialized services
	a.Services = serviceContainer
	if err := a.setupHotkeyCallbacks(); err != nil {
		a.Runtime.Logger.Warning("Failed to set up hotkey callbacks: %v", err)
	}

	a.Runtime.Logger.Info("Services initialized successfully")
	return nil
}

// setupServiceDependencies is a no-op placeholder for future cross-service wiring
// Note: All dependency injection is handled in FactoryAssembler.Assemble()
func (a *App) setupServiceDependencies() {
	// Wiring done in factory_assembler.go (Step 4: Late wiring)
}

// Connect hotkey manager events to their corresponding handler methods
// Callback Pattern - registers event handlers (SRP: handlers.go implements the logic)
func (a *App) setupHotkeyCallbacks() error {
	if a.Services == nil || a.Services.Hotkeys == nil {
		return fmt.Errorf("services not initialized")
	}

	// Register callback functions defined in handlers.go
	if err := a.Services.Hotkeys.SetupHotkeyCallbacks(
		a.handleStartRecording,             // handlers.go: Start audio recording
		a.handleStopRecordingAndTranscribe, // handlers.go: Stop recording and transcribe
		a.handleShowConfig,                 // handlers.go: Display configuration
		a.handleResetToDefaults,            // handlers.go: Reset settings to defaults
	); err != nil {
		return fmt.Errorf("failed to set up hotkey callbacks: %w", err)
	}
	return nil
}

// Start all application services
// Fail-Safe pattern - non-critical (warning) failures don't stop the application:
// WebSocket server: disabled by default in config
// Hotkey registration: app can work via CLI
func (a *App) startServices() error {
	if a.Services == nil {
		return fmt.Errorf("services not initialized")
	}

	a.Runtime.Logger.Info("Starting application services...")

	if a.Services.IO != nil {
		if err := a.Services.IO.StartWebSocketServer(); err != nil {
			a.Runtime.Logger.Warning("WebSocket server not started: %v", err)
		}
	}

	if a.Services.Hotkeys != nil {
		if err := a.Services.Hotkeys.RegisterHotkeys(); err != nil {
			a.Runtime.Logger.Warning("Hotkeys not registered: %v", err)
		}
	}

	a.Runtime.Logger.Info("Application services started successfully")
	return nil
}

// Start the application and wait for a shutdown signal
// Event Loop - blocks until OS signal or context cancellation
func (a *App) RunAndWait() error {
	a.Runtime.Logger.Info("Starting application...")
	if err := a.startServices(); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}
	if err := a.startIPCServer(); err != nil {
		a.Runtime.Logger.Warning("CLI IPC server not started: %v", err)
	}

	// The window manager owns the main loop: the GTK backend runs gtk.Main on
	// this (main) thread, the no-op backend blocks until Quit. A watcher
	// goroutine stops the loop on an OS signal or context cancellation.
	if a.Services != nil && a.Services.Window != nil {
		go func() {
			a.waitForShutdown()
			a.Services.Window.Quit()
		}()
		if err := a.Services.Window.Run(); err != nil {
			a.Runtime.Logger.Error("UI loop error: %v", err)
		}
		return a.Shutdown()
	}

	// Fallback when no window manager is wired (e.g. a partially built App).
	a.waitForShutdown()
	return a.Shutdown()
}

// waitForShutdown blocks until an OS signal or context cancellation.
func (a *App) waitForShutdown() {
	select {
	case <-a.Runtime.ShutdownCh: // OS signal (Ctrl+C, SIGTERM)
		a.Runtime.Logger.Info("Received shutdown signal")
	case <-a.Runtime.Ctx.Done(): // Programmatic cancellation
		a.Runtime.Logger.Info("Context cancelled")
	}
}

// Graceful Shutdown - ensures clean resource cleanup
// Shutdown sequence: Cancel() → Stop IPC → Shutdown Services
// Each service is responsible for waiting on its own goroutines via WaitGroup
func (a *App) Shutdown() error {
	a.Runtime.Logger.Info("Shutting down application...")
	a.Runtime.Cancel() // Signal all goroutines to stop
	if a.ipcServer != nil {
		a.ipcServer.Stop()
	}
	if a.Services != nil {
		if err := a.Services.Shutdown(); err != nil {
			a.Runtime.Logger.Error("Error during service shutdown: %v", err)
			return err
		}
	}
	a.Runtime.Logger.Info("Application shutdown complete")
	return nil
}

// Getter methods - provide clean accessors for services
// Return interfaces - Dependency Inversion Principle (DIP)
func (a *App) Audio() services.AudioServiceInterface {
	if a.Services != nil {
		return a.Services.Audio
	}
	return nil
}
func (a *App) UI() services.UIServiceInterface {
	if a.Services != nil {
		return a.Services.UI
	}
	return nil
}
func (a *App) IO() services.IOServiceInterface {
	if a.Services != nil {
		return a.Services.IO
	}
	return nil
}
func (a *App) Config() services.ConfigServiceInterface {
	if a.Services != nil {
		return a.Services.Config
	}
	return nil
}
func (a *App) Hotkeys() services.HotkeyServiceInterface {
	if a.Services != nil {
		return a.Services.Hotkeys
	}
	return nil
}
