// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

package outputters

import (
	"errors"
	"testing"

	"github.com/AshBuk/go-wlportal/typing"
)

func TestPortalOutputter_TypeToActiveWindow_ASCIIUsesPortalTyping(t *testing.T) {
	kbd := &fakePortalKeyboard{}
	clipboard := &fakePortalClipboard{}
	outputter := &PortalOutputter{kbd: kbd, clipboard: clipboard}

	if err := outputter.TypeToActiveWindow("hello world"); err != nil {
		t.Fatalf("TypeToActiveWindow returned error: %v", err)
	}
	if kbd.typed != "hello world" {
		t.Fatalf("typed = %q, want %q", kbd.typed, "hello world")
	}
	if clipboard.copied != "" {
		t.Fatalf("clipboard copied = %q, want empty", clipboard.copied)
	}
	if len(kbd.combos) != 0 {
		t.Fatalf("key combos = %v, want none", kbd.combos)
	}
}

func TestPortalOutputter_TypeToActiveWindow_NonASCIIUsesClipboardAndPortalPaste(t *testing.T) {
	kbd := &fakePortalKeyboard{}
	clipboard := &fakePortalClipboard{}
	outputter := &PortalOutputter{kbd: kbd, clipboard: clipboard}

	if err := outputter.TypeToActiveWindow("Здарова! Это тест!"); err != nil {
		t.Fatalf("TypeToActiveWindow returned error: %v", err)
	}
	if kbd.typed != "" {
		t.Fatalf("typed = %q, want empty", kbd.typed)
	}
	if clipboard.copied != "Здарова! Это тест!" {
		t.Fatalf("clipboard copied = %q, want input text", clipboard.copied)
	}
	if len(kbd.combos) != 1 {
		t.Fatalf("key combos count = %d, want 1", len(kbd.combos))
	}
	want := []typing.Keycode{typing.KeycodeLeftShift, typing.KeycodeInsert}
	if !sameKeycodes(kbd.combos[0], want) {
		t.Fatalf("key combo = %v, want %v", kbd.combos[0], want)
	}
}

func TestPortalOutputter_TypeToActiveWindow_NonASCIIClipboardError(t *testing.T) {
	wantErr := errors.New("copy failed")
	kbd := &fakePortalKeyboard{}
	clipboard := &fakePortalClipboard{copyErr: wantErr}
	outputter := &PortalOutputter{kbd: kbd, clipboard: clipboard}

	if err := outputter.TypeToActiveWindow("тест"); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if len(kbd.combos) != 0 {
		t.Fatalf("key combos = %v, want none", kbd.combos)
	}
}

func TestPortalOutputter_TypeToActiveWindow_NonASCIIPasteError(t *testing.T) {
	wantErr := errors.New("paste failed")
	kbd := &fakePortalKeyboard{comboErr: wantErr}
	clipboard := &fakePortalClipboard{}
	outputter := &PortalOutputter{kbd: kbd, clipboard: clipboard}

	if err := outputter.TypeToActiveWindow("тест"); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if clipboard.copied != "тест" {
		t.Fatalf("clipboard copied = %q, want input text", clipboard.copied)
	}
	if len(clipboard.copiedHistory) != 1 {
		t.Fatalf("clipboard history = %v, want only paste text", clipboard.copiedHistory)
	}
}

func TestPortalOutputter_GetToolNamesIncludesClipboard(t *testing.T) {
	outputter := &PortalOutputter{
		kbd:       &fakePortalKeyboard{},
		clipboard: &fakePortalClipboard{clipboardTool: "wl-copy"},
	}

	clipboardTool, typeTool := outputter.GetToolNames()
	if clipboardTool != "wl-copy" {
		t.Fatalf("clipboard tool = %q, want wl-copy", clipboardTool)
	}
	if typeTool != "portal" {
		t.Fatalf("type tool = %q, want portal", typeTool)
	}
}

type fakePortalKeyboard struct {
	typed    string
	typeErr  error
	combos   [][]typing.Keycode
	comboErr error
}

func (f *fakePortalKeyboard) Type(text string) error {
	f.typed = text
	return f.typeErr
}

func (f *fakePortalKeyboard) KeyCombo(keycodes ...typing.Keycode) error {
	f.combos = append(f.combos, append([]typing.Keycode(nil), keycodes...))
	return f.comboErr
}

type fakePortalClipboard struct {
	copied        string
	copiedHistory []string
	copyErr       error
	clipboardTool string
}

func (f *fakePortalClipboard) CopyToClipboard(text string) error {
	f.copied = text
	f.copiedHistory = append(f.copiedHistory, text)
	return f.copyErr
}

func (f *fakePortalClipboard) TypeToActiveWindow(text string) error {
	return errors.New("typing not supported")
}

func (f *fakePortalClipboard) GetToolNames() (clipboardTool, typeTool string) {
	return f.clipboardTool, ""
}

func sameKeycodes(a, b []typing.Keycode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
