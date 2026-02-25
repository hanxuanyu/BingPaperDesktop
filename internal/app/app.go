package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"math/rand"
	"net/http"
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
	mu        sync.RWMutex // 保护 lastFetch 和其他共享状态
	lastFetch *CurrentResult
	wmChan    chan string
	wmMu      sync.Mutex
}

type OverlayRequest struct {
	ImagePath       string             `json:"image_path"`
	Title           string             `json:"title"`
	Date            string             `json:"date"`
	Copyright       string             `json:"copyright"`
	Variant         string             `json:"variant"`
	EnableWatermark bool               `json:"enable_watermark"`
	EnableCalendar  bool               `json:"enable_calendar"`
	HolidayData     []store.HolidayDay `json:"holiday_data"`
	OnlyOverlay     bool               `json:"only_overlay"`
	Width           int                `json:"width"`
	Height          int                `json:"height"`
	TargetRatio     float64            `json:"target_ratio"` // 目标屏幕比例 (如 1.777, 1.333)
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
		// 自动获取并应用始终针对所有显示器
		return a.ApplyHistoryToMonitor("", -1, 1920, 1080)
	})
	return a
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	cfg, _ := store.LoadConfig()
	a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	a.sched.Start()

	slog.Info("App startup", "os", filepath.Base(os.Args[0]))

	// 检查节假日数据
	if cfg.EnableHoliday {
		go func() {
			year := time.Now().Year()
			if err := store.CheckAndDownloadHoliday(year, false); err != nil {
				slog.Error("Failed to check/download holiday data", "year", year, "error", err)
			}
		}()
	}

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
		// 检查节假日数据
		if cfg.EnableHoliday {
			go func() {
				year := time.Now().Year()
				// 只有当开关从关闭变为开启，或 API URL 发生变化时，强制触发下载
				force := (!oldCfg.EnableHoliday && cfg.EnableHoliday) || (oldCfg.HolidayApiUrl != cfg.HolidayApiUrl)
				if err := store.CheckAndDownloadHoliday(year, force); err != nil {
					slog.Error("Failed to check/download holiday data", "year", year, "error", err, "force", force)
				}
			}()
		}

		a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
		// 通知 main.go 更新日志配置
		if logUpdateFunc != nil {
			logUpdateFunc(cfg)
		}
	}
	return err
}

var logUpdateFunc func(store.Config)

func RegisterLogUpdate(fn func(store.Config)) {
	logUpdateFunc = fn
}

func (a *App) IsAutoStartEnabled() (bool, error) {
	return util.IsAutoStartEnabled()
}

