package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"BingPaperDesktop/internal/bing"
	"BingPaperDesktop/internal/overlay"
	"BingPaperDesktop/internal/scheduler"
	"BingPaperDesktop/internal/store"
	"BingPaperDesktop/internal/util"
	"BingPaperDesktop/internal/wallpaper"
)

type App struct {
	ctx       context.Context
	sched     *scheduler.Scheduler
	fetchMu   sync.Mutex
	lastFetch *CurrentResult
}

type CurrentResult struct {
	Item    store.HistoryItem `json:"item"`
	Success bool              `json:"success"`
	Error   string            `json:"error"`
}

func NewApp() *App {
	a := &App{}
	a.sched = scheduler.New(func() error {
		_, err := a.FetchToday(0, 0, 1.0)
		return err
	})
	return a
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	cfg, _ := store.LoadConfig()
	a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	a.sched.Start()

	slog.Info("App startup", "os", filepath.Base(os.Args[0]))
}

func (a *App) GetConfig() (store.Config, error) {
	return store.LoadConfig()
}

func (a *App) SaveConfig(cfg store.Config) error {
	err := store.SaveConfig(cfg)
	if err == nil {
		a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	}
	return err
}

func (a *App) FetchToday(screenW, screenH int, dpr float64) (CurrentResult, error) {
	a.fetchMu.Lock()
	defer a.fetchMu.Unlock()

	cfg, _ := store.LoadConfig()

	// Default fallback for resolution
	if screenW == 0 {
		screenW = 1920
		screenH = 1080
	}
	realW := int(float64(screenW) * dpr)
	realH := int(float64(screenH) * dpr)

	slog.Info("FetchToday started", "screen", fmt.Sprintf("%dx%d", realW, realH))

	meta, err := bing.FetchMeta(cfg.ApiMetaUrl)
	if err != nil {
		return CurrentResult{Error: err.Error()}, err
	}

	chosen := bing.SelectVariant(meta, realW, realH, cfg.ForceUHD, cfg.PreferAspectMatch)

	key := fmt.Sprintf("%s_%s", meta.Date, meta.Hsh)
	dayDir := filepath.Join("data", meta.Date)
	absDayDir := filepath.Join(store.GetBaseDir(), dayDir)
	os.MkdirAll(absDayDir, 0755)

	ext := ".jpg"
	if chosen.Format != "" {
		ext = "." + chosen.Format
	}
	relImagePath := filepath.Join(dayDir, "original"+ext)
	absImagePath := filepath.Join(store.GetBaseDir(), relImagePath)

	// Download if not exists
	if _, err := os.Stat(absImagePath); os.IsNotExist(err) {
		slog.Info("Downloading image", "url", chosen.URL)
		if err := bing.DownloadImage(chosen.URL, absImagePath); err != nil {
			return CurrentResult{Error: err.Error()}, err
		}
	}

	relWatermarkPath := ""
	if cfg.OverlayMetadata {
		relWatermarkPath = filepath.Join(dayDir, "watermarked"+ext)
		absWatermarkPath := filepath.Join(store.GetBaseDir(), relWatermarkPath)
		if _, err := os.Stat(absWatermarkPath); os.IsNotExist(err) {
			slog.Info("Generating watermark")
			if err := overlay.AddWatermark(absImagePath, absWatermarkPath, meta.Title, meta.Date, meta.Copyright); err != nil {
				slog.Error("Watermark failed", "error", err)
				relWatermarkPath = "" // Fallback to original
			}
		}
	}

	item := store.HistoryItem{
		Key:           key,
		Date:          meta.Date,
		Title:         meta.Title,
		Copyright:     meta.Copyright,
		ChosenVariant: chosen.Variant,
		ImagePath:     relImagePath,
		WatermarkPath: relWatermarkPath,
		CreatedAt:     time.Now(),
	}

	if err := store.AddToHistory(item); err != nil {
		slog.Error("Save history failed", "error", err)
	}

	res := CurrentResult{Item: item, Success: true}
	a.lastFetch = &res

	if cfg.AutoApply {
		slog.Info("Auto applying wallpaper")
		a.ApplyWallpaper(cfg.OverlayMetadata)
	}

	return res, nil
}

func (a *App) ApplyWallpaper(preferWatermarked bool) error {
	if a.lastFetch == nil || !a.lastFetch.Success {
		return fmt.Errorf("no current image to apply")
	}
	return a.ApplyHistory(a.lastFetch.Item.Key, preferWatermarked)
}

func (a *App) ListHistory() ([]store.HistoryItem, error) {
	idx, err := store.LoadIndex()
	if err != nil {
		return nil, err
	}
	return idx.Items, nil
}

func (a *App) ApplyHistory(key string, preferWatermarked bool) error {
	idx, err := store.LoadIndex()
	if err != nil {
		return err
	}

	var target *store.HistoryItem
	for _, item := range idx.Items {
		if item.Key == key {
			target = &item
			break
		}
	}

	if target == nil {
		return fmt.Errorf("history item not found")
	}

	path := target.ImagePath
	if preferWatermarked && target.WatermarkPath != "" {
		path = target.WatermarkPath
	}

	absPath := filepath.Join(store.GetBaseDir(), path)
	slog.Info("Applying wallpaper", "path", absPath)
	return wallpaper.Set(absPath)
}

func (a *App) DeleteHistory(key string) error {
	return store.DeleteFromHistory(key)
}

func (a *App) ClearHistory() error {
	return store.ClearHistory()
}

func (a *App) CleanupByRetainDays() (int, error) {
	cfg, _ := store.LoadConfig()
	return store.CleanupByRetainDays(cfg.RetainDays)
}

func (a *App) OpenDataDir() error {
	return util.OpenFolder(store.GetDataDir())
}

func (a *App) OpenLogsDir() error {
	return util.OpenFolder(store.GetLogsDir())
}

func (a *App) GetWallpaperSupport() (bool, string) {
	return wallpaper.Supported()
}

func (a *App) GetImageDataURL(relPath string) (string, error) {
	if relPath == "" {
		return "", nil
	}
	absPath := filepath.Join(store.GetBaseDir(), relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	mime := "image/jpeg"
	if filepath.Ext(absPath) == ".png" {
		mime = "image/png"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}
