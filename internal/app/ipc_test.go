// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package app

import (
	"errors"
	"slices"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/ipc"
	"github.com/AshBuk/dabri/v2/internal/services"
	"github.com/AshBuk/dabri/v2/internal/testutils"
	"github.com/AshBuk/dabri/v2/tests/mocks"
)

func newMockedApp() (*App, *mocks.CallLog, *mocks.MockAudioService, *mocks.MockConfigService) {
	log := &mocks.CallLog{}
	audio := &mocks.MockAudioService{Log: log}
	cfg := &mocks.MockConfigService{Log: log, Cfg: &config.Config{}}
	cfg.Cfg.General.Model = "base"
	cfg.Cfg.Hotkeys.StartRecording = "alt+r"
	a := NewApp(testutils.NewMockLogger())
	a.Services = &services.ServiceContainer{
		Audio:   audio,
		UI:      &mocks.MockUIService{Log: log},
		IO:      &mocks.MockIOService{Log: log},
		Config:  cfg,
		Hotkeys: &mocks.MockHotkeyService{Log: log},
	}
	return a, log, audio, cfg
}

func expectCalls(t *testing.T, log *mocks.CallLog, want ...string) {
	t.Helper()
	if !slices.Equal(log.Calls, want) {
		t.Errorf("calls:\n got  %q\n want %q", log.Calls, want)
	}
}

func TestIPCToggle(t *testing.T) {
	a, log, audio, _ := newMockedApp()
	if _, err := a.ipcHandleToggleRecording(ipc.Request{}); err != nil {
		t.Fatal(err)
	}
	audio.Recording = true
	resp, err := a.ipcHandleToggleRecording(ipc.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resp.Data.(map[string]any)["transcript"]; !ok {
		t.Errorf("stop response lacks transcript: %+v", resp)
	}
	expectCalls(t, log, "Audio.Start", "Audio.Stop")
}

func TestIPCSetModel(t *testing.T) {
	a, log, _, _ := newMockedApp()
	if _, err := a.ipcHandleSetModel(ipc.Request{Params: map[string]string{"model": "small"}}); err != nil {
		t.Fatal(err)
	}
	// IPC reports through its response, never through desktop notifications
	expectCalls(t, log, "Audio.SwitchModel:small", "Config.Model:small", "UI.UpdateSettings")
}

func TestIPCSetModel_Errors(t *testing.T) {
	a, log, audio, _ := newMockedApp()
	if _, err := a.ipcHandleSetModel(ipc.Request{}); err == nil {
		t.Error("missing model must fail")
	}
	audio.SwitchModelErr = map[string]error{"small": errors.New("boom")}
	_, err := a.ipcHandleSetModel(ipc.Request{Params: map[string]string{"model": "small"}})
	if err == nil || err.Error() != "boom" {
		t.Errorf("got %v", err)
	}
	expectCalls(t, log, "Audio.SwitchModel:small")
}

func TestIPCSetModel_PersistFailsRollsBack(t *testing.T) {
	a, log, _, cfg := newMockedApp()
	cfg.UpdateModelErr = errors.New("disk full")
	_, err := a.ipcHandleSetModel(ipc.Request{Params: map[string]string{"model": "small"}})
	if err == nil || err.Error() != "failed to persist model setting: disk full" {
		t.Errorf("got %v", err)
	}
	expectCalls(t, log, "Audio.SwitchModel:small", "Config.Model:small", "Audio.SwitchModel:base")
}
