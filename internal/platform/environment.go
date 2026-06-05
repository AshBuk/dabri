// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package platform

import (
	"os"
	"os/exec"

	"github.com/godbus/dbus/v5"
)

// EnvironmentType represents the display server type
type EnvironmentType string

const (
	// EnvironmentX11 represents X11 display server
	EnvironmentX11 EnvironmentType = "X11"
	// EnvironmentWayland represents Wayland display server
	EnvironmentWayland EnvironmentType = "Wayland"
	// EnvironmentUnknown represents unknown display server
	EnvironmentUnknown EnvironmentType = "Unknown"
)

// DetectEnvironment detects the current display server environment
func DetectEnvironment() EnvironmentType {
	// Check for Wayland
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return EnvironmentWayland
	}

	// Check for X11
	if os.Getenv("DISPLAY") != "" {
		return EnvironmentX11
	}

	// If neither is detected, assume unknown
	return EnvironmentUnknown
}

// DetectDesktopEnvironment detects the current desktop environment
func DetectDesktopEnvironment() string {
	// Check XDG_CURRENT_DESKTOP first (most reliable)
	if de := os.Getenv("XDG_CURRENT_DESKTOP"); de != "" {
		return de
	}

	// Fallback to legacy variables
	if de := os.Getenv("DESKTOP_SESSION"); de != "" {
		return de
	}

	return "Unknown"
}

// IsGNOMEWithWayland checks if running GNOME with Wayland
func IsGNOMEWithWayland() bool {
	de := DetectDesktopEnvironment()
	env := DetectEnvironment()

	return (de == "GNOME" || de == "ubuntu:GNOME") && env == EnvironmentWayland
}

// IsGNOME checks if running GNOME desktop environment (any protocol)
func IsGNOME() bool {
	de := DetectDesktopEnvironment()
	return de == "GNOME" || de == "ubuntu:GNOME"
}

// IsFlatpak reports whether the process runs inside a Flatpak sandbox
func IsFlatpak() bool {
	if os.Getenv("FLATPAK_ID") != "" {
		return true
	}
	_, err := os.Stat("/.flatpak-info")
	return err == nil
}

// HasStatusNotifierWatcher checks if a StatusNotifier watcher is present on the session D-Bus
func HasStatusNotifierWatcher() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}

	names := []string{
		"org.kde.StatusNotifierWatcher",
		"org.freedesktop.StatusNotifierWatcher",
	}

	// Query the bus for name ownership
	busObj := conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	for _, name := range names {
		var hasOwner bool
		call := busObj.Call("org.freedesktop.DBus.NameHasOwner", 0, name)
		if call.Err == nil {
			if err := call.Store(&hasOwner); err == nil && hasOwner {
				return true
			}
		}
	}
	return false
}

// UtilityExists checks if a specified command/utility exists in PATH
func UtilityExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// CheckPrivileges checks if the application has the necessary privileges
// to access input devices
func CheckPrivileges() bool {
	// Implementation depends on platform-specific details
	// This is a simple placeholder
	return os.Geteuid() == 0
}

// EnsureDirectoryExists creates a directory if it doesn't exist
func EnsureDirectoryExists(path string) error {
	return os.MkdirAll(path, 0700)
}
