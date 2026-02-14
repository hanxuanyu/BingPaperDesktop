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

- **Windows**: Portable standalone version (`.exe`).
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

> **Tip**: If you are prompted for a password when pushing via command line, it's recommended to [configure SSH](https://docs.github.com/en/authentication/connecting-to-github-with-ssh) or use [GitHub CLI](https://cli.github.com/) for authentication.

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

### 🔐 CI/CD Release Configuration

This project uses GitHub Actions for automated building, signing, and releasing. For the `release` workflow to function correctly, you need to configure the following Secrets in **Settings > Secrets and variables > Actions**:

#### macOS Signing & Notarization (Required)

| Secret Name | Description | How to Obtain |
| :--- | :--- | :--- |
| `APPLE_ID_USER` | **Apple ID Account (Email)** | Your Apple Developer account email. |
| `APPLE_PASSWORD` | **App-Specific Password** | Login to [appleid.apple.com](https://appleid.apple.com/), generate it in "Sign-In and Security". |
| `APPLE_PROVIDER` | **Team ID** | Find it in the [Apple Developer Portal](https://developer.apple.com/account/). |
| `APPLE_IDENTITY` | **Certificate Identity** | Run `security find-identity -v -p codesigning`, e.g., `Developer ID Application: Your Name (ABC123XYZ)`. |
| `APPLE_DEVELOPER_CERTIFICATE_P12_BASE64` | **Base64 encoded .p12 certificate** | Export cert and run `base64 -i cert.p12 \| pbcopy`. |
| `APPLE_DEVELOPER_CERTIFICATE_PASSWORD` | **.p12 certificate password** | The password you set during export. |
| `AUTHOR_NAME` | **Author Name** | Used to replace AUTHOR_NAME in `wails.json`. |
| `AUTHOR_EMAIL` | **Author Email** | Used to replace AUTHOR_EMAIL in `wails.json`. |

#### macOS Signing & Notarization (Optional)

| Secret Name | Description | Default Value |
| :--- | :--- | :--- |
| `APPLE_BUNDLE_ID` | **Application Bundle ID** | `com.wails.BingPaperDesktop` |

#### Windows Signing (Optional)

| Secret Name | Description | How to Obtain |
| :--- | :--- | :--- |
| `WIN_SIGNING_CERT` | **Base64 encoded PFX certificate** | Run `certutil -encode .\cert.pfx cert.txt` and copy content. |
| `WIN_SIGNING_CERT_PASSWORD` | **Certificate password** | The password for the certificate. |

### 📄 License

This project is licensed under the [MIT License](./LICENSE).
