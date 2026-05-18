// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"context"
	"fmt"

	"github.com/AshBuk/dabri/v2/audio/interfaces"
	"github.com/AshBuk/dabri/v2/audio/processing"
	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/hotkeys/manager"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/notify"
	"github.com/AshBuk/dabri/v2/internal/platform"
	"github.com/AshBuk/dabri/v2/internal/tray"
	outputInterfaces "github.com/AshBuk/dabri/v2/output/interfaces"
	"github.com/AshBuk/dabri/v2/websocket"
	"github.com/AshBuk/dabri/v2/whisper"
)

// ServiceFactoryConfig holds all dependencies needed to create services
type ServiceFactoryConfig struct {
	Ctx         context.Context          // Application context for cancellation propagation
	Logger      logger.Logger            // Application-wide logger
	Config      *config.Config           // Loaded YAML configuration
	ConfigFile  string                   // Path to config file for reloading
	Environment platform.EnvironmentType // Runtime environment (Native/AppImage)
}

// Components holds all initialized low-level application components
// These are created first, then assembled into high-level services
type Components struct {
	ModelManager    whisper.ModelManager        // Whisper model loader and manager
	Recorder        interfaces.AudioRecorder    // Audio recording implementation
	WhisperEngine   *whisper.WhisperEngine      // Speech-to-text engine
	OutputManager   outputInterfaces.Outputter  // Text output (clipboard/typing)
	HotkeyManager   *manager.HotkeyManager      // Global hotkey registration
	WebSocketServer *websocket.WebSocketServer  // WebSocket server for remote control
	TrayManager     tray.Manager                // System tray icon and menu
	NotifyManager   *notify.NotificationManager // Desktop notifications
	TempFileManager *processing.TempFileManager // Temporary audio file management
}

// ServiceFactory creates and configures all services with proper dependency injection
// Separates component creation, assembly, and wiring
type ServiceFactory struct {
	config ServiceFactoryConfig // Factory configuration (logger, config, environment)
}

// NewServiceFactory creates a new service factory with configuration
// Constructor - stores config for later use in CreateServices()
func NewServiceFactory(config ServiceFactoryConfig) *ServiceFactory {
	return &ServiceFactory{
		config: config,
	}
}

// CreateServices creates and configures all services via multi-stage factory
// Multi-Stage Factory Pattern (3 stages):
//  1. FactoryComponents → creates low-level components (Whisper, Audio, Hotkeys, etc)
//  2. FactoryAssembler  → assembles components into high-level services
//  3. FactoryWirer      → wires tray menu callbacks to services
//
// Returns fully configured ServiceContainer ready for use
func (sf *ServiceFactory) CreateServices() (*ServiceContainer, error) {
	// Stage 1: Create components
	compFactory := NewFactoryComponents(sf.config)
	components, err := compFactory.InitializeComponents()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}
	// Stage 2: Assemble services from components
	assembler := NewFactoryAssembler(sf.config)
	container := assembler.Assemble(components)

	// Stage 3: Wire tray menu callbacks
	wirer := NewFactoryWirer(sf.config.Logger)
	wirer.Wire(container, components)

	return container, nil
}
