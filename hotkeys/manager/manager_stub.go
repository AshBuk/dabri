//go:build !linux

// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package manager

import (
	"github.com/AshBuk/dabri/v2/hotkeys/adapters"
	"github.com/AshBuk/dabri/v2/hotkeys/interfaces"
	"github.com/AshBuk/dabri/v2/hotkeys/providers"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// Return a dummy provider on non-Linux systems to avoid build errors
func selectProviderForEnvironment(_ adapters.HotkeyConfig, _ interfaces.EnvironmentType, logger logger.Logger) interfaces.KeyboardEventProvider {
	return providers.NewDummyKeyboardProvider(logger)
}
