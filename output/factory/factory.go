// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package factory

import (
	"fmt"
	"os/exec"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/platform"
	"github.com/AshBuk/dabri/v2/output/interfaces"
	"github.com/AshBuk/dabri/v2/output/outputters"
)

// EnvironmentType is an alias for platform.EnvironmentType to avoid converter boilerplate
type EnvironmentType = platform.EnvironmentType

// Re-exported environment constants for package-local convenience
const (
	EnvironmentX11     = platform.EnvironmentX11
	EnvironmentWayland = platform.EnvironmentWayland
	EnvironmentUnknown = platform.EnvironmentUnknown
)

// Factory creates output managers (clipboard/typing) based on environment and configuration
// Specialized subfactory used by ServiceFactory hierarchy
//
// Factory Hierarchy:
//
//	ServiceFactory (internal/services/factory.go)
//	    │
//	    ├── Stage 1: FactoryComponents
//	    │       │
//	    │       └── uses → outputFactory.GetOutputterFromConfig() (this file)
//	    │                     │
//	    │                     └── creates → ClipboardOutputter or TypeOutputter
//	    │
//	    ├── Stage 2: FactoryAssembler
//	    └── Stage 3: FactoryWirer
//
// Tool Selection Strategy:
//   - Auto-detection: checks environment (X11/Wayland/GNOME) and tool availability
//   - Priority chains: Wayland: wl-copy | X11: xsel (clipboard)
//     GNOME+Wayland: ydotool>wtype | Other Wayland: wtype>ydotool | X11: xdotool (typing)
//   - Config override: manual tool selection via config.Output.ClipboardTool/TypeTool
//   - Security: allowlist validation via config.IsCommandAllowed
//
// Usage:
//
//	GetOutputterFromConfig(config, EnvironmentWayland) // one-line creation
type Factory struct {
	config *config.Config
}

// NewFactory Constructor - initializes factory with config for tool selection
func NewFactory(config *config.Config) *Factory {
	return &Factory{config: config}
}

// selectClipboardTool Tool selection - chooses clipboard tool based on environment
// Config override: returns config.Output.ClipboardTool if not "auto"
func (f *Factory) selectClipboardTool(env EnvironmentType) string {
	if tool := f.config.Output.ClipboardTool; tool != "auto" {
		return tool
	}
	switch env {
	case EnvironmentWayland:
		return "wl-copy"
	case EnvironmentX11:
		return "xsel"
	default:
		return "xsel"
	}
}

// selectTypeTool Tool selection - chooses typing tool based on environment
// Config override: returns config.Output.TypeTool if not "auto"
func (f *Factory) selectTypeTool(env EnvironmentType) string {
	if tool := f.config.Output.TypeTool; tool != "auto" {
		return tool
	}
	switch env {
	case EnvironmentWayland:
		return f.selectWaylandTypeTool()
	case EnvironmentX11:
		return "xdotool"
	default:
		return f.selectFallbackTypeTool()
	}
}

// selectWaylandTypeTool Wayland-specific selection - delegates to GNOME vs non-GNOME chains
func (f *Factory) selectWaylandTypeTool() string {
	if platform.IsGNOMEWithWayland() {
		return f.selectGNOMEWaylandTypeTool()
	}
	return f.selectNonGNOMEWaylandTypeTool()
}

// selectGNOMEWaylandTypeTool GNOME+Wayland priority chain
// Priority: ydotool (best compatibility) → wtype → xdotool (XWayland fallback)
func (f *Factory) selectGNOMEWaylandTypeTool() string {
	if f.isToolAvailable("ydotool") {
		return "ydotool"
	}
	if f.isToolAvailable("wtype") {
		return "wtype"
	}
	return "xdotool"
}

// selectNonGNOMEWaylandTypeTool Non-GNOME Wayland priority chain
// Priority: wtype (native Wayland) → ydotool → xdotool (XWayland fallback)
func (f *Factory) selectNonGNOMEWaylandTypeTool() string {
	// Inside Flatpak the wlroots security-context blocks wtype's virtual-keyboard
	// protocol, so ydotool (uinput) is the only working CLI path.
	if platform.IsFlatpak() && f.isToolAvailable("ydotool") {
		return "ydotool"
	}
	if f.isToolAvailable("wtype") {
		return "wtype"
	}
	if f.isToolAvailable("ydotool") {
		return "ydotool"
	}
	return "xdotool"
}

// selectFallbackTypeTool Unknown environment - auto-detect first available tool
// Priority: xdotool (widest compatibility) → wtype → ydotool
func (f *Factory) selectFallbackTypeTool() string {
	for _, tool := range []string{"xdotool", "wtype", "ydotool"} {
		if f.isToolAvailable(tool) {
			return tool
		}
	}
	return "xdotool"
}

// GetOutputter Factory Method - creates outputter instance from environment + config
// Process: tool selection → security validation → outputter creation
// Security: validates tools via config.IsCommandAllowed before instantiation
// Returns ClipboardOutputter or TypeOutputter based on config.Output.DefaultMode
func (f *Factory) GetOutputter(env EnvironmentType) (interfaces.Outputter, error) {
	if f.config.Output.DefaultMode == config.OutputModeActiveWindow {
		return f.createTypeOutputter(env)
	}
	return f.createClipboardOutputter(env)
}

// createClipboardOutputter builds a validated clipboard outputter
func (f *Factory) createClipboardOutputter(env EnvironmentType) (interfaces.Outputter, error) {
	tool := f.selectClipboardTool(env)
	if tool != "" && !config.IsCommandAllowed(f.config, tool) {
		return nil, fmt.Errorf("clipboard tool not allowed: %s", tool)
	}
	return outputters.NewClipboardOutputter(tool, f.config)
}

// createTypeOutputter builds a validated active-window outputter.
// Prefers the RemoteDesktop portal on Wayland (GNOME/KDE) when the typing tool is
// auto-selected: it is the only path that works inside a Flatpak sandbox without
// extra device permissions. Otherwise it falls back to a CLI tool.
func (f *Factory) createTypeOutputter(env EnvironmentType) (interfaces.Outputter, error) {
	if env == EnvironmentWayland && f.config.Output.TypeTool == "auto" && outputters.PortalRemoteDesktopAvailable() {
		if out, err := outputters.NewPortalOutputter(); err == nil {
			return out, nil
		}
	}
	tool := f.selectTypeTool(env)
	if tool != "" && !config.IsCommandAllowed(f.config, tool) {
		return nil, fmt.Errorf("type tool not allowed: %s", tool)
	}
	return outputters.NewTypeOutputter(tool, f.config)
}

// Create an outputter directly from a configuration
func GetOutputterFromConfig(config *config.Config, env EnvironmentType) (interfaces.Outputter, error) {
	factory := NewFactory(config)
	return factory.GetOutputter(env)
}

// Check if a command-line tool is available in the system's PATH
func (f *Factory) isToolAvailable(toolName string) bool {
	_, err := exec.LookPath(toolName)
	return err == nil
}
