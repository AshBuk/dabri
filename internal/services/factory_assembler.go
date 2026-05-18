// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"github.com/AshBuk/dabri/v2/internal/notify"
	"github.com/AshBuk/dabri/v2/internal/tray"
	outputInterfaces "github.com/AshBuk/dabri/v2/output/interfaces"
	"github.com/AshBuk/dabri/v2/websocket"
)

// FactoryAssembler assembles low-level components into high-level services
// Stage 2 of Multi-Stage Factory (see factory.go)
//
//	Services         ← Components assembly
//	ConfigService    ← logger + config + configFile (no components!)
//	HotkeyService    ← logger + HotkeyManager
//	AudioService     ← logger + config + Recorder + WhisperEngine + ModelManager + TempFileManager
//	UIService        ← logger + config + TrayManager + NotifyManager
//	IOService        ← logger + config + OutputManager + WebSocketServer
//
// Assembly process:
//  1. Create services from components
//  2. Early wiring  (Config→UI, IO→UI, IO→Config)
//  3. Pack into container
//  4. Late wiring   (Audio→UI, Audio→IO, Audio→Config)
type FactoryAssembler struct {
	factoryConfig ServiceFactoryConfig
}

func NewFactoryAssembler(config ServiceFactoryConfig) *FactoryAssembler {
	return &FactoryAssembler{factoryConfig: config}
}

// Assemble builds the ServiceContainer from components via 4-step process
func (sa *FactoryAssembler) Assemble(components *Components) *ServiceContainer {
	container := NewServiceContainer()

	// Step 1: Create services from components
	configSvc := sa.createConfigService()
	hotkeysSvc := sa.createHotkeyService(components.HotkeyManager)
	audioSvc := sa.createAudioService(components)
	uiSvc := sa.createUIService(components.TrayManager, components.NotifyManager)
	ioSvc := sa.createIOService(components.OutputManager, components.WebSocketServer)

	// Step 2: Early wiring - dependencies needed before container assembly
	configSvc.SetUIService(uiSvc)     // Config → UI (for reload notifications)
	ioSvc.SetUIService(uiSvc)         // IO → UI (for output notifications)
	ioSvc.SetConfigService(configSvc) // IO → Config (for output settings)
	if components.WebSocketServer != nil {
		components.WebSocketServer.SetAudioController(audioSvc)
	}

	// Step 3: Pack services into container
	container.Config = configSvc
	container.Hotkeys = hotkeysSvc
	container.Audio = audioSvc
	container.UI = uiSvc
	container.IO = ioSvc
	container.TempFileManager = components.TempFileManager

	// Step 4: Late wiring - cross-dependencies after container is ready
	audioSvc.SetDependencies(container.UI, container.IO)
	audioSvc.SetConfig(container.Config)

	return container
}

// Builders - construct services from components
//
// createConfigService creates the ConfigService
func (sa *FactoryAssembler) createConfigService() *ConfigService {
	return NewConfigService(
		sa.factoryConfig.Logger,
		sa.factoryConfig.Config,
		sa.factoryConfig.ConfigFile,
	)
}

// createHotkeyService creates the HotkeyService
func (sa *FactoryAssembler) createHotkeyService(hotkeyManager HotkeyManager) *HotkeyService {
	return NewHotkeyService(
		sa.factoryConfig.Logger,
		hotkeyManager,
	)
}

// createAudioService creates the AudioService
func (sa *FactoryAssembler) createAudioService(components *Components) *AudioService {
	return NewAudioService(
		sa.factoryConfig.Logger,
		sa.factoryConfig.Config,
		components.Recorder,
		components.WhisperEngine,
		components.ModelManager,
		components.TempFileManager,
	)
}

// createUIService creates the UIService
func (sa *FactoryAssembler) createUIService(trayManager tray.Manager, notifyManager *notify.NotificationManager) *UIService {
	return NewUIService(
		sa.factoryConfig.Logger,
		trayManager,
		notifyManager,
		sa.factoryConfig.Config,
	)
}

// createIOService creates the IOService
func (sa *FactoryAssembler) createIOService(outputManager outputInterfaces.Outputter, webSocketServer *websocket.WebSocketServer) *IOService {
	return NewIOService(
		sa.factoryConfig.Logger,
		sa.factoryConfig.Config,
		outputManager,
		webSocketServer,
	)
}