// GetCurrentItem 获取当前应用的壁纸信息，通常由前端主动拉取以进行同步。
func (a *App) GetCurrentItem() (CurrentResult, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.lastFetch == nil {
		return CurrentResult{Success: false, Error: "no item fetched yet"}, nil
	}
	return *a.lastFetch, nil
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

	// 5. 准备叠加层
	if cfg.OverlayMetadata {
		// 同时生成 16:9 和 4:3 两个版本
		a.ensureWatermarkOverlay(meta, chosen, dayDir, relImagePath, cfg, 16.0/9.0)
		a.ensureWatermarkOverlay(meta, chosen, dayDir, relImagePath, cfg, 4.0/3.0)
	}

	// 6. 准备当前项
	item := store.HistoryItem{
		Key:           key,
		Date:          meta.Date,
		Title:         meta.Title,
		Copyright:     meta.Copyright,
		ChosenVariant: chosen.Variant,
		ImagePath:     relImagePath,
		CreatedAt:     time.Now(),
	}

	if err := store.AddToHistory(item); err != nil {
		slog.Error("Failed to save history", "key", key, "error", err)
	}

	// 7. 根据配置自动应用壁纸
	if cfg.AutoApply {
		if cfg.RandomHistory {
			slog.Info("Random history enabled, picking a random wallpaper from history")
			err := a.ApplyRandomHistory(realW, realH)
			if err == nil {
				// 成功应用了随机，a.lastFetch 已经在 ApplyHistoryToMonitor 中被更新为随机项
				a.mu.RLock()
				defer a.mu.RUnlock()
				return *a.lastFetch, nil
			}
			slog.Error("Apply random history failed, fallback to today", "error", err)
		}

		slog.Info("Auto applying wallpaper")
		_ = a.ApplyHistoryToMonitor(item.Key, -1, realW, realH)
	} else {
		// 未自动应用，更新 lastFetch 并通知前端图片已更新
		res := CurrentResult{Item: item, Success: true}
		a.mu.Lock()
		a.lastFetch = &res
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "current-image-changed", item)
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
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

func (a *App) ApplyWallpaper(screenW, screenH int) error {
	return a.ApplyHistoryToMonitor("", -1, screenW, screenH)
}

func (a *App) ApplyRandomHistory(screenW, screenH int) error {
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
	return a.ApplyHistoryToMonitor(target.Key, -1, screenW, screenH)
}

func (a *App) ListHistory() ([]store.HistoryItem, error) {
	idx, err := store.LoadIndex()
	if err != nil {
		return nil, err
	}
	return idx.Items, nil
}

// ensureWatermarkOverlay 确保特定比例的水印叠加层存在 (PNG 格式)
func (a *App) ensureWatermarkOverlay(meta *bing.Meta, chosen bing.Variant, dayDir, relImagePath string, cfg store.Config, targetRatio float64) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

	// 归一化比例为 16:9 或 4:3
	normRatio := 1.777777
	ratioSuffix := "_16_9"
	if targetRatio < 1.5 {
		normRatio = 1.333333
		ratioSuffix = "_4_3"
	}

	relPath := filepath.Join(dayDir, "watermark"+ratioSuffix+".png")
	absPath := filepath.Join(store.GetBaseDir(), relPath)

	if _, err := os.Stat(absPath); err == nil {
		return relPath
	}

	slog.Info("Requesting frontend to render watermark overlay", "image", relImagePath, "ratio", normRatio)
	url, err := a.GetImageURL(relImagePath)
	if err != nil {
		slog.Error("Failed to get image url", "error", err)
		return ""
	}

	runtime.EventsEmit(a.ctx, "render-watermark", OverlayRequest{
		ImagePath:       url,
		Title:           meta.Title,
		Date:            meta.Date,
		Copyright:       meta.Copyright,
		Variant:         chosen.Variant,
		EnableWatermark: true,
		EnableCalendar:  false,
		OnlyOverlay:     true,
		TargetRatio:     normRatio,
	})

	select {
	case base64Data := <-a.wmChan:
		if base64Data == "" {
			return ""
		}
		if err := overlay.SaveBase64Image(base64Data, absPath); err != nil {
			slog.Error("Failed to save watermark overlay", "path", absPath, "error", err)
			return ""
		}
		return relPath
	case <-time.After(10 * time.Second):
		slog.Error("Watermark processing timeout")
		return ""
	}
}

// getCalendarOverlay 获取当日的特定比例日历叠加层，按日期和分辨率缓存
func (a *App) getCalendarOverlay(width, height int, cfg store.Config, targetRatio float64) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

	today := time.Now().Format("2006-01-02")
	dayDir := filepath.Join("data", today)
	absDayDir := filepath.Join(store.GetBaseDir(), dayDir)
	_ = os.MkdirAll(absDayDir, 0755)

	suffix := ""
	if cfg.EnableHoliday {
		suffix = "h"
	}

	// 归一化比例为 16:9 或 4:3
	normRatio := 1.777777
	ratioSuffix := "_16_9"
	if targetRatio < 1.5 {
		normRatio = 1.333333
		ratioSuffix = "_4_3"
	}

	relPath := filepath.Join(dayDir, "calendar_cache_"+fmt.Sprintf("%dx%d", width, height)+suffix+ratioSuffix+".png")
	absPath := filepath.Join(store.GetBaseDir(), relPath)

	if _, err := os.Stat(absPath); err == nil {
		return absPath
	}

	slog.Info("Requesting frontend to render calendar overlay", "date", today, "size", fmt.Sprintf("%dx%d", width, height), "ratio", normRatio)

	var holidayData []store.HolidayDay
	if cfg.EnableHoliday {
		hData, err := store.LoadHoliday(time.Now().Year())
		if err == nil {
			holidayData = hData.Days
		}
	}

	runtime.EventsEmit(a.ctx, "render-watermark", OverlayRequest{
		Date:            today,
		EnableWatermark: false,
		EnableCalendar:  true,
		HolidayData:     holidayData,
		OnlyOverlay:     true,
		Width:           width,
		Height:          height,
		TargetRatio:     normRatio,
	})

	select {
	case base64Data := <-a.wmChan:
		if base64Data == "" {
			return ""
		}
		if err := overlay.SaveBase64Image(base64Data, absPath); err != nil {
			slog.Error("Failed to save calendar overlay", "path", absPath, "error", err)
			return ""
		}
		return absPath
	case <-time.After(10 * time.Second):
		slog.Error("Calendar processing timeout")
		return ""
	}
}

