// Copyright (c) 2025 Asher Buk
// SPDX-License-Identifier: MIT

//go:build gtk

package window

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	configmodels "github.com/AshBuk/dabri/v2/config/models"
	"github.com/AshBuk/dabri/v2/internal/logger"
)

// toggleDebounce drops a second record-button "clicked" that arrives within this
// window of the first. GTK delivers two "clicked" signals on a double-tap, which
// would otherwise start and immediately stop a recording.
const toggleDebounce = 400 * time.Millisecond

// gtkManager is the GTK3 backend. Run owns the calling thread and runs the GTK
// main loop; all widget mutations from other goroutines are marshalled onto that
// thread via glib.IdleAdd.
type gtkManager struct {
	log  logger.Logger
	opts Options

	win         *gtk.Window
	statusLabel *gtk.Label
	modelCombo  *gtk.ComboBoxText
	langCombo   *gtk.ComboBoxText
	outputCombo *gtk.ComboBoxText
	modeCombo   *gtk.ComboBoxText
	startButton *gtk.Button
	trayButton  *gtk.Button

	suppress   bool      // ignore combo "changed" while we set values programmatically
	lastToggle time.Time // GTK-thread only: debounces record-button double-taps

	mu       sync.Mutex // guards actions
	actions  Actions
	quitOnce sync.Once
}

// New returns the GTK window manager, building the widget tree up front.
func New(log logger.Logger, opts Options) Manager {
	m := &gtkManager{log: log, opts: opts}
	gtk.Init(nil)
	if err := m.build(); err != nil {
		log.Error("window: failed to build GTK UI: %v", err)
	}
	return m
}

func (m *gtkManager) Run() error {
	if m.opts.StartVisible && m.win != nil {
		m.win.ShowAll()
	}
	gtk.Main()
	return nil
}

func (m *gtkManager) Quit() {
	m.quitOnce.Do(func() { idle(gtk.MainQuit) })
}

func (m *gtkManager) SetActions(a Actions) {
	m.mu.Lock()
	m.actions = a
	m.mu.Unlock()
}

func (m *gtkManager) Show() { idle(func() { m.show() }) }

func (m *gtkManager) EnableTray() {
	idle(func() {
		m.opts.HasTray = true
		if m.trayButton != nil {
			m.applyTray()
		}
	})
}

func (m *gtkManager) SetState(s State) { idle(func() { m.applyState(s) }) }

func (m *gtkManager) SetModel(id string) {
	idle(func() {
		if m.modelCombo == nil {
			return
		}
		m.suppress = true
		m.modelCombo.SetActiveID(id)
		m.suppress = false
		m.syncLanguage(id)
	})
}

// syncLanguage greys out the language picker for models that detect the language themselves
func (m *gtkManager) syncLanguage(modelID string) {
	for _, mc := range m.opts.Models {
		if mc.ID == modelID {
			m.langCombo.SetSensitive(!mc.DetectsLanguage)
		}
	}
}

func (m *gtkManager) SetLanguage(code string) {
	idle(func() {
		if m.langCombo == nil {
			return
		}
		m.suppress = true
		m.langCombo.SetActiveID(code)
		m.suppress = false
	})
}

func (m *gtkManager) SetOutput(mode string) {
	idle(func() {
		if m.outputCombo == nil {
			return
		}
		m.suppress = true
		m.outputCombo.SetActiveID(mode)
		m.suppress = false
	})
}

func (m *gtkManager) SetHotkeyMode(mode string) {
	idle(func() {
		if m.modeCombo == nil {
			return
		}
		m.suppress = true
		m.modeCombo.SetActiveID(hotkeyModeID(mode))
		m.suppress = false
	})
}

// hotkeyModeID maps a config mode to a combo ID; any mode other than
// push-to-talk is toggle
func hotkeyModeID(mode string) string {
	if mode == configmodels.HotkeyModePTT {
		return configmodels.HotkeyModePTT
	}
	return configmodels.HotkeyModeToggle
}

