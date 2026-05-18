// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package notify

import (
	"fmt"
	"os/exec"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/constants"
)

// Manages the sending of desktop notifications
type NotificationManager struct {
	appName string
	config  *config.Config
}

// Create a new notification manager
func NewNotificationManager(appName string, cfg *config.Config) *NotificationManager {
	return &NotificationManager{
		appName: appName,
		config:  cfg,
	}
}

// Show a notification when recording starts
func (nm *NotificationManager) NotifyStartRecording() error {
	if !nm.config.Notifications.EnableWorkflowNotifications {
		return nil // Skip if workflow notifications are disabled
	}
	return nm.sendNotification(constants.NotifyTitleRecordingStart, constants.NotifyRecordingStartMsg, "microphone-sensitivity-high-symbolic")
}

// Show a notification when recording stops
func (nm *NotificationManager) NotifyStopRecording() error {
	if !nm.config.Notifications.EnableWorkflowNotifications {
		return nil // Skip if workflow notifications are disabled
	}
	return nm.sendNotification(constants.NotifyTitleRecordingStop, constants.NotifyRecordingStopMsg, "microphone-sensitivity-muted-symbolic")
}

// Show a notification when transcription is complete
func (nm *NotificationManager) NotifyTranscriptionComplete() error {
	if !nm.config.Notifications.EnableWorkflowNotifications {
		return nil // Skip if workflow notifications are disabled
	}
	// Select message based on current output mode
	body := constants.NotifyTranscriptionMsg
	if nm.config.Output.DefaultMode == config.OutputModeActiveWindow {
		body = constants.NotifyTranscriptionTypedMsg
	}
	return nm.sendNotification(constants.NotifyTitleTranscription, body, "edit-copy-symbolic")
}

// Show an error notification
func (nm *NotificationManager) NotifyError(errMsg string) error {
	return nm.sendNotification(constants.NotifyTitleError, errMsg, "dialog-error-symbolic")
}

// Show a notification when the configuration is reset to defaults
func (nm *NotificationManager) NotifyConfigurationReset() error {
	return nm.sendNotification(constants.NotifyTitleConfigReset, constants.NotifyConfigResetSuccess, "preferences-system-symbolic")
}

// Show a generic notification
func (nm *NotificationManager) ShowNotification(summary, body string) error {
	return nm.sendNotification(summary, body, "dialog-information-symbolic")
}

// Send a notification using the notify-send command
func (nm *NotificationManager) sendNotification(summary, body, icon string) error {
	// Security: validate command before execution
	if !config.IsCommandAllowed(nm.config, "notify-send") {
		return fmt.Errorf("notify-send command not allowed")
	}

	// "--" terminates option parsing so summary/body starting with "-"
	// (e.g. an err.Error() that begins with a flag-like token) are not
	// interpreted as notify-send options.
	args := []string{
		"--app-name", nm.appName,
		"--icon", icon,
		"--",
		summary, body,
	}

	// Security boundary is IsCommandAllowed above. exec.Command does not invoke a
	// shell, so argv values are passed literally via execve and metacharacters in
	// summary/body (e.g. error messages) are not an injection vector.
	// #nosec G204 -- Safe: notify-send is from an allowlist; argv values are literal.
	cmd := exec.Command("notify-send", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// Check if the notification system is available
func (nm *NotificationManager) IsAvailable() bool {
	_, err := exec.LookPath("notify-send")
	return err == nil
}
