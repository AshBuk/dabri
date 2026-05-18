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
)

// Check if running inside AppImage
func isAppImage() bool {
	return os.Getenv("APPIMAGE") != "" || os.Getenv("APPDIR") != ""
}

// Select the most appropriate hotkey provider based on configuration and environment
func selectProviderForEnvironment(config adapters.HotkeyConfig, environment interfaces.EnvironmentType, logger logger.Logger) interfaces.KeyboardEventProvider {
	// Silence unused parameter warnings (reserved for future use)
	_ = config
	_ = environment

	// Handle an explicit provider override from the configuration
	switch config.GetProvider() {
	case "evdev":
		logger.Info("Hotkeys provider override: evdev")
		return providers.NewEvdevKeyboardProvider(logger)
	case "dbus":
		logger.Info("Hotkeys provider override: dbus")
		return providers.NewDbusKeyboardProvider(logger)
	}
	// Auto-select the provider based on the runtime environment
	if isAppImage() {
		return selectAppImageProvider(logger)
	}
	return selectSystemProvider(logger)
}

// Select the provider for an AppImage environment
func selectAppImageProvider(logger logger.Logger) interfaces.KeyboardEventProvider {
	logger.Info("AppImage detected - checking evdev first for better compatibility")
	// Try evdev first, as it is often more reliable in AppImage contexts
	if evdevProvider := providers.NewEvdevKeyboardProvider(logger); evdevProvider.IsSupported() {
		logger.Info("Using evdev keyboard provider (AppImage mode)")
		return evdevProvider
	}
	logger.Info("evdev not available in AppImage, falling back to D-Bus")
	logger.Info("HOTKEY SETUP: For reliable hotkeys in AppImage, run:")
	logger.Info("  sudo usermod -a -G input $USER")
	logger.Info("  Then reboot or log out/in")
	// Fallback to D-Bus if evdev is not available
	return providers.NewDbusKeyboardProvider(logger)
}

// Select the provider for a standard system environment
func selectSystemProvider(logger logger.Logger) interfaces.KeyboardEventProvider {
	// Try evdev first - more reliable, works if user is in 'input' group
	if evdevProvider := providers.NewEvdevKeyboardProvider(logger); evdevProvider.IsSupported() {
		logger.Info("Using evdev keyboard provider")
		return evdevProvider
	}
	logger.Info("evdev not available (user not in 'input' group?), trying D-Bus...")
	// Fallback to D-Bus GlobalShortcuts portal
	if dbusProvider := providers.NewDbusKeyboardProvider(logger); dbusProvider.IsSupported() {
		logger.Info("Using D-Bus keyboard provider (GNOME/KDE)")
		return dbusProvider
	}

	logger.Info("No hotkey provider available")
	return createFallbackProvider(logger)
}

// Create a dummy provider as a last resort
func createFallbackProvider(logger logger.Logger) interfaces.KeyboardEventProvider {
	logger.Warning("No supported keyboard provider available")
	logger.Info("For hotkeys to work:")
	logger.Info("  - On GNOME/KDE: Ensure D-Bus session is running")
	logger.Info("  - On other DEs: Run with sudo or add user to 'input' group")
	logger.Info("  - Alternative: Use system-wide hotkey tools like sxhkd")
	return providers.NewDummyKeyboardProvider(logger)
}
