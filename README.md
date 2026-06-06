<div align="center">
<img src="https://github.com/AshBuk/dabri/raw/master/icons/io.github.ashbuk.dabri-logo.svg" width="300" alt="Dabri"/>
  
> 🗣️ **Linux speech-to-text, the Unix way**

<p style="margin-bottom: 12px;">
<a href="https://pkg.go.dev/github.com/AshBuk/dabri/v2">
  <img src="https://pkg.go.dev/badge/github.com/AshBuk/dabri/v2.svg" alt="Go Reference">
</a>
<a href="https://goreportcard.com/report/github.com/AshBuk/dabri/v2">
  <img src="https://goreportcard.com/badge/github.com/AshBuk/dabri/v2" alt="Go Report Card">
</a>
<a href="https://go.dev/">
  <img src="https://img.shields.io/badge/go-1.25-00ADD8?logo=go" alt="Go Version">
</a>
</p>
  
[![CI](https://github.com/AshBuk/dabri/actions/workflows/ci.yml/badge.svg)](https://github.com/AshBuk/dabri/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/AshBuk/dabri?sort=semver)](https://github.com/AshBuk/dabri/releases)
[![AppImage](https://img.shields.io/badge/AppImage-available-0a7cff?logo=appimage)](https://github.com/AshBuk/dabri/releases)
[![AUR](https://img.shields.io/aur/version/dabri?logo=archlinux)](https://aur.archlinux.org/packages/dabri)
[![COPR](https://img.shields.io/badge/COPR-Fedora-51a2da?logo=fedora)](https://copr.fedorainfracloud.org/coprs/ashbuk/dabri/)

</div>

**Dabri** is a **minimalist**, **privacy-first** application for **offline voice recognition** directly into any active window (editors, browsers, IDEs, AI assistants).  

Written in pure **[Go](https://github.com/golang/go)**, it leverages **[whisper.cpp](https://github.com/ggerganov/whisper.cpp)** for fast, offline transcription.
The architecture is built from the ground up with a *minimal set of dependencies*, featuring a **custom dependency injection factory**, ensuring **lean and maintainable** design.

https://github.com/user-attachments/assets/b798f350-c6a8-4408-83eb-51eef392a731

## Features

[![Privacy](https://img.shields.io/badge/Privacy-Offline-blueviolet)](#-features)
[![Security](https://snyk.io/test/github/AshBuk/dabri/badge.svg)](https://snyk.io/test/github/AshBuk/dabri)
[![gosec](https://img.shields.io/badge/gosec-passing-brightgreen)](https://github.com/AshBuk/dabri/security/code-scanning)

▸ **<img src="icons/io.github.ashbuk.dabri.png" width="18" height="18" align="absmiddle">aemon:** Dabri runs quietly in the background and integrates into the system tray for convenient management.

▸ **Terminal:** it can also be invoked as a CLI tool (see **[CLI Usage Guide](docs/CLI_USAGE.md)**) for scripting purposes. 

▸ For integration enthusiasts, a WebSocket server is available at `localhost:8080`. Enable it in your config with web_server enabled: true (disabled by default).

- **Offline speech-to-text, privacy-first**: all processing happens locally
- **Portable**: AppImage package
- **Cross-platform support** for X11 and Wayland
- **Linux DEs**: native integration with GNOME, KDE, and others
- **GPU + CPU support**: Vulkan backend for faster transcription (auto-fallback to CPU)
- **Voice typing or clipboard mode**
- **Flexible audio recording**: arecord (ALSA) or ffmpeg (PulseAudio/PipeWire), see [audio pipeline](docs/AUDIO_PIPELINE_DIAGRAM.txt)
- **Multi-language support, custom hotkey binding, visual notifications**
- **Model management**: switch between base, small, medium, large-v3-turbo, and large-v3 whisper models via tray or CLI

## Beyond Minimalism

Intuitive minimalist UX, **robust STT infrastructure**. A foundation for voice-controlled automation:

- **Dual API**: Unix socket IPC + WebSocket — script locally or integrate remotely
- **Interface-driven**: focused contracts — swap STT engines, add I/O methods, extend hotkey providers
- **Daemon + CLI**: background hub + stateless commands — perfect for IoT pipelines
- **Graceful degradation**: provider fallbacks, optional components, no crashes

```bash
# Voice command → smart home action
transcript=$(dabri --json stop | jq -r '.data.transcript')
[[ "$transcript" == *"lights off"* ]] && curl -X POST http://hub/lights/off
```

## ✦ Installation

### AppImage

Download the latest AppImage from [Releases](https://github.com/AshBuk/dabri/releases):

```bash
# Download the file, then:
chmod +x dabri-*.AppImage
./dabri-*.AppImage
```

### Arch Linux [AUR](https://aur.archlinux.org/packages/dabri):
```bash
yay -S dabri
```

### Fedora [COPR](https://copr.fedorainfracloud.org/coprs/ashbuk/dabri/):
```bash
sudo dnf copr enable ashbuk/dabri
sudo dnf install dabri
```

## Desktop Environment Compatibility

[![OS](https://img.shields.io/badge/OS-Linux-34a853?logo=linux)](#-system-requirements)
[![Display](https://img.shields.io/badge/Display-Wayland%20%2F%20X11-ff69b4)](#-features)

📋 **[Desktop Environment Support Guide](docs/Desktop_Environment_Support.md)** - help us test different desktop environments!

**For system tray integration on GNOME — [install the AppIndicator extension](docs/Desktop_Environment_Support.md#for-system-tray-on-gnome---to-have-full-featured-ux-with-menu) ↑**
> KDE and other DEs have built-in system tray support out of the box

**Automatic typing and clipboard modes both work out of the box — [details & options](docs/Desktop_Environment_Support.md#text-output-status-outputter-for-automatic-text-insertion-into-active-window) ↑**  

For issues and bug reports: [GitHub Issues](https://github.com/AshBuk/dabri/issues)

See changes: [Releases](https://github.com/AshBuk/dabri/releases)

### System Requirements

| Category | Requirement |
|----------|-------------|
| **OS** | Linux with glibc 2.35+ |
| **Desktop** | X11 or Wayland |
| **Audio** | Microphone capability |
| **Storage** | ~54MB + model (57MB–1.1GB) |
| **Memory** | ~300MB RAM |
| **CPU** | AVX-capable (Intel/AMD 2011+) |

<details>
<summary><b>📋 Supported Distributions</b></summary>

| Family | Distributions |
|--------|---------------|
| **Ubuntu-based** | Ubuntu 22.04+, Linux Mint 21+, Pop!_OS 22.04+, Elementary OS 7+, Zorin OS 17+ |
| **Debian-based** | Debian 12+ |
| **Fedora** | Fedora 36+ |
| **Rolling release** | Arch Linux, Manjaro, EndeavourOS, openSUSE Tumbleweed |

</details>


## For Developers

Start onboarding with:

- [ARCHITECTURE.md](docs/ARCHITECTURE.md) — system architecture and component design
- [DEVELOPMENT.md](docs/DEVELOPMENT.md) — development workflow and build instructions
- [CONTRIBUTING.md](docs/CONTRIBUTING.md) — contribution guidelines and how to help improve the project
- [docker/README.md](docker/README.md) — Docker-based development

Technical dive into architecture and engineering challenges: [Building Dabri on Hashnode](https://ashbuk.hashnode.dev/an-offline-voice-to-text-solution-for-linux-users-using-whispercpp-and-go)

## ✦ Acknowledgments

- [whisper.cpp](https://github.com/ggerganov/whisper.cpp) for the excellent C++ implementation of OpenAI Whisper
- [fyne.io/systray](https://github.com/fyne-io/systray) for cross-platform system tray support
- [ydotool](https://github.com/ReimuNotMoe/ydotool) and [wtype](https://github.com/atx/wtype) for Wayland-compatible input automation
- OpenAI for the original Whisper model

## ✦ MIT [LICENSE](LICENSE)

If you use this project, please link back to this repo and ⭐ it if it helped you.
- Consider contributing back improvements

---

Sharing with the community for privacy-conscious Linux users

---

## Sponsor

[![Sponsor](https://img.shields.io/badge/Sponsor-💖-pink?style=for-the-badge&logo=github)](https://github.com/sponsors/AshBuk) [![PayPal](https://img.shields.io/badge/PayPal-00457C?style=for-the-badge&logo=paypal&logoColor=white)](https://www.paypal.com/donate/?hosted_button_id=R3HZH8DX7SCJG)

Please consider supporting development
