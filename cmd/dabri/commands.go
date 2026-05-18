// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/AshBuk/dabri/v2/internal/ipc"
	"github.com/AshBuk/dabri/v2/internal/utils"
)

// addCLICommands registers all IPC-based subcommands on the root command.
// Each command sends a request to the running daemon via Unix socket.
func addCLICommands(root *cobra.Command) {
	root.AddCommand(
		newIPCCommand("start", "Start recording", "start-recording"),
		newIPCCommand("stop", "Stop recording and return transcript", "stop-recording"),
		newIPCCommand("toggle", "Toggle recording (start if stopped, stop if recording)", "toggle-recording"),
		newIPCCommand("status", "Show current state and configuration", "status"),
		newIPCCommand("transcript", "Show the last transcript", "last-transcript"),
	)
}

// newIPCCommand creates a cobra.Command that sends an IPC request to the daemon.
// Factory Pattern — shared flags and execution logic for all IPC commands.
func newIPCCommand(use, short, ipcCommand string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIPCCommand(cmd, ipcCommand)
		},
		SilenceUsage: true,
	}
	addCLIFlags(cmd)
	return cmd
}

func addCLIFlags(cmd *cobra.Command) {
	cmd.Flags().String("socket", "", "Path to IPC socket (defaults to user runtime path)")
	cmd.Flags().Bool("json", false, "Print responses as JSON")
	cmd.Flags().Int("timeout", 0, "Override timeout in seconds for the command")
}

// runIPCCommand executes a single IPC request to the daemon and formats the response.
// Execution flow: read flags → send IPC → format output (text or JSON)
func runIPCCommand(cmd *cobra.Command, ipcCommand string) error {
	socketPath, _ := cmd.Flags().GetString("socket")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	timeoutSec, _ := cmd.Flags().GetInt("timeout")

	if socketPath == "" {
		socketPath = utils.GetDefaultSocketPath()
	}

	name := cmd.Name()
	timeout := deriveTimeout(name, timeoutSec)

	req := ipc.Request{Command: ipcCommand}
	resp, err := ipc.SendRequest(socketPath, req, timeout)
	if err != nil {
		return err
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}
	printResponse(name, resp)
	return nil
}

const (
	defaultStatusTimeout = 5 * time.Second
	defaultStopTimeout   = 60 * time.Second
	defaultModelTimeout  = 5 * time.Minute
)

func deriveTimeout(command string, override int) time.Duration {
	if override > 0 {
		return time.Duration(override) * time.Second
	}

	switch command {
	case "stop", "toggle":
		return defaultStopTimeout
	case "set", "delete":
		return defaultModelTimeout
	default:
		return defaultStatusTimeout
	}
}

func printResponse(command string, resp ipc.Response) {
	data := mapFromResponse(resp.Data)
	switch command {
	case "start":
		fmt.Println("Recording started.")
	case "toggle":
		recording := getBoolOr(data, "recording", false)
		if recording {
			fmt.Println("Recording started.")
		} else {
			if transcript, ok := getString(data, "transcript"); ok && transcript != "" {
				fmt.Println(transcript)
			} else {
				fmt.Println("Recording stopped.")
			}
		}
	case "stop":
		if warning, ok := getString(data, "warning"); ok && warning != "" {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
		}
		if transcript, ok := getString(data, "transcript"); ok && transcript != "" {
			fmt.Println(transcript)
		} else {
			fmt.Println("Recording stopped (no transcript available).")
		}
	case "status":
		printStatusResponse(data)
	case "transcript":
		if transcript, ok := getString(data, "transcript"); ok && transcript != "" {
			fmt.Println(transcript)
		} else {
			fmt.Println("No transcript available.")
		}
	case "set":
		if model, ok := getString(data, "model"); ok && model != "" {
			fmt.Printf("Model switched to: %s\n", model)
		}
	case "delete":
		if model, ok := getString(data, "model"); ok && model != "" {
			fmt.Printf("Model deleted: %s\n", model)
		}
	default:
		if resp.Message != "" {
			fmt.Println(resp.Message)
		}
	}
}

func printStatusResponse(data map[string]any) {
	recording := getBoolOr(data, "recording", false)
	fmt.Printf("Recording: %t\n", recording)

	// Config section
	if cfg := getMap(data, "config"); cfg != nil {
		fmt.Println("\nConfig:")
		if model, ok := getString(cfg, "model"); ok && model != "" {
			fmt.Printf("  Model: %s\n", model)
		}
		if lang, ok := getString(cfg, "language"); ok && lang != "" {
			fmt.Printf("  Language: %s\n", lang)
		}
		if mode, ok := getString(cfg, "output_mode"); ok && mode != "" {
			fmt.Printf("  Output mode: %s\n", mode)
		}
		if method, ok := getString(cfg, "audio_method"); ok && method != "" {
			fmt.Printf("  Audio method: %s\n", method)
		}
	}
	// Hotkeys section
	if hotkeys := getMap(data, "hotkeys"); hotkeys != nil {
		fmt.Println("\nHotkeys:")
		if startStop, ok := getString(hotkeys, "start_stop"); ok && startStop != "" {
			fmt.Printf("  Start/Stop: %s\n", startStop)
		}
		if showConfig, ok := getString(hotkeys, "show_config"); ok && showConfig != "" {
			fmt.Printf("  Show config: %s\n", showConfig)
		}
	}
	// WebSocket section
	if ws := getMap(data, "websocket"); ws != nil {
		enabled := getBoolOr(ws, "enabled", false)
		if enabled {
			host, _ := getString(ws, "host")
			port := getIntOr(ws, "port", 0)
			fmt.Printf("\nWebSocket: enabled (%s:%d)\n", host, port)
		} else {
			fmt.Println("\nWebSocket: disabled")
		}
	}
}

// --- Response parsing helpers ---

func mapFromResponse(data any) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	if m, ok := data.(map[string]any); ok && m != nil {
		return m
	}
	return map[string]any{}
}

func getString(data map[string]any, key string) (string, bool) {
	if value, ok := data[key]; ok {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

func getBoolOr(data map[string]any, key string, fallback bool) bool {
	if value, ok := data[key]; ok {
		switch v := value.(type) {
		case bool:
			return v
		case string:
			return strings.EqualFold(v, "true")
		}
	}
	return fallback
}

func getMap(data map[string]any, key string) map[string]any {
	if value, ok := data[key]; ok {
		if m, ok := value.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func getIntOr(data map[string]any, key string, fallback int) int {
	if value, ok := data[key]; ok {
		switch v := value.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return fallback
}