// build constructs the window and its widgets and connects user actions.
func (m *gtkManager) build() error {
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return err
	}
	m.win = win
	win.SetTitle("Dabri")
	win.SetDefaultSize(360, 0)
	win.SetResizable(false)
	win.SetBorderWidth(16)
	win.Connect("delete-event", func() bool {
		// Close hides to tray when one exists; otherwise it quits the app.
		if m.opts.HasTray {
			m.win.Hide()
			return true
		}
		if a := m.act(); a.OnQuit != nil {
			a.OnQuit()
		}
		return true
	})

	outer, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 12)
	if err != nil {
		return err
	}
	grid, err := m.buildGrid()
	if err != nil {
		return err
	}
	outer.PackStart(grid, false, false, 0)
	if err := m.buildButtons(outer); err != nil {
		return err
	}

	win.Add(outer)
	m.applyState(StateReady)
	return nil
}

// buildGrid lays out the status, settings and hotkey rows.
func (m *gtkManager) buildGrid() (*gtk.Grid, error) {
	grid, err := gtk.GridNew()
	if err != nil {
		return nil, err
	}
	grid.SetRowSpacing(10)
	grid.SetColumnSpacing(12)

	m.statusLabel = m.addRow(grid, 0, "Status", nil)

	m.modelCombo, err = m.newCombo(m.opts.ActiveModel, func(c *gtk.ComboBoxText) {
		for _, mc := range m.opts.Models {
			c.Append(mc.ID, mc.Name)
		}
	}, func(id string) {
		if a := m.act(); a.OnSelectModel != nil {
			go m.run("model switch", func() error { return a.OnSelectModel(context.Background(), id) })
		}
	})
	if err != nil {
		return nil, err
	}
	m.attachRow(grid, 1, "Model", m.modelCombo)

	m.langCombo, err = m.newCombo(m.opts.ActiveLanguage, func(c *gtk.ComboBoxText) {
		for _, l := range m.opts.Languages {
			c.Append(l.Code, l.Name)
		}
	}, func(code string) {
		if a := m.act(); a.OnSelectLanguage != nil {
			go m.run("language switch", func() error { return a.OnSelectLanguage(code) })
		}
	})
	if err != nil {
		return nil, err
	}
	m.attachRow(grid, 2, "Language", m.langCombo)
	m.syncLanguage(m.opts.ActiveModel)

	m.outputCombo, err = m.newCombo(m.opts.OutputMode, func(c *gtk.ComboBoxText) {
		c.Append(configmodels.OutputModeActiveWindow, "Type to window")
		c.Append(configmodels.OutputModeClipboard, "Clipboard")
	}, func(mode string) {
		if a := m.act(); a.OnSelectOutput != nil {
			go m.run("output switch", func() error { return a.OnSelectOutput(mode) })
		}
	})
	if err != nil {
		return nil, err
	}
	m.attachRow(grid, 3, "Output", m.outputCombo)

	m.addRow(grid, 4, "Hotkey", &m.opts.Hotkey)

	m.modeCombo, err = m.newCombo(hotkeyModeID(m.opts.HotkeyMode), func(c *gtk.ComboBoxText) {
		c.Append(configmodels.HotkeyModeToggle, "Press to start/stop")
		c.Append(configmodels.HotkeyModePTT, "Hold to talk")
	}, func(mode string) {
		if a := m.act(); a.OnSelectHotkeyMode != nil {
			go m.run("hotkey mode switch", func() error { return a.OnSelectHotkeyMode(mode) })
		}
	})
	if err != nil {
		return nil, err
	}
	m.attachRow(grid, 5, "Hotkey mode", m.modeCombo)
	return grid, nil
}

// newCombo fills a combo box and selects active before connecting onSelect,
// so only user changes reach it. Programmatic updates set m.suppress.
func (m *gtkManager) newCombo(active string, fill func(*gtk.ComboBoxText), onSelect func(id string)) (*gtk.ComboBoxText, error) {
	combo, err := gtk.ComboBoxTextNew()
	if err != nil {
		return nil, err
	}
	fill(combo)
	combo.SetActiveID(active)
	combo.Connect("changed", func() {
		if id := combo.GetActiveID(); !m.suppress && id != "" {
			onSelect(id)
		}
	})
	return combo, nil
}

