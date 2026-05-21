## 🖥️ Desktop Environment Support

> Tested on major Linux distributions (Fedora, Ubuntu, Arch Linux) and tiling window managers (Hyprland, Sway)

> If you encounter issues with your desktop environment, feel free to [open an issue](https://github.com/AshBuk/dabri/issues). 

### **For system tray on GNOME - to have full-featured UX with menu**:
```bash
# Ubuntu/Debian:
sudo apt install gnome-shell-extension-appindicator
# Fedora:
sudo dnf install gnome-shell-extension-appindicator
# Arch Linux:
sudo pacman -S gnome-shell-extension-appindicator
```
*KDE and other DEs have built-in system tray support, no need for appindicator*

### **Text output status (outputter, for automatic text insertion into active window)**

**Current Implementation: Smart Auto-Selection**

| Desktop Environment | Primary Tool | Fallback | Status |
|---------------------|--------------|----------|--------|
| **🟢 GNOME+Wayland** | ydotool | clipboard | ⚠️ Requires setup |
| **🟢 KDE+Wayland** | wtype → ydotool | clipboard | ✅ Auto-detected |
| **🟢 Sway/Other Wayland** | wtype → ydotool | clipboard | ✅ Auto-detected |
| **🟢 X11 (all DEs)** | xdotool | clipboard | ✅ Works out-of-box |

 GNOME/Wayland requires ydotool setup. 
 
 Other Wayland compositors (KDE, Sway, etc.) works with wtype out of the box.

## Direct typing on Wayland - Tool options

The application automatically selects the best available typing tool:
- **wtype**: Works without setup on non-GNOME Wayland compositors (KDE, Sway, etc.). Automatically selected if available.
- **ydotool**: Required for GNOME/Wayland, also works as fallback on other Wayland compositors. Requires setup (see below).

### ydotool setup (recommended user-unit)

> 1) Install ydotool:
```bash
sudo dnf install ydotool   # Fedora
sudo apt install ydotool   # Ubuntu/Debian
```
> 2) Allow access to /dev/uinput for non-root:
```bash
echo 'KERNEL=="uinput", GROUP="input", MODE="0660"' | sudo tee /etc/udev/rules.d/99-uinput.rules
sudo udevadm control --reload && sudo udevadm trigger
sudo usermod -a -G input $USER
# Re-login required for group change
```
> 3) Run ydotool as user-unit service (no root):
```bash
mkdir -p ~/.config/systemd/user
tee ~/.config/systemd/user/ydotool.service >/dev/null <<'EOF'
[Unit]
Description=ydotool user daemon

[Service]
ExecStart=/usr/bin/ydotoold --socket-perm=0660
Restart=always

[Install]
WantedBy=default.target
EOF
```
> 4) Restart and run the service
```bash
systemctl --user daemon-reload
systemctl --user enable --now ydotool
```
*This setup uses user service: safer and no root privileges needed*

*For non-GNOME Wayland compositors, wtype work without any setup - the app will automatically try it first*

*X11 works out-of-the-box without additional setups*

**Clipboard fallback**
- Works on **all** desktop environments  
- Requires manual `Ctrl+V` after speech recognition
- No additional setup needed

## ⌨️ **Hotkeys**

By default, Dabri uses the **D-Bus GlobalShortcuts portal** — the Wayland-native sandboxed approach. No setup required on GNOME and KDE. Rebind via *Settings → Keyboard Shortcuts*.

**Hyprland:** The portal registers shortcuts, but Hyprland requires explicit binding in `hyprland.conf`. Run `hyprctl globalshortcuts` while Dabri is running to see the registered IDs, then add:
```
bind = <mods>, <key>, global, <appid>:<shortcutid>
```

### **Optional: evdev**

The classic X11 approach — direct raw input access. Opt in if:
- Your WM/DE doesn't implement XDG GlobalShortcuts (i3, bspwm, openbox, etc.)
- You want to rebind hotkeys from the **Dabri tray menu** directly
- Portal behavior is inconsistent on your setup

**Trade-off:** requires access to all input devices (`/dev/input/event*`), not just keyboard.

**Option A — udev rule (scoped to session user):**
```bash
echo 'KERNEL=="event*", SUBSYSTEM=="input", ATTRS{capabilities/key}!="0", TAG+="uaccess"' \
  | sudo tee /etc/udev/rules.d/70-dabri-input.rules
sudo udevadm control --reload && sudo udevadm trigger
```

**Option B — input group (broader access):**
```bash
sudo usermod -a -G input $USER  # then logout/login
```

Then enable in `~/.config/dabri/config.yaml`:
```yaml
hotkeys:
  provider: evdev
```

### **Direct Hotkey Binding via DE/WM**
Bind `dabri toggle` to any key via your DE settings or WM config to start/stop recording:
- **GNOME:** Settings → Keyboard → Custom Shortcuts → `dabri toggle`
- **KDE:** System Settings → Shortcuts → Custom Shortcuts → `dabri toggle`
- **Tiling WMs** (i3, sway, Hyprland, bspwm, etc.):
  ```
  bindsym $mod+r exec dabri toggle
  ```
- **Separate start/stop** also available: `dabri start` / `dabri stop`
- See [CLI Usage Guide](CLI_USAGE.md) for command reference

## Autostart on Login

If you want Dabri ready as soon as you log in (without launching app manually), add to your session startup:

### GNOME / KDE / XFCE (XDG autostart)
```bash
mkdir -p ~/.config/autostart
cat > ~/.config/autostart/dabri.desktop << 'EOF'
[Desktop Entry]
Name=Dabri
Exec=dabri
Icon=io.github.ashbuk.dabri
Type=Application
Terminal=false
X-GNOME-Autostart-enabled=true
EOF
```

### Hyprland
Add to `~/.config/hypr/hyprland.conf`:
```
exec-once = dabri
```

*Last updated: 2026-05-21*