func (a *App) GetMonitors() ([]wallpaper.Monitor, error) {
	return wallpaper.GetMonitors()
}

func (a *App) ApplyHistory(key string, screenW, screenH int) error {
	return a.ApplyHistoryToMonitor(key, -1, screenW, screenH)
}

func (a *App) ApplyHistoryToMonitor(key string, monitorID int, screenW, screenH int) error {
	if key == "" {
		// 如果 key 为空，表示获取最新的
		res, err := a.FetchToday(screenW, screenH, 1.0)
		if err != nil {
			return err
		}
		key = res.Item.Key
	}

	idx, err := store.LoadIndex()
	if err != nil {
		return err
	}

	var target *store.HistoryItem
	for i := range idx.Items {
		if idx.Items[i].Key == key {
			target = &idx.Items[i]
			break
		}
	}

	if target == nil {
		return fmt.Errorf("history item not found")
	}

	cfg, _ := store.LoadConfig()

	// 获取所有显示器
	monitors, err := wallpaper.GetMonitors()
	if err != nil || len(monitors) == 0 {
		slog.Warn("Failed to get monitors, falling back to single monitor", "error", err)
		monitors = []wallpaper.Monitor{{ID: 0, Width: screenW, Height: screenH}}
	}

	// 过滤显示器
	var targets []wallpaper.Monitor
	if monitorID >= 0 {
		for _, m := range monitors {
			if m.ID == monitorID {
				targets = append(targets, m)
				break
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("monitor with ID %d not found", monitorID)
		}
	} else {
		targets = monitors
	}

	// 为每个目标显示器独立合成并应用壁纸
	for _, m := range targets {
		applyPath, err := a.prepareWallpaperForMonitor(target, m, cfg)
		if err != nil {
			slog.Error("Failed to prepare wallpaper for monitor", "id", m.ID, "error", err)
			continue
		}

		slog.Info("Applying wallpaper to monitor", "id", m.ID, "name", m.Name, "path", applyPath)
		if err := wallpaper.SetOnMonitor(m.ID, applyPath); err != nil {
			if strings.Contains(err.Error(), "IDesktopWallpaper not supported") {
				slog.Warn("Multi-monitor wallpaper not supported by system, falling back to global setting")
				_ = wallpaper.Set(applyPath)
				// 更新状态并跳出显示器循环 (因为它是一个全局设置)
				break
			}
			slog.Error("Failed to set wallpaper on monitor", "id", m.ID, "error", err)
			// 如果设置特定显示器失败，尝试全局设置作为回退
			_ = wallpaper.Set(applyPath)
		}
	}

	// Update last fetch and notify frontend
	a.mu.Lock()
	a.lastFetch = &CurrentResult{Item: *target, Success: true}
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "current-image-changed", *target)

	return nil
}

