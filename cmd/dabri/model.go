// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/constants"
	"github.com/AshBuk/dabri/v2/internal/ipc"
	"github.com/AshBuk/dabri/v2/internal/utils"
	"github.com/AshBuk/dabri/v2/whisper/providers"
)

// addModelCommand registers the "model" command tree:
//
//	model        — show current active model
//	model list   — list all available models
//	model set    — switch to a different model (IPC → daemon)
//	model delete — delete a downloaded model (IPC → daemon)
func addModelCommand(root *cobra.Command) {
	modelCmd := &cobra.Command{
		Use:   "model",
		Short: "Manage whisper models (list/set/delete)",
		Args:  cobra.NoArgs,
		// Bare "model" with no subcommand → show current model
		Run: func(cmd *cobra.Command, args []string) {
			printActiveModel()
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available whisper models",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			printModelList()
		},
	}

	setCmd := &cobra.Command{
		Use:   "set <model-id>",
		Short: "Switch to a different whisper model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelMutation(cmd, "set-model", args[0])
		},
		SilenceUsage: true,
	}
	addCLIFlags(setCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <model-id>",
		Short: "Delete a downloaded whisper model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelMutation(cmd, "delete-model", args[0])
		},
		SilenceUsage: true,
	}
	addCLIFlags(deleteCmd)

	modelCmd.AddCommand(listCmd, setCmd, deleteCmd)
	root.AddCommand(modelCmd)
}

// runModelMutation validates the model ID, sends an IPC request to the daemon,
// and formats the response.
func runModelMutation(cmd *cobra.Command, ipcCommand, modelID string) error {
	if constants.ModelByID(modelID) == nil {
		return fmt.Errorf("unknown model: %s (use 'dabri model list' to see available models)", modelID)
	}

	socketPath, _ := cmd.Flags().GetString("socket")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	timeoutSec, _ := cmd.Flags().GetInt("timeout")

	if socketPath == "" {
		socketPath = utils.GetDefaultSocketPath()
	}
	timeout := deriveTimeout(cmd.Name(), timeoutSec)

	req := ipc.Request{
		Command: ipcCommand,
		Params:  map[string]string{"model": modelID},
	}
	resp, err := ipc.SendRequest(socketPath, req, timeout)
	if err != nil {
		return err
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}
	printResponse(cmd.Name(), resp)
	return nil
}

func activeModelID() string {
	activeID := constants.DefaultModelID
	if path, err := config.ConfigFilePath(); err == nil {
		if cfg, err := config.LoadConfig(path); err == nil && cfg.General.WhisperModel != "" {
			activeID = cfg.General.WhisperModel
		}
	}
	return activeID
}

func printActiveModel() {
	activeID := activeModelID()
	if m := constants.ModelByID(activeID); m != nil {
		fmt.Printf("Active model: %s (%s)\n", m.ID, m.Name)
	} else {
		fmt.Printf("Active model: %s\n", activeID)
	}
}

func printModelList() {
	activeID := activeModelID()
	fmt.Println("Whisper models:")
	fmt.Println()
	for _, m := range constants.WhisperModels {
		status := "  "
		if m.ID == activeID {
			status = "● "
		}
		downloaded := ""
		resolver := providers.NewModelPathResolver(nil, m.FileName)
		if isModelDownloaded(resolver) {
			downloaded = " [downloaded]"
		}
		fmt.Printf("  %s%-18s %s%s\n", status, m.ID, m.Name, downloaded)
	}
	fmt.Println("\nUsage: dabri model set <model-id>")
	fmt.Println("       dabri model delete <model-id>")
}

func isModelDownloaded(resolver *providers.ModelPathResolver) bool {
	path := resolver.GetBundledModelPath()
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}
