# BingPaperDesktop

[![Build Verification](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/verify.yml/badge.svg)](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/verify.yml)
[![Release](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/release.yml/badge.svg)](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/release.yml)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/hanxuanyu/BingPaperDesktop)](https://github.com/hanxuanyu/BingPaperDesktop/releases)

English | [简体中文](./README.md)

A cross-platform Bing Daily Wallpaper desktop client developed with Wails v2 + Go + React. It automatically fetches high-definition daily wallpapers from Bing and supports features like previewing, history management, auto-updates, and metadata overlay.

### 🚀 Features

- **Multi-API Mode**: Supports official Bing API and custom APIs.
- **Auto Wallpaper Update**: Supports scheduled tasks (fixed time or interval) to automatically change the desktop wallpaper.
- **Wallpaper History Management**: Browse historical wallpapers, set as current, or delete them.
- **Smart Storage Management**:
    - **Portable Mode**: Prioritizes storing data in the same directory as the application.
    - **Standard Mode**: Automatically switches to the user configuration directory when the application directory is read-only (e.g., inside macOS .app).
- **Metadata Overlay**: Optional overlay of title, copyright, and date on the wallpaper.
- **Cross-Platform Support**: Full support for Windows (10/11), macOS, and Linux (GNOME).

### 📦 Installation & Download

Go to the [Releases](https://github.com/hanxuanyu/BingPaperDesktop/releases) page to download the installer for your platform:

- **Windows**: `.exe` installer or portable standalone version.
- **macOS**: `.dmg` disk image for easy drag-and-drop installation.
- **Linux**: Standalone binary executable.

### 🛠️ Local Development

#### Prerequisites
- [Go](https://go.dev/) (1.23+)
- [Node.js](https://nodejs.org/) (20+)
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

#### Setup Steps
1. Clone the repository:
   ```bash
   git clone https://github.com/hanxuanyu/BingPaperDesktop.git
   cd BingPaperDesktop
   ```
2. Run development mode:
   ```bash
   wails dev
   ```

### 🏗️ Build

You can use the scripts in the `script` directory for quick building:

- **macOS / Linux**:
  ```bash
  ./script/build.sh
  ```
- **Windows (PowerShell)**:
  ```powershell
  .\script\build.ps1
  ```

Build artifacts will be located in the `build/bin` directory.


### 📄 License

This project is licensed under the [MIT License](./LICENSE).
