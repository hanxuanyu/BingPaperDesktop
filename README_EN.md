# BingPaperDesktop

[![Build Verification](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/verify.yml/badge.svg)](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/verify.yml)
[![Release](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/release.yml/badge.svg)](https://github.com/hanxuanyu/BingPaperDesktop/actions/workflows/release.yml)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/hanxuanyu/BingPaperDesktop)](https://github.com/hanxuanyu/BingPaperDesktop/releases)

English | [简体中文](./README.md)  [Developer Docs ](./DEV_EN.md)

> A cross-platform Bing daily wallpaper desktop client built with Wails v2 + Go + React. Automatically fetches the Bing picture of the day in high resolution and keeps your desktop fresh every day.

---

## Features

###  Dual API Modes

The application supports two image data sources, switchable at any time in Settings:

| Mode | Endpoint | Description |
|------|----------|-------------|
| **Bing Official** | `https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=1&uhd=1&mkt=zh-CN` | Fetches today's wallpaper metadata directly from Microsoft Bing's official API  stable and reliable |
| **BingPaper API** | `https://bing.coding.icu/api/v1/image/today/meta` (default) | Provided by the open-source project [BingPaper](https://github.com/hanxuanyu/BingPaper), which automatically captures the Bing daily image every day and exposes it via a standardized API with multi-resolution variant support |

> **About the BingPaper backend**: [hanxuanyu/BingPaper](https://github.com/hanxuanyu/BingPaper) is a standalone open-source server project focused on daily Bing image capturing and API serving. You can self-host it and configure this application to use your own API endpoint for a fully self-controlled image source.

---

###  Flexible Auto-Update Scheduling

Choose one of four update modes under **Settings  Schedule Mode**:

| Mode | Description |
|------|-------------|
| **Off** | No automatic updates  refresh manually only |
| **Daily** | Automatically fetch and apply at a specific time (e.g. `08:30`) |
| **Interval** | Refresh every N minutes |
| **On Wake** | Refresh automatically when the system wakes from sleep or hibernation |

---

###  Smart Resolution Selection

- Automatically detects screen resolution and DPR (device pixel ratio) to pick the best matching image variant
- **Force UHD mode** available to always prefer ultra-high-definition images (38402160)
- Stores both 16:9 and 4:3 aspect ratio versions for multi-monitor setups

---

###  Wallpaper History Management

- All downloaded wallpapers are automatically saved to a local history
- Browse thumbnail previews in the history drawer
- One-click to set any historical image as the current wallpaper
- **Random History Mode**: randomly picks a wallpaper from history on each auto-update cycle
- Automatic cleanup of old history by configurable retention days (set to `0` to keep forever)

---

###  Watermark & Calendar Overlay

- **Metadata watermark**: optionally overlay the image title, copyright, and date on the wallpaper
- **Calendar overlay**: optionally display the lunar calendar date and Chinese holidays (requires downloading holiday data)
- Overlay images are saved as separate PNG files  the original image is never modified

---

###  Flexible Data Storage

| Scenario | Storage Location |
|----------|------------------|
| **Portable mode** (default) | Same directory as the executable  perfect for USB drives |
| **Standard mode** | Automatically falls back to user config directory when the app directory is read-only (e.g. `C:\Program Files`) |
| **macOS** | Defaults to `~/Library/Application Support/BingPaperDesktop/` |
| **Custom directory** | Manually specify any directory via the Settings UI |

Data directory layout:

```
<data directory>/
 config.json           # Configuration file
 data/                 # Downloaded wallpapers and metadata (organized by date)
    YYYY-MM-DD/
        original.jpg
        watermark_16_9.png
        watermark_4_3.png
        meta.json
 thumbnails/           # History thumbnails
 logs/                 # Application logs
```

---

###  System Tray Integration

- Minimize to system tray  keeps your taskbar clean
- Tray menu supports: **Show Window** / **Refresh Wallpaper Now** / **Quit**
- macOS: option to hide the Dock icon for a tray-only experience

---

###  Additional Features

| Feature | Description |
|---------|-------------|
| **Auto-start** | Optionally launch at system startup |
| **Log management** | Rotating log files with configurable max size and retention days |
| **Holiday data** | Uses embedded annual holiday data by default, with optional custom update source for calendar overlays |
| **Quick actions** | One-click refresh and apply buttons in the main UI |
| **Wallpaper info panel** | Displays the current wallpaper's title and copyright in the main UI |

---

##  Download & Install

Visit the [Releases page](https://github.com/hanxuanyu/BingPaperDesktop/releases) to download for your platform:

| Platform | Format | Installation |
|----------|--------|--------------|
| **Windows 10/11** | `.exe` portable single file | Run directly  no installation needed |
| **macOS** | `.dmg` disk image | Mount and drag to the Applications folder |
| **Linux (GNOME)** | Binary executable | Make executable and run directly |

> **macOS first-run notice**: If macOS warns that the developer cannot be verified, go to **System Settings  Privacy & Security** and allow the app to run, or run `xattr -cr /Applications/BingPaperDesktop.app` in Terminal.

---

## Advanced Usage

### Command-line Arguments

```bash
# Specify a custom data directory
./BingPaperDesktop --data-path /path/to/data
```

### Environment Variables

```bash
# Specify data directory via environment variable
export BING_PAPER_DATA_PATH=/path/to/data
./BingPaperDesktop
```

---

## Self-Hosting the BingPaper API

If you want to use your own wallpaper API backend, you can deploy the open-source project [hanxuanyu/BingPaper](https://github.com/hanxuanyu/BingPaper).

After deployment, configure this application:

1. Switch **API Mode** to **BingPaper (Custom)**
2. Set the **Custom API URL** to your deployed service endpoint, e.g.:
   ```
   https://your-domain.com/api/v1/image/today/meta
   ```

---

## Contributing

If you are interested in the technical implementation or want to contribute, please refer to the [Developer Documentation DEV_EN.md](./DEV_EN.md), which covers:

- Project directory structure and module descriptions
- Frontend and backend build workflows
- CI/CD release configuration

---

##  Copyright Notice

- All wallpaper images displayed by this application are **copyright of Microsoft Bing**. Copyright information is included in the image metadata (`copyright` field).
- This application only retrieves image URLs through the public API for personal desktop use. **It does not redistribute or commercially use any images.**
- Copyright text shown in watermarks (e.g. ` Getty Images`) is sourced directly from Bing's official metadata and is presented as-is to respect original copyright.
- If you intend to use any wallpaper for commercial or other purposes, please review the image copyright and obtain appropriate authorization.

---

## License

This project is open-sourced under the [MIT License](./LICENSE).
