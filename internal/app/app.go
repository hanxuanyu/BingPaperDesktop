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
	"strings"
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

var (
	Version    = "dev"
	CommitHash = "none"
	BuildTime  = "unknown"
)

type VersionInfo struct {
	Version    string `json:"version"`
	CommitHash string `json:"commit_hash"`
	BuildTime  string `json:"build_time"`
}

func (a *App) GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:    Version,
		CommitHash: CommitHash,
		BuildTime:  BuildTime,
	}
}

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

	// 启动时同步开机启动设置
	if cfg.AutoStart {
		if err := util.SetAutoStart(true); err != nil {
			slog.Error("Failed to set auto start on startup", "error", err)
		}
	}
}

func (a *App) GetBaseDir() string {
	return store.GetBaseDir()
}

func (a *App) SelectDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Data Directory",
	})
}

func (a *App) SetBaseDir(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// 1. 设置新路径
	store.SetBaseDir(path)

	// 2. 重新初始化 store (创建目录等)
	if err := store.ReInit(); err != nil {
		return err
	}

	// 3. 重新加载配置
	cfg, err := store.LoadConfig()
	if err != nil {
		return err
	}

	// 4. 重启调度器 (因为数据保存路径变了，可能需要重新获取或清理)
	a.sched.Stop()
	a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	a.sched.Start()

	slog.Info("BaseDir changed", "newPath", path)

	return nil
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
	// 同步开机启动设置
	oldCfg, _ := store.LoadConfig()
	if oldCfg.AutoStart != cfg.AutoStart {
		if err := util.SetAutoStart(cfg.AutoStart); err != nil {
			slog.Error("Failed to set auto start", "enable", cfg.AutoStart, "error", err)
			// 继续保存配置，但可能在这里返回错误或者保持旧的 AutoStart 状态
		}
	}

	// 同步 macOS Dock 图标显示设置
	if oldCfg.HideDockIcon != cfg.HideDockIcon {
		if cfg.HideDockIcon {
			util.HideDockIcon()
		} else {
			util.ShowDockIcon()
		}
	}

	err := store.SaveConfig(cfg)
	if err == nil {
		a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	}
	return err
}

func (a *App) IsAutoStartEnabled() (bool, error) {
	return util.IsAutoStartEnabled()
}

// FetchToday 获取今日壁纸并根据配置决定是否应用。
// 它是程序的核心业务逻辑，由调度器或手动触发。
func (a *App) FetchToday(screenW, screenH int, dpr float64) (CurrentResult, error) {
	a.fetchMu.Lock()
	defer a.fetchMu.Unlock()

	cfg, _ := store.LoadConfig()

	// 1. 设置默认分辨率，计算实际像素尺寸
	if screenW == 0 {
		screenW = 1920
		screenH = 1080
	}
	realW := int(float64(screenW) * dpr)
	realH := int(float64(screenH) * dpr)

	slog.Info("FetchToday started", "screen", fmt.Sprintf("%dx%d", realW, realH), "api", cfg.ApiType)

	// 2. 获取元数据并选择合适的图片变体
	apiUrl := cfg.BingApiUrl
	if cfg.ApiType == "custom" {
		apiUrl = cfg.CustomApiUrl
	}
	meta, err := bing.FetchMeta(cfg.ApiType, apiUrl)
	if err != nil {
		slog.Error("Failed to fetch meta", "error", err)
		return CurrentResult{Error: err.Error()}, err
	}

	chosen := bing.SelectVariant(meta, realW, realH, cfg.ForceUHD)
	slog.Info("Selected variant", "variant", chosen.Variant, "url", chosen.URL)

	// 3. 准备存储路径
	key := fmt.Sprintf("%s_%s", meta.Date, meta.Hsh)
	dayDir := filepath.Join("data", meta.Date)
	absDayDir := filepath.Join(store.GetBaseDir(), dayDir)

	// 旧版本兼容性处理：如果存在旧的目录格式（无短横线），尝试迁移
	a.migrateOldDataDir(meta.Date, absDayDir)

	if err := os.MkdirAll(absDayDir, 0755); err != nil {
		slog.Error("Failed to create day directory", "dir", absDayDir, "error", err)
		return CurrentResult{Error: err.Error()}, err
	}

	// 4. 下载图片（如果本地不存在）
	ext := ".jpg"
	if chosen.Format != "" {
		ext = "." + chosen.Format
	}
	relImagePath := filepath.Join(dayDir, "original"+ext)
	absImagePath := filepath.Join(store.GetBaseDir(), relImagePath)

	// 保存元数据以便后续查看
	a.saveMetaJson(meta, absDayDir)

	if _, err := os.Stat(absImagePath); os.IsNotExist(err) {
		slog.Info("Downloading image", "url", chosen.URL, "dest", absImagePath)
		if err := bing.DownloadImage(chosen.URL, absImagePath); err != nil {
			slog.Error("Download failed", "error", err)
			return CurrentResult{Error: err.Error()}, err
		}
	}

	// 5. 处理水印（通过前端渲染）
	relWatermarkPath := ""
	if cfg.OverlayMetadata {
		relWatermarkPath = a.ensureWatermark(meta, chosen, dayDir, relImagePath, ext)
	}

	// 6. 保存到历史记录
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
		slog.Error("Failed to save history", "key", key, "error", err)
	}

	res := CurrentResult{Item: item, Success: true}
	a.lastFetch = &res

	// 7. 根据配置自动应用壁纸
	if cfg.AutoApply {
		if cfg.RandomHistory {
			slog.Info("Random history enabled, picking a random wallpaper from history")
			return *a.lastFetch, a.ApplyRandomHistory(cfg.OverlayMetadata)
		}

		slog.Info("Auto applying wallpaper")
		a.ApplyWallpaper(cfg.OverlayMetadata)
	} else {
		// 通知前端图片已更新，但未自动应用
		runtime.EventsEmit(a.ctx, "current-image-changed", item)
	}

	return *a.lastFetch, nil
}

