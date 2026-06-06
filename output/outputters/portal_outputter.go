// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AshBuk/go-wlportal/typing"

	"github.com/AshBuk/dabri/v2/output/interfaces"
)

// PortalRemoteDesktopAvailable reports whether the RemoteDesktop portal exposes
// keyboard injection. This is the sandbox-clean typing path on GNOME/KDE; the
// wlroots/Hyprland portal does not implement it.
func PortalRemoteDesktopAvailable() bool {
	return typing.Available()
}

// types text through the RemoteDesktop portal via the go-wlportal/typing adapter
type PortalOutputter struct {
	kbd *typing.Keyboard
}

// NewPortalOutputter creates a portal-based type outputter (session is opened lazily).
func NewPortalOutputter() (interfaces.Outputter, error) {
	kbd, err := typing.NewKeyboard(typing.WithRestoreTokenPath(portalTokenPath()))
	if err != nil {
		return nil, err
	}
	return &PortalOutputter{kbd: kbd}, nil
}

// TypeToActiveWindow injects text as keyboard input into the focused window.
// The portal can only type characters in the active keyboard layout, so it
// errors on non-ASCII text and lets IOService fall back to clipboard.
func (o *PortalOutputter) TypeToActiveWindow(text string) error {
	if isNonASCII(text) {
		return fmt.Errorf("portal cannot type non-ASCII text through the active keyboard layout")
	}
	return o.kbd.Type(text)
}

// CopyToClipboard is not supported by this outputter
func (o *PortalOutputter) CopyToClipboard(text string) error {
	return fmt.Errorf("copying to clipboard not supported by portal outputter")
}

// GetToolNames reports the active typing backend
func (o *PortalOutputter) GetToolNames() (clipboardTool, typeTool string) {
	return "", "portal"
}

// portalTokenPath returns where the portal permission token is persisted so the
// consent dialog appears only once across restarts.
func portalTokenPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "dabri", "portal-remotedesktop.token")
}
