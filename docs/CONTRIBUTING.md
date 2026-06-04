# Contributing to Dabri

Thank you for your interest to Dabri!

## Before You Start

- **For features** (not bug fixes or small PRs): please open an issue first to discuss the idea before sending a pull request.
- **Stay in scope:** any new contribution must fit the scope and goals of the project.
- AI-assisted contributions accepted **if author understands and can defend the code**.

## 🐛 Bug Reports

When reporting bugs, include:

- Create an issue
- Operating system and version
- Desktop environment (GNOME, KDE, etc.)
- Display server (X11/Wayland)
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR-USERNAME/dabri.git`
3. Follow the development setup in [DEVELOPMENT.md](https://github.com/AshBuk/dabri/blob/master/docs/DEVELOPMENT.md)

4. **Dev Workflow**
   1. Create a feature branch: `git checkout -b feature/your-feature-name`
   2. Make your changes
   3. Add license headers to new Go files
   ```go
   // Copyright (c) 2025 Asher Buk
   // SPDX-License-Identifier: MIT
   ```
   4. Commit with clear message
   5. Push and create a Pull Request

5. **Code Style**

Before opening a PR, make sure your changes are formatted, linted, tested, and build cleanly. See the commands and tooling details in [DEVELOPMENT.md](https://github.com/AshBuk/dabri/blob/master/docs/DEVELOPMENT.md).

CI validates formatting, linting, security scanning, tests (with race detector), and license headers. PRs must pass all checks before merge.

## 📜 License

By contributing, you agree that your contributions will be licensed under the MIT License. All contributed code becomes part of the project under the same license terms.