// migrateOldDataDir 处理旧版本日期格式 (YYYYMMDD) 到新格式 (YYYY-MM-DD) 的迁移
func (a *App) migrateOldDataDir(newDate, newAbsDayDir string) {
	if newDate == util.NormalizeDate(newDate) && len(newDate) == 10 {
		oldDate := strings.ReplaceAll(newDate, "-", "")
		oldDayDir := filepath.Join("data", oldDate)
		oldAbsDayDir := filepath.Join(store.GetBaseDir(), oldDayDir)
		if _, err := os.Stat(oldAbsDayDir); err == nil {
			slog.Info("Migrating old date format directory", "from", oldDate, "to", newDate)
			os.Rename(oldAbsDayDir, newAbsDayDir)
		}
	}
}

// saveMetaJson 将元数据保存为 meta.json
func (a *App) saveMetaJson(meta *bing.Meta, absDayDir string) {
	metaPath := filepath.Join(absDayDir, "meta.json")
	if metaData, err := json.MarshalIndent(meta, "", "  "); err == nil {
		if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
			slog.Warn("Failed to save meta.json", "path", metaPath, "error", err)
		}
	}
}

// ensureWatermark 确保水印图片存在，如果不存在则通过前端渲染生成
func (a *App) ensureWatermark(meta *bing.Meta, chosen bing.Variant, dayDir, relImagePath, ext string) string {
	relWatermarkPath := filepath.Join(dayDir, "watermarked"+ext)
	absWatermarkPath := filepath.Join(store.GetBaseDir(), relWatermarkPath)

	if _, err := os.Stat(absWatermarkPath); err == nil {
		return relWatermarkPath // 已存在
	}

	slog.Info("Requesting frontend to render watermark", "image", relImagePath)
	dataURL, err := a.GetImageDataURL(relImagePath)
	if err != nil {
		slog.Error("Failed to get image data url for watermark", "error", err)
		return ""
	}

	runtime.EventsEmit(a.ctx, "render-watermark", WatermarkRequest{
		ImagePath: dataURL,
		Title:     meta.Title,
		Date:      meta.Date,
		Copyright: meta.Copyright,
		Variant:   chosen.Variant,
	})

	// 等待前端回传结果（带超时）
	select {
	case base64Data := <-a.wmChan:
		if err := overlay.SaveBase64Image(base64Data, absWatermarkPath); err != nil {
			slog.Error("Failed to save watermarked image", "path", absWatermarkPath, "error", err)
			return ""
		}
		slog.Info("Watermark saved successfully", "path", relWatermarkPath)
		return relWatermarkPath
	case <-time.After(10 * time.Second):
		slog.Error("Watermark rendering timeout")
		return ""
	}
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

	// Update last fetch and notify frontend
	a.lastFetch = &CurrentResult{Item: *target, Success: true}
	runtime.EventsEmit(a.ctx, "current-image-changed", *target)

	slog.Info("Applying wallpaper", "path", absPath)
	if err := wallpaper.Set(absPath); err != nil {
		return err
	}

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

func (a *App) BrowserOpenURL(url string) error {
	return util.OpenURL(url)
}

func (a *App) GetWallpaperSupport() (bool, string) {
	return wallpaper.Supported()
}

func (a *App) ResetSettings() error {
	slog.Info("Reset: only settings")
	a.sched.Stop()

	// 1. 恢复并保存默认配置
	cfg := store.DefaultConfig()
	if err := store.SaveConfig(cfg); err != nil {
		slog.Error("Reset: saving default config failed", "error", err)
		return fmt.Errorf("failed to save default config: %w", err)
	}

	// 2. 同步调度器状态
	a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	a.sched.Start()

	slog.Info("Reset settings completed")
	return nil
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