// buildButtons adds the record and hide-to-tray buttons.
func (m *gtkManager) buildButtons(outer *gtk.Box) error {
	var err error
	m.startButton, err = gtk.ButtonNewWithLabel("Start Listening")
	if err != nil {
		return err
	}
	if sc, scErr := m.startButton.GetStyleContext(); scErr == nil {
		sc.AddClass("suggested-action")
	}
	m.startButton.Connect("clicked", func() {
		// Runs on the GTK thread, so lastToggle needs no lock. Coalesce a
		// double-tap (GTK emits two "clicked") into a single toggle.
		now := time.Now()
		if now.Sub(m.lastToggle) < toggleDebounce {
			m.log.Info("window: record button click ignored (debounce)")
			return
		}
		m.lastToggle = now
		if a := m.act(); a.OnToggleRecording != nil {
			m.log.Info("window: record button clicked")
			go m.run("toggle recording", a.OnToggleRecording)
		}
	})
	outer.PackStart(m.startButton, false, false, 0)

	m.trayButton, err = gtk.ButtonNew()
	if err != nil {
		return err
	}
	// Insensitive without a tray, so the click only fires in tray mode.
	m.trayButton.Connect("clicked", func() { m.win.Hide() })
	m.applyTray()
	outer.PackStart(m.trayButton, false, false, 0)

	for _, w := range []interface{ SetCanFocus(bool) }{
		m.startButton, m.trayButton,
	} {
		w.SetCanFocus(false)
	}
	return nil
}

// applyTray sets the background button for the current tray availability.
func (m *gtkManager) applyTray() {
	if m.opts.HasTray {
		m.trayButton.SetLabel("Run in Tray")
		m.trayButton.SetTooltipText("Minimizes to the tray icon; click the tray icon to reopen.")
	} else {
		m.trayButton.SetLabel("Need App Indicator")
		m.trayButton.SetTooltipText("No tray icon backend detected. Install the \"AppIndicator and KStatusNotifierItem Support\" GNOME Shell extension (extensions.gnome.org/extension/615) to enable background mode.")
	}
	m.trayButton.SetSensitive(m.opts.HasTray)
}

// addRow attaches a caption and a value label (read-only) on the given grid row.
// When value is nil the value label is created empty and returned for later use.
func (m *gtkManager) addRow(grid *gtk.Grid, row int, caption string, value *string) *gtk.Label {
	text := ""
	if value != nil {
		text = *value
	}
	val, err := gtk.LabelNew(text)
	if err != nil {
		return nil
	}
	val.SetHAlign(gtk.ALIGN_START)
	m.attachRow(grid, row, caption, val)
	return val
}

// attachRow attaches a left caption and a right widget on the given grid row.
func (m *gtkManager) attachRow(grid *gtk.Grid, row int, caption string, right gtk.IWidget) {
	label, err := gtk.LabelNew(caption)
	if err != nil {
		return
	}
	label.SetHAlign(gtk.ALIGN_START)
	grid.Attach(label, 0, row, 1, 1)
	grid.Attach(right, 1, row, 1, 1)
}

func (m *gtkManager) applyState(s State) {
	if m.statusLabel == nil {
		return
	}
	color, text, recording := "green", "Ready", false
	switch s {
	case StateRecording:
		color, text, recording = "red", "Recording…", true
	case StateTranscribing:
		color, text = "orange", "Transcribing…"
	case StateError:
		color, text = "red", "Error"
	}
	m.statusLabel.SetMarkup(fmt.Sprintf("<span foreground=%q>●</span>  %s", color, text))
	if m.startButton != nil {
		if recording {
			m.startButton.SetLabel("Stop Listening")
		} else {
			m.startButton.SetLabel("Start Listening")
		}
	}
}

func (m *gtkManager) show() {
	m.win.ShowAll()
	m.win.Present()
}

// act returns a snapshot of the current actions.
func (m *gtkManager) act() Actions {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.actions
}

// run executes a user action off the GTK thread and logs failures.
func (m *gtkManager) run(what string, fn func() error) {
	if err := fn(); err != nil {
		m.log.Error("window: %s failed: %v", what, err)
	}
}

// idle schedules f on the GTK main loop as a one-shot.
func idle(f func()) {
	glib.IdleAdd(func() bool { f(); return false })
}
