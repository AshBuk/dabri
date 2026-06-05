// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/assets"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/logger"
	"github.com/AshBuk/dabri/v2/internal/notify"
	"github.com/AshBuk/dabri/v2/internal/tray"
	appversion "github.com/AshBuk/dabri/v2/internal/version"
)

// Coordinates tray icon states and desktop notifications
type UIService struct {
	logger        logger.Logger
	trayManager   tray.Manager
	notifyManager *notify.NotificationManager
	config        *config.Config
}

// Create a new service instance
func NewUIService(
	logger logger.Logger,
	trayManager tray.Manager,
	notifyManager *notify.NotificationManager,
	config *config.Config,
) *UIService {
	return &UIService{
		logger:        logger,
		trayManager:   trayManager,
		notifyManager: notifyManager,
		config:        config,
	}
}

// Update tray icon to reflect current recording state
func (us *UIService) SetRecordingState(isRecording bool) {
	if us.trayManager != nil {
		us.trayManager.SetRecordingState(isRecording)
	}
}

// Display desktop notification with appropriate icon
func (us *UIService) ShowNotification(title, message string) {
	if us.notifyManager != nil {
		// Use appropriate notification method based on message type
		// Use generic sendNotification method
		if err := us.sendNotification(title, message, "dialog-information"); err != nil {
			us.logger.Warning("Failed to show notification: %v", err)
		}
	}
}

// Refresh tray menu items to reflect config changes
func (us *UIService) UpdateSettings(cfg *config.Config) {
	if us.trayManager != nil {
		us.trayManager.UpdateSettings(cfg)
	}
}

// Update visual feedback during active recording
func (us *UIService) UpdateRecordingUI(isRecording bool, level float64) {
	us.SetRecordingState(isRecording)
}

// Display error notification and log for debugging
func (us *UIService) SetError(message string) {
	us.logger.Error("UI Error: %s", message)
	if us.notifyManager != nil {
		if err := us.sendNotification(constants.NotifyError, message, "dialog-error"); err != nil {
			us.logger.Warning("Failed to show error notification: %v", err)
		}
	}
}

// Display success notification for completed operations
func (us *UIService) SetSuccess(message string) {
	us.logger.Info("UI Success: %s", message)
	if us.notifyManager != nil {
		if err := us.sendNotification(constants.NotifySuccess, message, "dialog-ok-apply"); err != nil {
			us.logger.Warning("Failed to show success notification: %v", err)
		}
	}
}

// Write the About HTML to a cache file and open it with the system browser.
// We use the user cache dir rather than os.TempDir(): under Flatpak the latter
// is the sandbox-private /tmp, which the host browser cannot read, so xdg-open
// would silently open nothing. XDG_CACHE_HOME maps to a host-visible path.
func (us *UIService) ShowAboutPage() error {
	us.logger.Info("Showing about page...")
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	path := filepath.Join(cacheDir, "dabri-about.html")
	html := strings.Replace(assets.AboutHTML, "{{VERSION}}", appversion.Version, 1)
	if err := os.WriteFile(path, []byte(html), 0o600); err != nil {
		return fmt.Errorf("failed to write about page: %w", err)
	}
	return us.openWithSystem(path)
}

// Open configuration file with system default editor
func (us *UIService) ShowConfigFile() error {
	us.logger.Info("Showing config file...")
	path, ok := us.getConfigPath()
	if !ok {
		us.logger.Warning("Could not determine config file path")
		return fmt.Errorf("could not determine config file path")
	}
	// Create config file with defaults if it doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		us.logger.Info("Config file does not exist, creating with defaults: %s", path)
		if err := us.createDefaultConfig(path); err != nil {
			us.logger.Error("Failed to create default config: %v", err)
			return fmt.Errorf("failed to create config file: %w", err)
		}
	}
	// Open with system handler using sanitized environment (AppImage-safe)
	if err := us.openWithSystem(path); err != nil {
		us.logger.Info("Config file open failed, trying directory: %v", err)
		// As a fallback, try to open the containing directory
		dir := filepath.Dir(path)
		if err := us.openWithSystem(dir); err != nil {
			us.logger.Error("Failed to open config directory as fallback: %v", err)
			return fmt.Errorf("failed to open config file or directory: %w", err)
		}
	}
	if us.notifyManager != nil {
		if err := us.sendNotification("Configuration", "Config file opened", "preferences-desktop"); err != nil {
			us.logger.Error("Failed to show config notification: %v", err)
		}
	}
	return nil
}

// Create a default config file at the specified path
func (us *UIService) createDefaultConfig(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Use current config if available, otherwise create default
	cfg := us.config
	if cfg == nil {
		cfg = &config.Config{}
		config.SetDefaultConfig(cfg)
	}
	return config.SaveConfig(path, cfg)
}

// Open file with system handler while cleaning AppImage environment variables
// to prevent host application conflicts
func (us *UIService) openWithSystem(target string) error {
	// #nosec G204 -- Safe: xdg-open is a system tool; arguments are not shell-interpreted.
	cmd := exec.Command("xdg-open", target)

	// AppImage environment cleanup for host app compatibility
	filtered := make([]string, 0, len(os.Environ()))
	for _, env := range os.Environ() {
		// Skip AppImage-specific variables that break host apps
		if !strings.HasPrefix(env, "LD_LIBRARY_PATH=") &&
			!strings.HasPrefix(env, "LD_PRELOAD=") &&
			!strings.HasPrefix(env, "APPIMAGE=") &&
			!strings.HasPrefix(env, "APPDIR=") &&
			!strings.HasPrefix(env, "ARGV0=") {
			filtered = append(filtered, env)
		}
	}
	cmd.Env = filtered

	// Start detached and do not block tray event loop
	if err := cmd.Start(); err != nil {
		// Try gio as an alternative opener if available
		if _, lookErr := exec.LookPath("gio"); lookErr == nil {
			alt := exec.Command("gio", "open", target)
			alt.Env = filtered
			if err2 := alt.Start(); err2 == nil {
				_ = alt.Process.Release()
				return nil
			}
		}
		return err
	}
	// avoid keeping child as a zombie if parent exits
	_ = cmd.Process.Release()
	return nil
}

// Locate config file
func (us *UIService) getConfigPath() (string, bool) {
	configPath, err := config.ConfigFilePath()
	if err != nil {
		return "", false
	}
	return configPath, true
}

// Route notification to appropriate handler based on message type
func (us *UIService) sendNotification(title, message, _ string) error {
	if us.notifyManager == nil {
		return fmt.Errorf("notification manager not available")
	}

	// Use appropriate notification method based on title/context
	switch title {
	case constants.NotifyError:
		return us.notifyManager.NotifyError(message)
	case constants.NotifyRecordingStarted:
		return us.notifyManager.NotifyStartRecording()
	case constants.NotifyRecordingStopped:
		return us.notifyManager.NotifyStopRecording()
	case constants.NotifyTranscriptionDone, constants.NotifySuccess:
		return us.notifyManager.NotifyTranscriptionComplete()
	case constants.NotifyConfigReset:
		return us.notifyManager.NotifyConfigurationReset()
	default:
		// Generic notification - show to user
		us.logger.Info("Notification: %s - %s", title, message)
		return us.notifyManager.ShowNotification(title, message)
	}
}

// Clean termination of UI components
func (us *UIService) Shutdown() error {
	if us.trayManager != nil {
		// Ensure tray event loop and systray are stopped to avoid shutdown hangs
		us.trayManager.Stop()
	}
	us.logger.Info("UIService shutdown complete")
	return nil
}
