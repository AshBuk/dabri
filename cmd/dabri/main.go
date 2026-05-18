// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	appversion "github.com/AshBuk/dabri/v2/internal/version"
)

// Command Router — two execution modes:
//
//	→ Root (no subcommand): daemon — app.NewApp() → Initialize() → RunAndWait()
//	→ Subcommands (client): start/stop/toggle/status/transcript/model → IPC
var rootCmd = &cobra.Command{
	Use:   "dabri",
	Short: "Linux STT",
	Long:  "Dabri — offline speech recognition with hotkey support, system tray integration, and CLI control.",
	// Root command with no subcommand → run as daemon
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemonCobra(cmd)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
	Version: fmt.Sprintf("%s (%s %s/%s)",
		appversion.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH),
}

func init() {
	addDaemonFlags(rootCmd)
	addCLICommands(rootCmd)
	addModelCommand(rootCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
