// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/output/interfaces"
)

const (
	pasteToolYdotool         = "ydotool"
	pasteShortcutCtrlV       = "ctrl+v"
	pasteShortcutCtrlShiftV  = "ctrl+shift+v"
	pasteShortcutShiftInsert = "shift+insert"
	keyLeftCtrl              = "29"
	keyLeftShift             = "42"
	keyV                     = "47"
	keyInsert                = "110"
)

// AutoPasteOutputter types ASCII normally and pastes non-ASCII text via clipboard.
type AutoPasteOutputter struct {
	typing     interfaces.Outputter
	clipboard  interfaces.Outputter
	pasteTool  string
	config     *config.Config
	pasteDelay time.Duration
}

// NewAutoPasteOutputter creates an outputter that uses Ctrl+V for Unicode text.
func NewAutoPasteOutputter(typing, clipboard interfaces.Outputter, pasteTool string, cfg *config.Config) interfaces.Outputter {
	return &AutoPasteOutputter{
		typing:     typing,
		clipboard:  clipboard,
		pasteTool:  pasteTool,
		config:     cfg,
		pasteDelay: 80 * time.Millisecond,
	}
}

// CopyToClipboard delegates clipboard writes to the configured clipboard backend.
func (o *AutoPasteOutputter) CopyToClipboard(text string) error {
	return o.clipboard.CopyToClipboard(text)
}

// TypeToActiveWindow sends ASCII through the typing backend and Unicode through paste.
func (o *AutoPasteOutputter) TypeToActiveWindow(text string) error {
	if !isNonASCII(text) {
		return o.typing.TypeToActiveWindow(text)
	}
	if err := o.clipboard.CopyToClipboard(text); err != nil {
		return err
	}
	time.Sleep(o.pasteDelay)
	return o.pressPaste()
}

// GetToolNames reports the composed clipboard and typing tools.
func (o *AutoPasteOutputter) GetToolNames() (clipboardTool, typeTool string) {
	clipboardTool, _ = o.clipboard.GetToolNames()
	_, typeTool = o.typing.GetToolNames()
	if typeTool == "" {
		typeTool = o.pasteTool
	}
	return clipboardTool, typeTool + "+paste"
}

func (o *AutoPasteOutputter) pressPaste() error {
	if !config.IsCommandAllowed(o.config, o.pasteTool) {
		return fmt.Errorf("paste tool not allowed: %s", o.pasteTool)
	}
	if o.pasteTool != pasteToolYdotool {
		return fmt.Errorf("unsupported paste tool: %s", o.pasteTool)
	}
	if _, err := exec.LookPath(o.pasteTool); err != nil {
		return fmt.Errorf("paste tool not found: %s", o.pasteTool)
	}

	args, err := pasteKeyArgs(o.config.Output.PasteShortcut)
	if err != nil {
		return err
	}
	// #nosec G204 -- Tool is allowlisted; arguments are fixed key codes.
	cmd := exec.Command(o.pasteTool, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to paste with %s: %w, output: %s", o.pasteTool, err, string(output))
	}
	return nil
}

func pasteKeyArgs(shortcut string) ([]string, error) {
	switch normalizePasteShortcut(shortcut) {
	case pasteShortcutCtrlV:
		return []string{"key", keyLeftCtrl + ":1", keyV + ":1", keyV + ":0", keyLeftCtrl + ":0"}, nil
	case pasteShortcutCtrlShiftV:
		return []string{"key", keyLeftCtrl + ":1", keyLeftShift + ":1", keyV + ":1", keyV + ":0", keyLeftShift + ":0", keyLeftCtrl + ":0"}, nil
	case pasteShortcutShiftInsert:
		return []string{"key", keyLeftShift + ":1", keyInsert + ":1", keyInsert + ":0", keyLeftShift + ":0"}, nil
	default:
		return nil, fmt.Errorf("unsupported paste shortcut: %s", shortcut)
	}
}

func normalizePasteShortcut(shortcut string) string {
	shortcut = strings.ToLower(strings.TrimSpace(shortcut))
	if shortcut == "" {
		return pasteShortcutCtrlV
	}
	return strings.ReplaceAll(shortcut, " ", "")
}
