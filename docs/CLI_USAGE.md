# CLI Usage Guide

Dabri provides a dual-mode binary that works as both a background daemon and a command-line interface. This is useful for custom scripts and tiling WMs users.

## Quick Start

**Launch the daemon** (run once, keeps running in background):
```bash
dabri                          # Launch daemon in background
```

**Use CLI commands** (while daemon is running):
```bash
dabri --version                # Show version
dabri start                    # Begin recording
dabri stop                     # Stop and show transcript
dabri toggle                   # Toggle recording (start/stop with one command)
dabri status                   # Show state and configuration
dabri transcript               # Show last transcript

# Model selection
dabri model list               # List available models (works without daemon)
dabri model set <model-id>     # Switch model (requires daemon)
dabri model set base-q5_1      # ~57 MB, fast
dabri model set small-q5_1     # ~181 MB, default
dabri model set medium-q5_0    # ~539 MB
dabri model set large-v3-turbo-q5_0 # ~820 MB, faster large-v3 variant
dabri model set large-v3-q5_0       # ~1.1 GB, best quality
dabri model set parakeet-tdt-0.6b-v3-q8_0 # ~669 MB, NVIDIA Parakeet, 25 European languages, ignores the language setting
dabri model delete <model-id>  # Delete a downloaded model (cannot delete active)
```

**Notes:**
- `toggle` is ideal for binding to a single DE shortcut — one key to start and stop recording
- Transcript is printed to stdout
- If using `active_window` output mode, text is also typed into the active window
- To suppress duplicate output: `dabri stop >/dev/null`

---

## CLI Flags

### `--socket <path>`
Specify custom IPC socket path.

```bash
dabri --socket /tmp/custom.sock start     # Custom socket path
```

**Default:** `$XDG_RUNTIME_DIR/dabri.sock`

---

### `--json`
Output responses in JSON format for scripting.

```bash
dabri --json status                       # JSON output
```

---

### `--timeout <seconds>`
Override default timeout for the command.

```bash
dabri --timeout 120 stop                  # 120 second timeout
```

**Default timeouts:**
- `stop`, `toggle`: 60 seconds (transcription can take time)
- Other commands: 5 seconds

---

## Daemon Flags

When running as daemon (without CLI command):

### `--config <path>`
Specify custom configuration file path.

```bash
dabri --config ~/.config/dabri/custom.yaml    # Custom config
```

**Default path:** `~/.config/dabri/config.yaml`

---

### `--debug`
Enable debug logging.

```bash
dabri --debug                             # Debug mode
```

---

## AppImage CLI Usage

AppImage requires the full path to run CLI commands (e.g. `./dabri-x.x.x-x86_64.AppImage status`).
For a shorter command, create a symlink:
```bash
ln -sf /path/to/dabri-x.x.x-x86_64.AppImage ~/.local/bin/dabri
```
## Flatpak CLI Usage

Run CLI commands through `flatpak run`, e.g. `flatpak run io.github.ashbuk.dabri toggle`.
The daemon socket lives inside the sandbox, so a host `dabri` binary can't reach a Flatpak daemon.
For a shorter command, add an alias to your shell config:
```bash
alias dabri='flatpak run io.github.ashbuk.dabri'
```
Use the full `flatpak run io.github.ashbuk.dabri toggle` command for DE/WM shortcuts, since they don't read shell aliases.
The Flatpak config file is `~/.var/app/io.github.ashbuk.dabri/config/dabri/config.yaml`.
