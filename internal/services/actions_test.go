// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package services

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/AshBuk/dabri/v2/config"
	"github.com/AshBuk/dabri/v2/internal/testutils"
	"github.com/AshBuk/dabri/v2/tests/mocks"
)

type actionFixture struct {
	log     *mocks.CallLog
	audio   *mocks.MockAudioService
	cfg     *mocks.MockConfigService
	hotkeys *mocks.MockHotkeyService
	c       *ServiceContainer
}

func newActionFixture() *actionFixture {
	log := &mocks.CallLog{}
	f := &actionFixture{
		log:     log,
		audio:   &mocks.MockAudioService{Log: log},
		cfg:     &mocks.MockConfigService{Log: log, Cfg: &config.Config{}},
		hotkeys: &mocks.MockHotkeyService{Log: log},
	}
	f.cfg.Cfg.General.Model = "base"
	f.cfg.Cfg.Hotkeys.StartRecording = "alt+r"
	f.c = &ServiceContainer{
		Logger:  testutils.NewMockLogger(),
		Audio:   f.audio,
		UI:      &mocks.MockUIService{Log: log},
		IO:      &mocks.MockIOService{Log: log},
		Config:  f.cfg,
		Hotkeys: f.hotkeys,
	}
	return f
}

func (f *actionFixture) expect(t *testing.T, want ...string) {
	t.Helper()
	if !slices.Equal(f.log.Calls, want) {
		t.Errorf("calls:\n got  %q\n want %q", f.log.Calls, want)
	}
}

func TestToggleRecording(t *testing.T) {
	f := newActionFixture()
	_ = f.c.ToggleRecording()
	f.audio.Recording = true
	_ = f.c.ToggleRecording()
	f.expect(t, "Audio.Start", "Audio.Stop")
}

func TestRecordingHotkey(t *testing.T) {
	// Toggle mode acts on press only
	f := newActionFixture()
	_ = f.c.RecordingHotkey(true)
	_ = f.c.RecordingHotkey(false)
	f.expect(t, "Audio.Start")

	// Push-to-talk records while held
	f = newActionFixture()
	f.hotkeys.PTT = true
	_ = f.c.RecordingHotkey(true)
	_ = f.c.RecordingHotkey(false)
	f.expect(t, "Audio.Start", "Audio.Stop")
}

func TestSetHotkeyMode(t *testing.T) {
	// The mode picks the hotkey provider, so switching it reloads hotkeys
	f := newActionFixture()
	if err := f.c.setHotkeyMode(config.HotkeyModePTT); err != nil {
		t.Fatal(err)
	}
	f.expect(t, "Config.HotkeyMode:ptt", "Hotkeys.Reload:alt+r", "UI.UpdateSettings")

	// A failed reload tells the user, since hotkey errors only reach the log
	f = newActionFixture()
	f.hotkeys.ReloadErr = errors.New("no provider")
	if err := f.c.setHotkeyMode(config.HotkeyModeToggle); err == nil {
		t.Fatal("expected error")
	}
	f.expect(t, "Config.HotkeyMode:toggle", "Hotkeys.Reload:alt+r", "UI.UpdateSettings", "UI.Notify:Hotkeys unavailable")
}

func TestResetToDefaults(t *testing.T) {
	f := newActionFixture()
	if err := f.c.ResetToDefaults(); err != nil {
		t.Fatal(err)
	}
	f.expect(t, "Config.Reset", "Hotkeys.Reload:alt+r", "UI.UpdateSettings")

	f = newActionFixture()
	f.cfg.ResetErr = errors.New("boom")
	if err := f.c.ResetToDefaults(); err == nil {
		t.Fatal("expected error")
	}
	f.expect(t, "Config.Reset")
}

func TestSwitchModel(t *testing.T) {
	f := newActionFixture()
	if err := f.c.selectModel(context.Background(), "small"); err != nil {
		t.Fatal(err)
	}
	f.expect(t, "Audio.SwitchModel:small", "Config.Model:small", "UI.UpdateSettings", "UI.Notify:Model Switch")
}

func TestSwitchModel_SwitchFails(t *testing.T) {
	f := newActionFixture()
	f.audio.SwitchModelErr = map[string]error{"small": errors.New("boom")}
	if err := f.c.selectModel(context.Background(), "small"); err == nil {
		t.Fatal("expected error")
	}
	f.expect(t, "Audio.SwitchModel:small", "UI.Notify:Model Switch Failed")
}

func TestSwitchModel_CancelledIsSilent(t *testing.T) {
	f := newActionFixture()
	f.audio.SwitchModelErr = map[string]error{"small": context.Canceled}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := f.c.selectModel(ctx, "small"); err == nil {
		t.Fatal("expected error")
	}
	f.expect(t, "Audio.SwitchModel:small")
}

func TestSwitchModel_PersistFailsRollsBack(t *testing.T) {
	f := newActionFixture()
	f.cfg.UpdateModelErr = errors.New("disk full")
	if err := f.c.selectModel(context.Background(), "small"); err == nil {
		t.Fatal("expected error")
	}
	f.expect(t, "Audio.SwitchModel:small", "Config.Model:small", "Audio.SwitchModel:base",
		"UI.Notify:Model Switch Failed")
}

func TestSettingsActions(t *testing.T) {
	f := newActionFixture()
	_ = f.c.setLanguage("ru")
	_ = f.c.setOutputMode("clipboard")
	_ = f.c.setRecordingMethod("ffmpeg")
	_ = f.c.toggleNotifications()
	f.expect(t,
		"Config.Language:ru", "UI.UpdateSettings",
		"Config.OutputMode:clipboard", "IO.SetOutputMethod:clipboard", "UI.UpdateSettings",
		"Config.RecordingMethod:ffmpeg", "Audio.ClearSession", "UI.UpdateSettings",
		"Config.ToggleNotifications", "UI.Notify:Workflow Notifications",
	)
}

func TestRebindHotkey(t *testing.T) {
	f := newActionFixture()
	if err := f.c.rebindHotkey("start_recording"); err != nil {
		t.Fatal(err)
	}
	f.expect(t, "UI.Notify:Rebind Hotkey", "Hotkeys.Capture", "UI.Notify:Rebind Hotkey")

	f = newActionFixture()
	f.hotkeys.Captured = "ctrl+space"
	if err := f.c.rebindHotkey("start_recording"); err != nil {
		t.Fatal(err)
	}
	f.expect(t, "UI.Notify:Rebind Hotkey", "Hotkeys.Capture",
		"Config.Hotkey:start_recording=ctrl+space", "Hotkeys.Reload:alt+r", "UI.UpdateSettings", "UI.Notify:Hotkey Updated")
}
