// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package constants

// AppID is the application's freedesktop identifier. It matches the installed
// .desktop file and is declared to the desktop portal so it can resolve the
// app's name and icon when opening portal sessions.
const AppID = "io.github.ashbuk.dabri"

// Common UI Messages
const (
	MsgReady                  = "Ready"
	MsgRecording              = "Recording..."
	MsgTranscribing           = "Transcribing..."
	MsgTranscriptionComplete  = "Transcription complete"
	MsgModelUnavailable       = "Model unavailable"
	MsgRecorderUnavailable    = "Audio recorder unavailable"
	MsgTranscriptionFailed    = "Transcription failed"
	MsgTranscriptionEmpty     = "No speech was detected in the recording"
	MsgNoSpeechDetected       = "No speech detected"
	MsgModelSwitchFailed      = "Model switch failed"
	MsgTranscriptionCancelled = "Transcription cancelled"
)

// Notification Titles
const (
	NotifyError             = "Error"
	NotifySuccess           = "Success"
	NotifyNoSpeech          = "No Speech"
	NotifyConfigReset       = "Configuration Reset"
	NotifyRecordingStarted  = "Recording Started"
	NotifyRecordingStopped  = "Recording stopped"
	NotifyTranscriptionDone = "Transcription Complete"
	NotifyTranscriptionErr  = "Transcription Error"
)

// Notification Messages
const (
	NotifyTypingFallback         = "Typing not supported by compositor. Text copied to clipboard - press Ctrl+V to paste."
	NotifyOutputBothFailed       = "both typing and clipboard failed, check output configuration"
	NotifyClipboardFallback      = "Text copied to clipboard - press Ctrl+V to paste."
	NotifyTranscriptionCancelled = "Transcription was cancelled"
	NotifyConfigResetSuccess     = "Settings reset to defaults successfully"
	NotifyRecordingStartMsg      = "Recording started"
	NotifyRecordingStopMsg       = "Transcribing audio..."
	NotifyTranscriptionMsg       = "Text copied to clipboard"
	NotifyTranscriptionTypedMsg  = "Text typed to active window"
	NotifyAppName                = "Dabri"
)

// Notification Full Titles
// Visual indication is delegated to freedesktop symbolic icons passed via notify-send --icon.
const (
	NotifyTitleRecordingStart = NotifyAppName
	NotifyTitleRecordingStop  = NotifyRecordingStopped
	NotifyTitleTranscription  = NotifyTranscriptionDone
	NotifyTitleError          = NotifyError
	NotifyTitleConfigReset    = NotifyConfigReset
)
