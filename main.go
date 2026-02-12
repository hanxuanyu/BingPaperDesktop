package main

import (
	"embed"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"BingPaperDesktop/internal/app"
	"BingPaperDesktop/internal/store"

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
	// Use f first to ensure it's written even if stdout fails (common in GUI apps)
	mw := io.MultiWriter(f, &ignoreErrorWriter{os.Stdout})
	logger := slog.New(slog.NewTextHandler(mw, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
}

type ignoreErrorWriter struct {
	w io.Writer
}

func (iw *ignoreErrorWriter) Write(p []byte) (n int, err error) {
	n, _ = iw.w.Write(p)
	return n, nil
}

func main() {
	// Initialize store and portable paths
	if err := store.Init(); err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize logger after store is ready
	initLogger()

	slog.Info("Log system initialized", "baseDir", store.GetBaseDir(), "logPath", filepath.Join(store.GetLogsDir(), "app.log"))

	// Create an instance of the app structure
	appInstance := app.NewApp()

	// Create a system tray menu
	go func() {
		systray.Run(func() {
			if runtime.GOOS == "windows" {
				systray.SetIcon(appIconIco)
			} else {
				systray.SetIcon(appIcon)
			}
			systray.SetTooltip("BingPaperDesktop")

			mShow := systray.AddMenuItem("显示界面", "显示界面")
			mShow.Click(func() {
				ctx := appInstance.GetContext()
				if ctx != nil {
					wruntime.WindowShow(ctx)
				}
			})
			mFetch := systray.AddMenuItem("立即刷新壁纸", "立即获取并设置今日壁纸")
			mFetch.Click(func() {
				// Get screen dimensions from Wails if possible, otherwise use defaults
				appInstance.FetchToday(0, 0, 1.0)
			})
			systray.AddSeparator()
			mQuit := systray.AddMenuItem("退出程序", "彻底退出")
			mQuit.Click(func() {
				ctx := appInstance.GetContext()
				if ctx != nil {
					wruntime.Quit(ctx)
				} else {
					os.Exit(0)
				}
			})
		}, nil)
	}()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "BingPaperDesktop",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        appInstance.Startup,
		Bind: []interface{}{
			appInstance,
		},
		HideWindowOnClose: true,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
