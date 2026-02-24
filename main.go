package main

import (
	"context"
	"embed"
	"flag"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"BingPaperDesktop/internal/app"
	"BingPaperDesktop/internal/store"
	"BingPaperDesktop/internal/util"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed frontend/src/assets/images/appicon.png
var appIcon []byte

//go:embed build/windows/icon.ico
var appIconIco []byte

func initLogger() {
	logPath := filepath.Join(store.GetLogsDir(), "app.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	// MultiWriter to both file and stdout
	mw := io.MultiWriter(f, os.Stdout)
	logger := slog.New(slog.NewTextHandler(mw, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
}

func main() {
	// Parse command line flags
	dataPath := flag.String("data-path", "", "Custom path for configuration and data files")
	flag.Parse()

	if *dataPath != "" {
		store.SetBaseDir(*dataPath)
	}

	// Initialize store and portable paths
	if err := store.Init(); err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize logger after store is ready
	initLogger()

	slog.Info("Log system initialized", "baseDir", store.GetBaseDir(), "logPath", filepath.Join(store.GetLogsDir(), "app.log"))

	// Create an instance of the app structure
	appInstance := app.NewApp()

	// Setup Tray
	trayStart, trayEnd := appInstance.SetupTray(appIcon, appIconIco)
	if trayStart != nil {
		trayStart()
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:       "BingPaperDesktop",
		Width:       1024,
		Height:      768,
		StartHidden: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			appInstance.Startup(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			if trayEnd != nil {
				trayEnd()
			} else {
				systray.Quit()
			}
		},
		Bind: []interface{}{
			appInstance,
		},
		HideWindowOnClose: true,
		OnDomReady: func(ctx context.Context) {
			if runtime.GOOS == "darwin" {
				// 启动时窗口隐藏，因此也隐藏 Dock 图标
				util.HideDockIcon()
			}
		},
		OnBeforeClose: func(ctx context.Context) bool {
			wruntime.EventsEmit(ctx, "prepare-show-window") // 借用这个事件来清除 toast，虽然是关闭
			if runtime.GOOS == "darwin" {
				util.HideDockIcon()
			}
			return false // 返回 false 以允许隐藏窗口（配合 HideWindowOnClose: true）
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
