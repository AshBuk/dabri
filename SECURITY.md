# Security Policy

## Supported Versions

Only the latest released version of Dabri receives security fixes.

## Reporting a Vulnerability

Please report security issues privately via GitHub's [**Report a vulnerability**](https://github.com/AshBuk/dabri/security/advisories/new) form (Security tab → Advisories) rather than opening a public issue. If GitHub is not an option for you, you can email **asherbuk@gmail.com** instead.

Include, if possible:

- A description of the issue and its impact
- Steps to reproduce (or a proof of concept)
- The affected version and Linux distribution

You can expect an initial response within **7 days**. Once the issue is confirmed, a fix will be prepared and released as soon as reasonably possible, and you will be credited in the release notes unless you prefer to stay anonymous.

## Scope

Dabri is an offline Linux desktop application: speech recognition runs locally via whisper.cpp, and no audio or transcripts are sent over the network. Reports most relevant to this project include:

- Path traversal or arbitrary file read/write via model paths or configuration (`~/.config/dabri/config.yaml`)
- Privilege or input-device issues in the hotkey layer (evdev, D-Bus)
- Unsafe handling of recorded audio buffers or temporary files
- Issues in the local WebSocket server exposed by the daemon

Out of scope: vulnerabilities in upstream dependencies (whisper.cpp, Go modules — please report those to the respective projects) and issues that require prior local root access.
