package main

import (
	"bytes"
	"context"
	"embed"
	"image"
	"image/png"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/image/draw"

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

func resizeIcon(data []byte, size int) []byte {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	newImg := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(newImg, newImg.Bounds(), img, img.Bounds(), draw.Over, nil)
	var buf bytes.Buffer
	if err := png.Encode(&buf, newImg); err != nil {
		return data
	}
	return buf.Bytes()
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

	// Define tray initialization
	onReady := func() {
		systray.CreateMenu()
		slog.Info("Systray onReady start")

		showWindow := func() {
			go func() {
				ctx := appInstance.GetContext()
				if ctx != nil {
					wruntime.WindowShow(ctx)
				}
			}()
		}

		// 显式设置点击回调以进行诊断
		systray.SetOnClick(func(menu systray.IMenu) {
			slog.Info("Tray: Left click triggered")
			if runtime.GOOS == "darwin" {
				menu.ShowMenu()
			}
		})
		systray.SetOnRClick(func(menu systray.IMenu) {
			slog.Info("Tray: Right click triggered")
			menu.ShowMenu()
		})
		systray.SetOnDClick(func(menu systray.IMenu) {
			slog.Info("Tray: Double click triggered")
			showWindow()
		})

		slog.Info("Adding tray menu items")
		mShow := systray.AddMenuItem("显示界面", "显示界面")
		mShow.Click(func() {
			slog.Info("Tray menu: Show clicked")
			showWindow()
		})
		mFetch := systray.AddMenuItem("立即刷新壁纸", "立即获取并设置今日壁纸")
		mFetch.Click(func() {
			go func() {
				slog.Info("Tray menu: Fetch clicked")
				appInstance.FetchToday(0, 0, 1.0)
			}()
		})
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出程序", "彻底退出")
		mQuit.Click(func() {
			go func() {
				slog.Info("Tray menu: Quit clicked")
				ctx := appInstance.GetContext()
				if ctx != nil {
					wruntime.Quit(ctx)
				} else {
					os.Exit(0)
				}
			}()
		})

		if runtime.GOOS == "windows" {
			systray.SetIcon(appIconIco)
		} else if runtime.GOOS == "darwin" {
			// macOS 菜单栏图标推荐尺寸为 22x22
			iconSmall := resizeIcon(appIcon, 22)
			slog.Info("Setting systray icon (macOS)", "original_len", len(appIcon), "new_len", len(iconSmall))
			systray.SetTemplateIcon(iconSmall, iconSmall)
		} else {
			slog.Info("Setting systray icon", "len", len(appIcon))
			systray.SetIcon(appIcon)
		}
		systray.SetTooltip("BingPaperDesktop")
		slog.Info("Systray onReady complete")
	}

	var trayStart, trayEnd func()
	if runtime.GOOS == "darwin" {
		trayStart, trayEnd = systray.RunWithExternalLoop(onReady, nil)
		slog.Info("Darwin: using RunWithExternalLoop")
		trayStart()
	} else {
		systray.Register(onReady, nil)
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "BingPaperDesktop",
		Width:  1024,
		Height: 768,
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
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
