package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

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
	wmChan    chan string
}

type WatermarkRequest struct {
	ImagePath string `json:"image_path"`
	Title     string `json:"title"`
	Date      string `json:"date"`
	Copyright string `json:"copyright"`
	Variant   string `json:"variant"`
}

type CurrentResult struct {
	Item    store.HistoryItem `json:"item"`
	Success bool              `json:"success"`
	Error   string            `json:"error"`
}

func NewApp() *App {
	a := &App{
		wmChan: make(chan string),
	}
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

func (a *App) GetContext() context.Context {
	return a.ctx
}

func (a *App) GetConfig() (store.Config, error) {
	return store.LoadConfig()
}

func (a *App) SaveConfig(cfg store.Config) error {
	if cfg.IntervalMinutes < 1 {
		cfg.IntervalMinutes = 1
	}
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

	meta, err := bing.FetchMeta(cfg.ApiType, cfg.ApiMetaUrl)
	if err != nil {
		return CurrentResult{Error: err.Error()}, err
	}

	chosen := bing.SelectVariant(meta, realW, realH, cfg.ForceUHD)

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

	// Save meta.json
	metaPath := filepath.Join(absDayDir, "meta.json")
	if metaData, err := json.MarshalIndent(meta, "", "  "); err == nil {
		os.WriteFile(metaPath, metaData, 0644)
	}

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
			slog.Info("Requesting frontend to render watermark")
			dataURL, err := a.GetImageDataURL(relImagePath)
			if err == nil {
				runtime.EventsEmit(a.ctx, "render-watermark", WatermarkRequest{
					ImagePath: dataURL,
					Title:     meta.Title,
					Date:      meta.Date,
					Copyright: meta.Copyright,
					Variant:   chosen.Variant,
				})

				// Wait for response with timeout
				select {
				case base64Data := <-a.wmChan:
					if err := overlay.SaveBase64Image(base64Data, absWatermarkPath); err != nil {
						slog.Error("Failed to save watermark", "error", err)
						relWatermarkPath = ""
					}
				case <-time.After(10 * time.Second):
					slog.Error("Watermark rendering timeout")
					relWatermarkPath = ""
				}
			} else {
				slog.Error("Failed to get image data url for watermark", "error", err)
				relWatermarkPath = ""
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
		if cfg.RandomHistory {
			a.ApplyRandomHistory(cfg.OverlayMetadata)
		} else {
			// ApplyWallpaper will call ApplyHistory, which handles the event emission
			a.ApplyWallpaper(cfg.OverlayMetadata)
		}
	} else {
		// Notify frontend about the new image
		runtime.EventsEmit(a.ctx, "current-image-changed", item)
	}

	return res, nil
}

func (a *App) ApplyWallpaper(preferWatermarked bool) error {
	if a.lastFetch == nil || !a.lastFetch.Success {
		return fmt.Errorf("no current image to apply")
	}
	return a.ApplyHistory(a.lastFetch.Item.Key, preferWatermarked)
}

func (a *App) ApplyRandomHistory(preferWatermarked bool) error {
	idx, err := store.LoadIndex()
	if err != nil {
		return err
	}
	if len(idx.Items) == 0 {
		return fmt.Errorf("no history items found")
	}

	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	target := idx.Items[r.Intn(len(idx.Items))]

	slog.Info("Randomly selected from history", "key", target.Key, "title", target.Title)
	return a.ApplyHistory(target.Key, preferWatermarked)
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
	if err := wallpaper.Set(absPath); err != nil {
		return err
	}

	// Update last fetch and notify frontend
	a.lastFetch = &CurrentResult{Item: *target, Success: true}
	runtime.EventsEmit(a.ctx, "current-image-changed", *target)

	return nil
}

func (a *App) DeleteHistory(key string) error {
	return store.DeleteFromHistory(key)
}

func (a *App) ClearHistory() error {
	return store.ClearHistory()
}

func (a *App) CleanupByRetainDays(days int) (int, error) {
	return store.CleanupByRetainDays(days)
}

func (a *App) OpenDataDir() error {
	return util.OpenFolder(store.GetDataDir())
}

func (a *App) OpenBaseDir() error {
	return util.OpenFolder(store.GetBaseDir())
}

func (a *App) OpenLogsDir() error {
	return util.OpenFolder(store.GetLogsDir())
}

func (a *App) GetWallpaperSupport() (bool, string) {
	return wallpaper.Supported()
}

func (a *App) ResetApplication() error {
	slog.Info("!!! AUTOMATIC RESET TRIGGERED !!!")
	a.sched.Stop()

	base := store.GetBaseDir()
	dataDir := filepath.Join(base, "data")
	configFile := filepath.Join(base, "config.json")

	slog.Info("Reset: cleaning up data and config", "dataDir", dataDir, "configFile", configFile)

	// 1. 物理删除数据目录（包含 index.json 和所有图片子目录）
	if err := os.RemoveAll(dataDir); err != nil {
		slog.Warn("Reset: failed to remove data directory", "error", err)
	}

	// 2. 删除配置文件
	if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
		slog.Warn("Reset: failed to remove config file", "error", err)
	}

	// 3. 重新初始化存储结构（重建 data 和 logs 目录）
	if err := store.Init(); err != nil {
		slog.Error("Reset: store.Init failed", "error", err)
		return fmt.Errorf("failed to re-initialize storage: %w", err)
	}

	// 4. 恢复并保存默认配置
	cfg := store.DefaultConfig()
	if err := store.SaveConfig(cfg); err != nil {
		slog.Error("Reset: saving default config failed", "error", err)
		return fmt.Errorf("failed to save default config: %w", err)
	}

	// 5. 同步调度器状态
	a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	a.sched.Start()

	// 6. 清理内存状态
	a.lastFetch = nil

	slog.Info("!!! AUTOMATIC RESET COMPLETED !!!")
	return nil
}

func (a *App) Quit() {
	runtime.Quit(a.ctx)
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

func (a *App) SubmitWatermark(base64Data string) {
	a.wmChan <- base64Data
}
