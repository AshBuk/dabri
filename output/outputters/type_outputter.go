// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"fmt"
	"os/exec"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/platform"
	"github.com/AshBuk/dabri/v2/output/interfaces"
)

// Implements the Outputter interface for typing text into the active window
type TypeOutputter struct {
	typeTool string
	config   *config.Config
}

// Create a new type outputter
func NewTypeOutputter(typeTool string, cfg *config.Config) (interfaces.Outputter, error) {
	// Verify the required tool exists in the system's PATH
	if _, err := exec.LookPath(typeTool); err != nil {
		return nil, fmt.Errorf("type tool not found: %s", typeTool)
	}
	return &TypeOutputter{
		typeTool: typeTool,
		config:   cfg,
	}, nil
}

// Simulate typing text into the currently active window
func (o *TypeOutputter) TypeToActiveWindow(text string) error {
	// Security: validate the command before execution
	if !config.IsCommandAllowed(o.config, o.typeTool) {
		return fmt.Errorf("typing tool not allowed: %s", o.typeTool)
	}
	// Proactive fallback: ydotool on Wayland does not support non-ASCII characters
	if platform.DetectEnvironment() == platform.EnvironmentWayland && o.typeTool == "ydotool" && isNonASCII(text) {
		return fmt.Errorf("ydotool on Wayland doesn't support non-ASCII characters, use clipboard fallback")
	}
	var cmd *exec.Cmd
	var args []string

	// Use "--" to terminate option parsing where supported, so a transcript
	// that happens to start with "-"/"--" is not interpreted as a flag.
	switch o.typeTool {
	case "xdotool":
		args = []string{"type", "--clearmodifiers", "--", text}
	case "wtype":
		args = []string{"--", text}
	case "ydotool":
		// ydotool's option parser does not document "--", pass text directly.
		args = []string{"type", text}
	default:
		return fmt.Errorf("unsupported typing tool: %s", o.typeTool)
	}

	// #nosec G204 -- Tool is allowlisted; text is passed as argv, not shell input.
	cmd = exec.Command(o.typeTool, args...)
	// Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Runtime fallback: if wtype fails, try ydotool if it is allowed and available
		if o.typeTool == "wtype" && config.IsCommandAllowed(o.config, "ydotool") {
			if _, lookErr := exec.LookPath("ydotool"); lookErr == nil {
				// #nosec G204 -- Tool is allowlisted; text is passed as argv, not shell input.
				fallbackCmd := exec.Command("ydotool", "type", text)
				if fbOut, fbErr := fallbackCmd.CombinedOutput(); fbErr == nil {
					return nil
				} else {
					return fmt.Errorf("wtype failed: %w (out: %s); ydotool fallback failed: %v (out: %s)", err, string(output), fbErr, string(fbOut))
				}
			}
		}
		return fmt.Errorf("failed to type text with %s: %w, output: %s", o.typeTool, err, string(output))
	}
	return nil
}

// Return an error as clipboard operations are not supported by this outputter
func (o *TypeOutputter) CopyToClipboard(text string) error {
	return fmt.Errorf("copying to clipboard not supported by type outputter")
}

// Return the name of the typing tool being used
func (o *TypeOutputter) GetToolNames() (clipboardTool, typeTool string) {
	return "", o.typeTool
}

// Check if a string contains any non-ASCII characters
func isNonASCII(text string) bool {
	for _, r := range text {
		if r > 127 {
			return true
		}
	}
	return false
}
