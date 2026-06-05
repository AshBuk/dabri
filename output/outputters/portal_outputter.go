// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AshBuk/go-wlportal/remotedesktop"

	"github.com/AshBuk/dabri/v2/output/interfaces"
)

// PortalRemoteDesktopAvailable reports whether the RemoteDesktop portal exposes
// keyboard injection. This is the sandbox-clean typing path on GNOME/KDE; the
// wlroots/Hyprland portal does not implement it.
func PortalRemoteDesktopAvailable() bool {
	return remotedesktop.Available()
}

// PortalOutputter types text through the RemoteDesktop portal (go-wlportal)
type PortalOutputter struct {
	kbd *remotedesktop.Keyboard
	// clipboard handles text the portal cannot type (non-ASCII), nil disables fallback
	clipboard interfaces.Outputter
}

// NewPortalOutputter creates a portal-based type outputter (session is opened lazily).
// clipboard is an optional fallback used for characters the portal cannot inject.
func NewPortalOutputter(clipboard interfaces.Outputter) (interfaces.Outputter, error) {
	kbd, err := remotedesktop.NewKeyboard(remotedesktop.WithRestoreTokenPath(portalTokenPath()))
	if err != nil {
		return nil, err
	}
	return &PortalOutputter{kbd: kbd, clipboard: clipboard}, nil
}

// TypeToActiveWindow injects text as keyboard input into the focused window.
// The RemoteDesktop portal maps keysyms through the compositor's active layout,
// so characters outside it (e.g. Cyrillic on a Latin layout) are silently dropped.
// Such non-ASCII text is routed to the clipboard so nothing is lost; the typing
// mode itself is preserved, so subsequent ASCII text keeps typing.
func (o *PortalOutputter) TypeToActiveWindow(text string) error {
	if o.clipboard != nil && isNonASCII(text) {
		return o.clipboard.CopyToClipboard(text)
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
