# Main Window — Specification (Draft)

> Status: **initial draft / sketch**. Captures the goal, scope decision, layout, and
> integration points for Dabri's main window. Refine as implementation lands.

## 1. Goal

Add a small, useful main window so Dabri is no longer tray-only. Two drivers:

1. **Usability** — the app must be controllable on GNOME (and any environment)
   where the system tray is unavailable without an AppIndicator extension.
2. **Flathub** — a tray-only app has no real UI to screenshot. Flathub review
   expects screenshots of the application's *own interface*, not a logo or a
   shell-rendered tray menu. The window produces a legitimate screenshot as a
   natural by-product.

The window is a **control surface**, not a welcome stub and not a full settings
editor: current state, the few most-used controls, and clear primary actions.

## 2. Design principles

Dabri stays a lightweight, unobtrusive, tray-first app. The window must not
betray that.

- **The window never shows itself.** It appears only on explicit user action, or
  automatically when there is no tray to fall back to. No `show()`/`present()` on
  errors, recording start, or transcription completion — those go through
  notifications and the tray icon.
- **No focus stealing.** When opened from the tray, use a plain `present()` —
  no `keep-above`, no aggressive raise.
- **One screen, no scroll, no tabs.** If a feature needs a tab or a scrollbar, it
  belongs in the config file, not the window. This is the line that keeps the
  window from drifting into a "settings for everything" panel.

## 3. Build scope

**Ship the window in all builds** (AppImage, Flatpak, Arch, Fedora) — not gated
to Flatpak.

Rationale:
- The "no tray on GNOME" problem is identical across every package format. Gating
  to Flatpak would leave non-Flatpak users on bare GNOME with no way to reach the app.
- No new dependency burden anywhere: GTK3 is already present wherever the tray
  works (`libayatana-appindicator` depends on GTK3), and it is in the GNOME
  runtime for Flatpak.
- The real branching axis is **tray available vs not, decided at runtime**, not
  package type. Adding build tags (`//go:build flatpak`) would create divergent
  behavior, a larger test matrix, and "the window won't open for me" bug reports.

Branch on environment only at the narrow points that actually leave the sandbox
(e.g. opening the config file or an external link), via `platform.IsFlatpak()` /
`/.flatpak-info` — never for the feature as a whole.

## 4. Technology

**gotk3** (GTK3 bindings for Go). GTK3 is guaranteed where the tray works and is
included in the GNOME runtime — no new system dependency is introduced.

The window runs on the GTK main loop. All UI mutations must be marshalled onto the
GTK thread (`glib.IdleAdd`) since recording/transcription callbacks fire from
other goroutines.

## 5. Layout (sketch)

```
┌────────────────────────────────────┐
│  [icon] Dabri                 – ×   │
├────────────────────────────────────┤
│                                    │
│  Status     ● Ready                │
│                                    │
│  Model      Small (Q5_1)      ▼    │
│  Output     Type to window    ▼    │
│  Hotkey     Ctrl+Alt+Space         │
│                                    │
│   ┌──────────────────────────┐     │
│   │  🎤  Start Listening      │     │
│   └──────────────────────────┘     │
│                                    │
│                 [ Run in Background ]│
└────────────────────────────────────┘
```

- `[icon]` — app icon via `gtk.Image` (`io.github.ashbuk.dabri`). Window title is
  still set for taskbar / alt-tab.
- `🎤` — `audio-input-microphone-symbolic` when idle, `media-playback-stop-symbolic`
  while recording.

## 6. Elements and capabilities

| Element | Type | Capability | Backed by |
|---|---|---|---|
| **Status** | dot + label | Live state: `Ready` / `Recording...` / `Transcribing...` / `Error` | `constants.MsgReady` / `MsgRecording` / `MsgTranscribing`, driven via UI service |
| **Model** | ComboBox | Switch active model; auto-download if absent | `constants.WhisperModels`, `Services.Audio.SwitchModel(ctx, id)` |
| **Output** | ComboBox | `Type to window` / `Clipboard` | `Services.Config.UpdateOutputMode(mode)` |
| **Hotkey** | read-only label | Show current binding (edit via config file) | `cfg.Hotkeys.StartRecording` via `Services.Config.GetConfig()` |
| **Start / Stop** | primary button | Toggle capture; label + icon flip on state | `Services.Audio.HandleStartRecording / HandleStopRecording`, `IsRecording()` |
| **Run in Background** | secondary button | Hide window, keep process alive | window manager |

The window updates only on discrete state changes (`SetState`), never on a
continuous stream — see §11.3 on why the VU meter is deliberately excluded.

Model switching shows progress/disabled state while a download runs and rolls the
selection back on failure (the IPC/tray path already does this rollback — reuse it).

## 7. Behavior

| Situation | Behavior |
|---|---|
| Launch, tray available | Window hidden, tray icon shown |
| Launch, tray unavailable | Window shown automatically |
| First launch ever (no config yet) | Show window once even if tray exists, so first-run users see the app |
| Click tray icon | Toggle window visibility |
| Click **Run in Background** | Hide window, keep process alive |
| Close window (×), tray available | Hide to tray (do not exit) |
| Close window (×), no tray | Exit process |
| Recording / transcription events | Update widgets in place; **never** raise or focus the window |

If the tray is unavailable on GNOME, show a non-modal banner inside the window
with a one-line instruction to install the AppIndicator / KStatusNotifierItem
extension and a link to `docs/Desktop_Environment_Support.md`.

## 8. Modern look

Aim for clean and native, not custom-skinned:

- Lean on GTK3 + Adwaita defaults; respect the user's light/dark theme. Do not
  hardcode colors.
