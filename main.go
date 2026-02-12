package main

import (
	"embed"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"BingPaperDesktop/internal/app"
	"BingPaperDesktop/internal/store"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func initLogger() {
	logPath := filepath.Join(store.GetLogsDir(), "app.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}

	// MultiWriter to both file and stdout
	mw := io.MultiWriter(os.Stdout, f)
	logger := slog.New(slog.NewTextHandler(mw, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
}

func main() {
	// Initialize store and portable paths
	if err := store.Init(); err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize logger after store is ready
	initLogger()

	// Create an instance of the app structure
	appInstance := app.NewApp()

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
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
