//go:build linux

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package manager

import (
	"os"

	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/hotkeys/interfaces"
	"github.com/AshBuk/dabri/v2/hotkeys/providers"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/platform"
)

// Check if running inside AppImage
func isAppImage() bool {
	return os.Getenv("APPIMAGE") != "" || os.Getenv("APPDIR") != ""
}

// Check if running under Hyprland
func isHyprland() bool {
	return os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != ""
}

// Select the most appropriate hotkey provider based on configuration and environment.
// Provider selection order (unless overridden in config):
//  1. D-Bus GlobalShortcuts portal (Wayland-native, no extra permissions)
//  2. evdev (X11-era direct access, requires input group or udev rule)
//  3. Dummy provider (hotkeys disabled, logs instructions)
//
// Override via config: hotkeys.provider = "dbus" | "evdev"
// Any other value (including empty) triggers auto-selection.
func selectProviderForEnvironment(config adapters.HotkeyConfig, environment interfaces.EnvironmentType, logger logger.Logger) interfaces.KeyboardEventProvider {
	_ = environment // reserved for future environment-specific logic

	switch config.GetProvider() {
	case "evdev":
		logger.Info("Hotkeys provider override: evdev")
		return providers.NewEvdevKeyboardProvider(logger)
	case "dbus":
		logger.Info("Hotkeys provider override: dbus")
		return providers.NewDbusKeyboardProvider(logger)
	}
	// Auto-select the provider based on the runtime environment
	if platform.IsFlatpak() {
		logger.Info("Flatpak detected - using D-Bus keyboard provider")
		return providers.NewDbusKeyboardProvider(logger)
	}
	if isAppImage() {
		return selectAppImageProvider(logger)
	}
	return selectSystemProvider(logger)
}

// Select the provider for an AppImage environment
func selectAppImageProvider(logger logger.Logger) interfaces.KeyboardEventProvider {
	// D-Bus GlobalShortcuts portal requires no special permissions
	if dbusProvider := providers.NewDbusKeyboardProvider(logger); dbusProvider.IsSupported() {
		logger.Info("Using D-Bus keyboard provider (AppImage mode)")
		if isHyprland() {
			logger.Warning("Hyprland detected: hotkeys require manual binding in hyprland.conf — see docs/Desktop_Environment_Support.md")
		}
		return dbusProvider
	}
	logger.Info("D-Bus not available in AppImage, trying evdev...")
	if evdevProvider := providers.NewEvdevKeyboardProvider(logger); evdevProvider.IsSupported() {
		logger.Info("Using evdev keyboard provider (AppImage mode)")
		return evdevProvider
	}
	return createFallbackProvider(logger)
}

// Select the provider for a standard system environment
func selectSystemProvider(logger logger.Logger) interfaces.KeyboardEventProvider {
	// D-Bus GlobalShortcuts portal requires no special permissions
	if dbusProvider := providers.NewDbusKeyboardProvider(logger); dbusProvider.IsSupported() {
		logger.Info("Using D-Bus keyboard provider")
		if isHyprland() {
			logger.Warning("Hyprland detected: hotkeys require manual binding in hyprland.conf — see docs/Desktop_Environment_Support.md")
		}
		return dbusProvider
	}
	logger.Info("D-Bus not available, trying evdev...")
	if evdevProvider := providers.NewEvdevKeyboardProvider(logger); evdevProvider.IsSupported() {
		logger.Info("Using evdev keyboard provider")
		return evdevProvider
	}

	logger.Info("No hotkey provider available")
	return createFallbackProvider(logger)
}

// Create a dummy provider as a last resort
func createFallbackProvider(logger logger.Logger) interfaces.KeyboardEventProvider {
	logger.Warning("No hotkey provider available — hotkeys will be disabled")
	logger.Info("To enable hotkeys:")
	logger.Info("  - GNOME/KDE: ensure xdg-desktop-portal is running")
	logger.Info("  - Minimal WMs (i3, bspwm, etc.): enable evdev via 'provider: evdev' in config")
	return providers.NewDummyKeyboardProvider(logger)
}