- Use symbolic freedesktop icons (they recolor with the theme).
- Generous, consistent padding; a single vertical content column; one clear
  primary action (the Start/Stop button gets `suggested-action` styling).
- Keep a small, mostly-fixed window size — it is a control surface, not a
  resizable workspace.
- Optional light polish via a small embedded CSS provider (spacing, the status
  dot color: green Ready / red Recording / amber Transcribing). Keep it minimal
  so theming still wins.

## 9. Flathub / metainfo follow-up

Once the window exists, update `io.github.ashbuk.dabri.appdata.xml`:

- Replace the logo screenshot with real screenshots of the window.
- Store images in-repo (e.g. `docs/screenshots/main-window.png`) and reference
  them by raw URL, as today.
- Provide 1–2 screenshots (idle / recording state) with accurate captions.

Note: `flatpak-builder-lint` only checks that a screenshot *exists*; the
requirement that it depicts real UI is a review guideline. There is no automated
"is a window open" runtime test — the window is needed for genuine usability and
a legitimate screenshot, not to satisfy a checker.

## 10. Out of scope

- Full settings editor / editing all config fields from the window
- Advanced model management UI (download queue, deletion)
- Replacing the existing tray menu UX
- In-window hotkey rebinding (config file + existing capture flow stays)

## 11. Resolved decisions

### 11.1 Window controller location

UI backends live under an **`internal/ui/`** umbrella: `internal/ui/tray`
(systray) and `internal/ui/window` (gotk3, isolated inside the package).
`UIService` keeps a `window.Manager` alongside the `tray.Manager` and acts as the
**fan-out point**: a single `SetRecordingState` / `UpdateRecordingUI` call routes
to *both* tray and window. (Naming follows the project convention — `Manager`, as
with `tray.Manager` / `NotifyManager`.)

- `UIServiceInterface` is **not** widened — audio/app callers keep calling the
  same methods; only `UIService`'s internals change.
- Wiring follows the existing factory path: `factory_components.go` builds the
  window controller (`createWindowManager`, like `createTrayManager`),
  `factory_assembler.go` (`createUIService`) injects it.
- Backend selection mirrors the tray: a `gtk` build tag picks the gotk3 backend;
  without it a no-op backend keeps headless/CI builds GTK-free.

**Lifecycle / threading.** `fyne.io/systray` on Linux is DBus-based
(StatusNotifierItem, pure-Go godbus) and already runs in its own goroutine
(`go systray.Run(...)`), so there is no existing GTK loop to share and no thread
pinning from the tray. gotk3 therefore **owns the main OS thread**: under the
`gtk` build, `main()` calls `runtime.LockOSThread()`, `App.RunAndWait` delegates
the blocking loop to `Controller.Run()` (which runs `gtk.Main()`), and the
shutdown-signal `select` moves to a goroutine that calls `Controller.Quit()` via
`glib.IdleAdd`. All window mutations are marshalled with `glib.IdleAdd`. The
no-op backend's `Run()` returns immediately, so the non-GUI lifecycle is
unchanged.

**Status — Phase 1 + Phase 2 landed:**
- Phase 1: `internal/ui/window` (`Manager` interface + no-op backend), tray moved
  to `internal/ui/tray`, `UIService` fan-out, factory wiring.
- Phase 2: gotk3 backend in `gtk.go` behind `//go:build gtk` (gotk3, GTK3 widgets,
  `glib.IdleAdd` marshalling); `Options`/`SetActions` for config snapshot and
  callback wiring; actions wired in `FactoryWirer` (toggle/model/output/quit);
  `ServiceContainer.Window` owns the main loop; `RunAndWait` delegates the
  blocking loop to `Window.Run()` with a watcher goroutine calling `Window.Quit()`
  on signal; `cmd/dabri/lockthread_gtk.go` pins the main thread under `gtk`.
  Builds: default (no GTK) and `-tags gtk` both compile/link; tests green.

> gotk3: pinned to a post-v0.6.4 master pseudo-version because v0.6.4's gdk
> package fails to build against GTK ≥3.22 (missing `internal/callback` import in
> `Seat.Grab`).

**Remaining (packaging):** the release builds must pass `-tags gtk` (alongside
`systray`) for the window to ship — Makefile and the release workflows still need
that flag. Visual/behavioral QA on a real GTK session is also pending (the loop
and widgets are compile- and unit-verified, not yet run on a display).

### 11.2 First-launch detection

**Absence of the config file** is the signal — no `firstRun` flag needed.
`LoadConfig` returns in-memory defaults *without writing* when the file is missing
(`config/loaders/yaml_loader.go`), and the file is created lazily on first
`ShowConfigFile` / `SaveConfig`. So: file missing at startup ⇒ first run ⇒ show
the window once even if a tray exists. After the user changes anything, the file
appears and later launches behave normally.

### 11.3 No VU meter (deliberately excluded)

The VU meter is **out of v1.** It would be the only widget fed by a continuous
stream (mic level at ~real-time), forcing the GTK loop to repaint many times per
second — runtime load this previously-headless daemon never had, for marginal
value on a control surface.

Instead the window updates **only on discrete state changes** via `SetState`
(`Ready` / `Recording` / `Transcribing` / `Error`). `UIService.UpdateRecordingUI`
keeps ignoring `level` (as it does today for the tray), and `window.Manager` has
no `SetLevel`. The GTK loop is then idle whenever the window isn't being
interacted with.

If a live level indicator is ever wanted, it can be reintroduced behind the same
`UpdateRecordingUI(level)` path (the level already flows there from
`audio_service.go`), but it should be opt-in, throttled, and active only while the
window is visible.
