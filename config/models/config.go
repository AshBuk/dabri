// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package models

// OutputMode constants define the supported methods for delivering the transcribed text.
// Using constants avoids magic strings and ensures consistency across the application.
const (
	OutputModeClipboard    = "clipboard"     // Copy text to the clipboard
	OutputModeActiveWindow = "active_window" // Type text into the currently active window
)

// Config defines the application's configuration structure, organized into logical groups.
// It uses YAML tags for serialization and deserialization.
type Config struct {
	General struct {
		Debug         bool   `yaml:"debug"`
		WhisperModel  string `yaml:"whisper_model"`   // The specific Whisper model to use (e.g., "small-q5_1")
		TempAudioPath string `yaml:"temp_audio_path"` // Directory to store temporary audio files
		Language      string `yaml:"language"`        // Language for speech recognition (e.g., "en", "ru")
	} `yaml:"general"`

	Hotkeys struct {
		// Hotkey provider: "dbus" (default, portal-based) | "evdev" (direct input, requires udev rule or input group).
		// Omit or leave empty for auto-selection: dbus → evdev → disabled.
		Provider        string `yaml:"provider"`
		StartRecording  string `yaml:"start_recording"`
		StopRecording   string `yaml:"stop_recording"`
		ShowConfig      string `yaml:"show_config"`
		ResetToDefaults string `yaml:"reset_to_defaults"`
	} `yaml:"hotkeys"`

	Audio struct {
		Device              string `yaml:"device"`
		SampleRate          int    `yaml:"sample_rate"`
		Format              string `yaml:"format"`
		RecordingMethod     string `yaml:"recording_method"`       // Recording backend: "arecord" or "ffmpeg"
		ExpectedDuration    int    `yaml:"expected_duration"`      // Expected recording duration in seconds (0 for indefinite)
		MaxRecordingTime    int    `yaml:"max_recording_time"`     // Maximum recording time in seconds to prevent runaway recordings
		TempFileCleanupTime int    `yaml:"temp_file_cleanup_time"` // Cleanup timeout for temp files in minutes (default: 30)
	} `yaml:"audio"`

	Output struct {
		DefaultMode   string `yaml:"default_mode"`   // Default output mode: "clipboard" or "active_window"
		ClipboardTool string `yaml:"clipboard_tool"` // Tool for clipboard operations (e.g., "wl-copy", "xsel"). "auto" for detection
		TypeTool      string `yaml:"type_tool"`      // Tool for typing text (e.g., "xdotool", "wtype"). "auto" for detection
	} `yaml:"output"`

	Notifications struct {
		EnableWorkflowNotifications bool `yaml:"enable_workflow_notifications"` // If true, show notifications for events like "Recording started"
	} `yaml:"notifications"`

	WebServer struct {
		Enabled     bool   `yaml:"enabled"`
		Port        int    `yaml:"port"`
		Host        string `yaml:"host"`
		AuthToken   string `yaml:"auth_token"`   // Optional Bearer token for API authentication
		APIVersion  string `yaml:"api_version"`  // API version string
		LogRequests bool   `yaml:"log_requests"` // If true, log incoming HTTP requests
		CORSOrigins string `yaml:"cors_origins"` // CORS origins to allow (e.g., "*" or "http://localhost:3000")
		MaxClients  int    `yaml:"max_clients"`  // Maximum number of concurrent WebSocket clients
	} `yaml:"web_server"`

	Security struct {
		AllowedCommands []string `yaml:"allowed_commands"`   // Whitelist of external commands the application is allowed to execute
		CheckIntegrity  bool     `yaml:"check_integrity"`    // If true, verify the config file's hash on startup
		ConfigHash      string   `yaml:"config_hash"`        // The SHA-256 hash of the config file for integrity checking
		MaxTempFileSize int64    `yaml:"max_temp_file_size"` // Maximum allowed size for temporary audio files in bytes
	} `yaml:"security"`
}
