// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// YdotoolDaemon owns a ydotoold process. Inside Flatpak on wlroots/Hyprland the
// host daemon's socket is not visible to the sandbox, so the client needs its own.
type YdotoolDaemon struct {
	cmd *exec.Cmd
}

// StartYdotoolDaemon launches ydotoold and exports YDOTOOL_SOCKET for the client.
// Returns nil when uinput is inaccessible (no --device=all / udev rule) or the
// daemon binary is missing, so callers transparently fall back to clipboard.
func StartYdotoolDaemon() *YdotoolDaemon {
	if !UinputWritable() {
		return nil
	}
	bin, err := exec.LookPath("ydotoold")
	if err != nil {
		return nil
	}
	socket := filepath.Join(runtimeDir(), ".ydotool_socket")
	_ = os.Setenv("YDOTOOL_SOCKET", socket)

	// #nosec G204 -- fixed binary resolved from PATH, no user-controlled input
	cmd := exec.Command(bin, "-p", socket, "-P", "0600")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil
	}
	waitForSocket(socket, 2*time.Second)
	return &YdotoolDaemon{cmd: cmd}
}

// Stop terminates the daemon
func (d *YdotoolDaemon) Stop() {
	if d == nil || d.cmd == nil || d.cmd.Process == nil {
		return
	}
	_ = d.cmd.Process.Kill()
	_ = d.cmd.Wait()
}

// UinputWritable reports whether /dev/uinput can be opened for writing
func UinputWritable() bool {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func runtimeDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d
	}
	return os.TempDir()
}

func waitForSocket(path string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
