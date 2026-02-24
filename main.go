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
	"gopkg.in/natefinch/lumberjack.v2"

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

var logWriter *lumberjack.Logger

func initLogger() {
	cfg, _ := store.LoadConfig()
	logPath := filepath.Join(store.GetLogsDir(), "app.log")

	logWriter = &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    cfg.LogMaxSize,    // megabytes
		MaxBackups: 3,                 // keep 3 old files
		MaxAge:     cfg.LogRetainDays, // days
		Compress:   false,             // disabled by default
	}

	// MultiWriter to both file and stdout
	mw := io.MultiWriter(logWriter, os.Stdout)
	logger := slog.New(slog.NewTextHandler(mw, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
}

func updateLogger(cfg store.Config) {
	if logWriter == nil {
		return
	}
	logWriter.MaxSize = cfg.LogMaxSize
	logWriter.MaxAge = cfg.LogRetainDays
	// Trigger rotation check if needed (lumberjack does this on write,
	// but we could call logWriter.Rotate() if we want immediate cleanup)
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

	// Register log cleanup function
	app.RegisterLogCleanup(func() error {
		if logWriter != nil {
			return logWriter.Rotate()
		}
		return nil
	})

	// Register log update function
	app.RegisterLogUpdate(updateLogger)

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