func (a *App) prepareWallpaperForMonitor(target *store.HistoryItem, m wallpaper.Monitor, cfg store.Config) (string, error) {
	absOriginalPath := filepath.Join(store.GetBaseDir(), target.ImagePath)

	// 计算当前屏幕比例
	targetRatio := 1.777777 // 默认 16:9
	if m.Width > 0 && m.Height > 0 {
		targetRatio = float64(m.Width) / float64(m.Height)
	}

	// 根据当前配置决定是否显示叠加层
	showOverlay := cfg.OverlayMetadata || cfg.EnableCalendar
	if !showOverlay {
		return absOriginalPath, nil
	}

	var overlays []string

	// 1. 水印 (针对图片固定)
	if cfg.OverlayMetadata {
		dayDir := filepath.Dir(target.ImagePath)
		tempMeta := &bing.Meta{
			Date:      target.Date,
			Title:     target.Title,
			Copyright: target.Copyright,
		}
		tempChosen := bing.Variant{
			Variant: target.ChosenVariant,
		}
		wmPath := a.ensureWatermarkOverlay(tempMeta, tempChosen, dayDir, target.ImagePath, cfg, targetRatio)
		if wmPath != "" {
			overlays = append(overlays, filepath.Join(store.GetBaseDir(), wmPath))
		}
	}

	// 2. 日历 (针对当前日期)
	if cfg.EnableCalendar {
		// 获取原图尺寸
		file, err := os.Open(absOriginalPath)
		if err == nil {
			imgCfg, _, err := image.DecodeConfig(file)
			file.Close()
			if err == nil {
				calPath := a.getCalendarOverlay(imgCfg.Width, imgCfg.Height, cfg, targetRatio)
				if calPath != "" {
					overlays = append(overlays, calPath)
				}
			}
		}
	}

	if len(overlays) > 0 {
		// 合成针对该显示器的最终图片
		// 在 macOS 上，如果文件路径完全相同，系统可能不会触发壁纸更新。
		// 因此我们在文件名中加入时间戳（分钟级别即可，或者秒，为了确保每次应用都生效，秒更好）
		suffix := time.Now().Unix()
		tempWallpaperPath := filepath.Join(store.GetBaseDir(), fmt.Sprintf("current_wallpaper_%d_%d.jpg", m.ID, suffix))

		// 清理该显示器之前的旧临时壁纸文件
		if matches, err := filepath.Glob(filepath.Join(store.GetBaseDir(), fmt.Sprintf("current_wallpaper_%d_*.jpg", m.ID))); err == nil {
			for _, oldPath := range matches {
				_ = os.Remove(oldPath)
			}
		}

		if err := overlay.Composite(absOriginalPath, overlays, tempWallpaperPath); err == nil {
			return tempWallpaperPath, nil
		} else {
			return "", fmt.Errorf("failed to composite image: %w", err)
		}
	}

	return absOriginalPath, nil
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

func (a *App) CleanupLogs() error {
	slog.Info("Manually triggering log cleanup")
	// Since main.go holds the logWriter, we might need a way to access it.
	// However, lumberjack cleans up based on MaxBackups and MaxAge.
	// To force a cleanup now, we can trigger a rotation.
	// But wait, App doesn't have access to logWriter directly if it's in main.
	// Let's define a callback or a package level variable in util or somewhere.
	// Or, more simply, we can just call Rotate on the lumberjack instance.
	// I'll add a way to register the logger or a cleanup function.
	if logCleanupFunc != nil {
		return logCleanupFunc()
	}
	return nil
}

var logCleanupFunc func() error

func RegisterLogCleanup(fn func() error) {
	logCleanupFunc = fn
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
	a.mu.Lock()
	a.lastFetch = nil
	a.mu.Unlock()

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

func (a *App) GetThumbnailURL(relPath string) (string, error) {
	if relPath == "" {
		return "", nil
	}

	// For thumbnails, we store them in a separate directory structure mirroring the data directory
	thumbRelPath := filepath.Join("thumbnails", relPath)
	thumbAbsPath := filepath.Join(store.GetBaseDir(), thumbRelPath)

	if _, err := os.Stat(thumbAbsPath); os.IsNotExist(err) {
		srcAbsPath := filepath.Join(store.GetBaseDir(), relPath)
		if _, err := os.Stat(srcAbsPath); err != nil {
			return "", err
		}

		if err := os.MkdirAll(filepath.Dir(thumbAbsPath), 0755); err != nil {
			return "", err
		}

		slog.Info("Generating thumbnail", "src", relPath)
		if err := util.GenerateThumbnail(srcAbsPath, thumbAbsPath, 400); err != nil {
			slog.Error("Failed to generate thumbnail", "error", err)
			return "/images/" + relPath, nil // Fallback to full image
		}
	}

	// Replace backslashes with forward slashes for URL consistency
	urlPath := filepath.ToSlash(thumbRelPath)
	return "/images/" + urlPath, nil
}

func (a *App) GetImageURL(relPath string) (string, error) {
	if relPath == "" {
		return "", nil
	}
	urlPath := filepath.ToSlash(relPath)
	return "/images/" + urlPath, nil
}

func (a *App) AssetsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/images/") {
			relPath := strings.TrimPrefix(path, "/images/")
			// Convert URL slashes back to system separators
			relPath = filepath.FromSlash(relPath)
			absPath := filepath.Join(store.GetBaseDir(), relPath)

			// Basic security check: ensure the path is within baseDir
			if !strings.HasPrefix(absPath, store.GetBaseDir()) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}

			// Add caching headers for performance
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.ServeFile(w, r, absPath)
			return
		}
		http.NotFound(w, r)
	})
}

func (a *App) SubmitWatermark(base64Data string) {
	// 避免在没有接收者时阻塞
	select {
	case a.wmChan <- base64Data:
	default:
		slog.Warn("SubmitWatermark: no receiver for channel")
	}
}
