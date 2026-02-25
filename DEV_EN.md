# BingPaperDesktop Developer Documentation

[简体中文](./DEV.md) | English  [Back to README](./README_EN.md)

This document is intended for developers and covers the project structure, module breakdown, frontend and backend build workflows, and CI/CD configuration to help you get started quickly.

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Desktop framework | [Wails v2](https://wails.io/) (Go + WebView) |
| Backend language | Go 1.23 |
| Frontend framework | React 18 + TypeScript |
| Frontend build tool | Vite |
| UI styling | Tailwind CSS + shadcn/ui |
| Logging | slog + lumberjack (log rotation) |
| System tray | energye/systray |

---

## Project Directory Structure

```
BingPaperDesktop/
 main.go                  # Entry point: initializes store, logger, Wails app, and tray
 go.mod / go.sum          # Go module dependencies
 wails.json               # Wails project config (app name, Bundle ID, etc.)

 internal/                # All backend business logic
    app/
       app.go           # Core app logic: methods exposed to the frontend (FetchToday, ApplyWallpaper, etc.)
       tray.go          # System tray menu initialization
    bing/
       client.go        # Bing API client: fetch metadata, select variant, download image
    overlay/
       watermark.go     # Pure-Go watermark overlay (fallback; primary overlay is rendered by frontend Canvas)
    scheduler/
       scheduler.go     # Scheduler supporting off/daily/interval/wakeup modes
    store/
       config.go        # Config read/write, defaults, path management (portable/standard/custom)
       holiday.go       # Holiday data download and caching
       index.go         # Wallpaper history index (CRUD)
    util/
       util.go          # Generic utilities (retry, date normalization, etc.)
       util_windows.go  # Windows-specific implementations (auto-start, Dock, etc.)
       util_darwin.go   # macOS-specific implementations
       util_others.go   # Other platform implementations
       image.go         # Image processing helpers (resize, etc.)
       wake_listener.go # System wake event listener
    wallpaper/
        wallpaper.go         # Wallpaper setter interface
        wallpaper_windows.go # Windows implementation (SystemParametersInfo)
        wallpaper_darwin.go  # macOS implementation (AppleScript)
        wallpaper_linux.go   # Linux/GNOME implementation (gsettings)

 frontend/                # Frontend source code (React + TypeScript)
    index.html
    vite.config.ts       # Vite build configuration
    tailwind.config.js   # Tailwind CSS configuration
    package.json
    src/
        main.tsx         # React app entry point
        App.tsx          # Root component: overall layout and global state management
        components/
           WallpaperInfo.tsx    # Wallpaper info display panel
           HistoryDrawer.tsx    # Wallpaper history drawer
           HistoryItemCard.tsx  # History item card component
           QuickToggles.tsx     # Quick action buttons
           SettingsDialog.tsx   # Settings dialog
           ResetDialog.tsx      # Reset confirmation dialog
           ui/                  # shadcn/ui base components (button, dialog, etc.)
        lib/
           utils.ts      # Frontend utilities (cn, etc.)
           watermark.ts  # Frontend Canvas watermark rendering logic
        types/
            lunar-javascript.d.ts  # Type declarations for the lunar calendar library

 wailsjs/                 # Auto-generated JS/TS bindings by Wails (do not edit manually)
    go/
       models.ts        # TypeScript types corresponding to Go structs
       app/
           App.d.ts     # Type declarations for Go methods callable from the frontend
           App.js       # IPC call implementations (via Wails runtime)
    runtime/             # Wails runtime JS library

 build/                   # Platform build resources
    bin/                 # Build output directory
    windows/             # Windows icon and manifest files
    darwin/              # macOS signing and notarization configuration

 script/                  # Quick build scripts
     build.sh             # Linux/macOS build script
     build.ps1            # Windows PowerShell build script
     publish_tag.sh       # Tag release script (Linux/macOS)
     publish_tag.ps1      # Tag release script (Windows)
```

---

## Module Details

### `internal/app/app.go`  Core Application Layer

This is the API layer exposed to the frontend. All methods listed in `wailsjs/go/app/App.d.ts` are implemented here. Key methods include:

- `FetchToday(screenW, screenH int, dpr float64)`  Full workflow: fetch metadata  select variant  download  generate watermark  save history  optionally apply
- `ApplyHistoryToMonitor(key string, monitorIdx, screenW, screenH int)`  Set a specific history image as the wallpaper
- `GetConfig() / SaveConfig()`  Read and write configuration
- `ListHistory() / DeleteHistory()`  Manage wallpaper history
- `GetVersionInfo()`  Retrieve version information

### `internal/bing/client.go`  API Client

Handles all HTTP request logic:

- Supports two API response formats: **Bing Official** JSON and **BingPaper** custom format
- `FetchMeta(apiType, url)`  Fetches metadata and normalizes it into the internal `Meta` struct
- `SelectVariant(meta, screenW, screenH, forceUHD)`  Intelligently selects the best image variant based on screen resolution and aspect ratio
- `DownloadImage(url, destPath)`  Image download with automatic retry

### `internal/scheduler/scheduler.go`  Scheduler

Implements four scheduling modes:

| Mode | Implementation |
|------|----------------|
| `off` | No goroutine started |
| `daily` | Calculates time until the target time, waits with `time.Sleep` |
| `interval` | Triggers every N minutes using `time.Ticker` |
| `wakeup` | Registers a system wake event callback via `wake_listener.go` |

### `internal/store/`  Data Persistence

- **`config.go`**: JSON config read/write, including auto-migration of legacy config fields; path strategy: portable  standard  custom
- **`index.go`**: Maintains `index.json` as the wallpaper history list  supports deduplication, date-based sorting, and cleanup by retention days
- **`holiday.go`**: Downloads and caches the current year's holiday JSON data from GitHub

### `internal/wallpaper/`  Wallpaper Setting

Cross-platform wallpaper setting, using Go build tags to select the correct platform implementation at compile time:

- **Windows**: Uses `win32.SystemParametersInfo(SPI_SETDESKWALLPAPER)`
- **macOS**: Invokes `System Events` via AppleScript
- **Linux**: Uses `gsettings set org.gnome.desktop.background picture-uri`

### `frontend/src/lib/watermark.ts`  Frontend Watermark Rendering

The primary watermark rendering logic runs in the frontend using HTML Canvas:

1. Loads the original image into a `<canvas>` element
2. Draws text (title, copyright, date) and optionally a lunar calendar with holiday information
3. Exports the canvas as a Base64-encoded PNG, which is sent to Go via Wails IPC to be saved as a file

---

## Setting Up the Development Environment

### Prerequisites

| Tool | Minimum Version | Purpose |
|------|-----------------|---------|
| [Go](https://go.dev/) | 1.23 | Backend language runtime |
| [Node.js](https://nodejs.org/) | 20 | Frontend build tooling |
| [Wails CLI](https://wails.io/docs/gettingstarted/installation) | v2 | Project build and dev server |

Install Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Clone and Run

```bash
git clone https://github.com/hanxuanyu/BingPaperDesktop.git
cd BingPaperDesktop

# Start development mode (hot-reload for frontend; backend changes require restart)
wails dev
```

In development mode, Wails automatically:
- Watches for Go file changes and recompiles the backend
- Starts the Vite dev server for frontend hot module replacement
- Displays the frontend inside a WebView window

---

## Production Build

### Using the Quick Scripts

```bash
# Linux / macOS
./script/build.sh

# Windows (PowerShell)
.\script\build.ps1
```

### Manual Build

```bash
# Basic build (no extra flags)
wails build

# Inject version information via ldflags
wails build -ldflags "-X BingPaperDesktop/internal/app.Version=1.2.3 \
  -X BingPaperDesktop/internal/app.CommitHash=$(git rev-parse --short HEAD) \
  -X BingPaperDesktop/internal/app.BuildTime=$(date -u +%FT%TZ)"
```

Build output is placed in the `build/bin/` directory.

### Frontend-Only Build

For debugging the frontend independently:

```bash
cd frontend
npm install
npm run build   # Production build
npm run dev     # Standalone Vite dev server (no Go backend)
```

---

## Wails FrontendBackend Communication

Wails uses code generation to implement IPC between Go and the frontend:

1. Define public methods (capitalized) in `internal/app/app.go`
2. When running `wails dev` or `wails generate module`, Wails auto-generates TypeScript declaration files and JS call wrappers in `wailsjs/go/app/`
3. **Do not manually edit** any files under the `wailsjs/` directory

For Go-to-frontend event pushing, use `runtime.EventsEmit(ctx, "event-name", payload)` on the Go side, and listen with `EventsOn("event-name", callback)` on the frontend.

---

## CI/CD Release Configuration

This project uses GitHub Actions for automated building, signing, and releasing. Workflow files are located in `.github/workflows/`.

### Trigger Conditions

- **verify.yml**: Triggered on every push and pull request. Performs a compilation check (no build artifacts produced).
- **release.yml**: Triggered when a tag matching `v*.*.*` is pushed. Builds and publishes platform-specific installers.

### Pre-release Setup (Secrets Configuration)

Configure the following secrets in **GitHub repository  Settings  Secrets and variables  Actions**:

#### Required  Author Information

| Secret | Description |
|--------|-------------|
| `AUTHOR_NAME` | Author name injected into `wails.json` |
| `AUTHOR_EMAIL` | Author email injected into `wails.json` |

#### Required  macOS Signing & Notarization

| Secret | Description |
|--------|-------------|
| `APPLE_ID_USER` | Apple Developer account email |
| `APPLE_PASSWORD` | App-specific password (generated at appleid.apple.com) |
| `APPLE_PROVIDER` | Team ID (found in Apple Developer Portal) |
| `APPLE_IDENTITY` | Certificate identifier, e.g. `Developer ID Application: Your Name (TEAMID)` |
| `APPLE_DEVELOPER_CERTIFICATE_P12_BASE64` | Base64-encoded `.p12` certificate |
| `APPLE_DEVELOPER_CERTIFICATE_PASSWORD` | Password for the `.p12` certificate |

#### Optional  macOS Custom Bundle ID

| Secret | Default |
|--------|---------|
| `APPLE_BUNDLE_ID` | `com.wails.BingPaperDesktop` |

#### Optional  Windows Code Signing

| Secret | Description |
|--------|-------------|
| `WIN_SIGNING_CERT` | Base64-encoded PFX certificate |
| `WIN_SIGNING_CERT_PASSWORD` | PFX certificate password |

### Releasing a New Version

```bash
# Push a tag to trigger the release workflow
git tag v1.2.3
git push origin v1.2.3

# Or use the project scripts (automatically updates the version in wails.json)
.\script\publish_tag.ps1   # Windows
./script/publish_tag.sh    # Linux/macOS
```

---

## Contribution Guide

1. Fork this repository and develop on your own branch
2. Follow the existing code style (standard `gofmt` for Go; Prettier for frontend)
3. For new features, include a brief description and explain the changes in the Pull Request
4. If you find a bug, please open an [Issue](https://github.com/hanxuanyu/BingPaperDesktop/issues) with reproduction steps and relevant logs (logs are located at `logs/app.log` inside the data directory)

---

## License

This project is open-sourced under the [MIT License](./LICENSE).
