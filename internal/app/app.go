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
	ctx               context.Context
	sched             *scheduler.Scheduler
	fetchMu           sync.Mutex
	mu                sync.RWMutex // 保护 lastFetch 和其他共享状态
	lastFetch         *CurrentResult
	wmChan            chan string
	wmMu              sync.Mutex
	monitorWallpapers map[int]string
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
		// wmChan 容量为 1：避免前端 JS 回调在 Go select 就绪之前发送数据时被丢弃。
		// 场景：Go 通过 EventsEmit 触发前端渲染，前端完成后调用 SubmitWatermark。
		// 若 JS 微任务先于 Go goroutine 的 select 就绪，无缓冲 channel 会使数据无声丢失。
		wmChan:            make(chan string, 1),
		monitorWallpapers: make(map[int]string),
	}
	a.sched = scheduler.New(func() error {
		// 调度器触发时不依赖前端传入的分辨率，直接使用 0 触发 FetchToday 默认分辨率逻辑。
		// FetchToday(0,0,1) 内部会回退到 1920×1080，实际壁纸应用时由 GetMonitors() 读取真实显示器信息。
		return a.ApplyHistoryToMonitor("", -1, 0, 0)
	})
	return a
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	cfg, _ := store.LoadConfig()

	// 按当前配置初始化调度器
	a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	a.sched.Start()

	slog.Info("App startup", "executable", filepath.Base(os.Args[0]))

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

	slog.Info("FetchToday started",
		"logicalScreen", fmt.Sprintf("%dx%d", screenW, screenH),
		"physicalScreen", fmt.Sprintf("%dx%d", realW, realH),
		"dpr", dpr,
		"api", cfg.ApiType,
		"forceUHD", cfg.ForceUHD,
	)

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

// normRatioAndSuffix 将目标比例归一化为 16:9 或 4:3，返回用于文件名的后缀（_16_9 或 _4_3）。
func normRatioAndSuffix(targetRatio float64) (normRatio float64, ratioSuffix string) {
	if targetRatio < 1.5 {
		return 1.333333, "_4_3"
	}
	return 1.777777, "_16_9"
}

// ensureWatermarkOverlay 确保特定比例（16:9 或 4:3）的元数据水印叠加图（PNG）已生成并保存。
// 若文件已存在则直接返回路径（缓存命中），否则通过 Wails 事件触发前端 Canvas 渲染后等待结果。
// 使用 wmMu 互斥锁保证同一时刻只有一个叠加渲染请求在进行，避免 wmChan 竞争。
func (a *App) ensureWatermarkOverlay(meta *bing.Meta, chosen bing.Variant, dayDir, relImagePath string, cfg store.Config, targetRatio float64) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

	normRatio, ratioSuffix := normRatioAndSuffix(targetRatio)
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
			slog.Warn("ensureWatermarkOverlay: received empty data from frontend")
			return ""
		}
		if err := overlay.SaveBase64Image(base64Data, absPath); err != nil {
			slog.Error("Failed to save watermark overlay", "path", absPath, "error", err)
			return ""
		}
		slog.Info("Watermark overlay saved", "path", relPath)
		return relPath
	case <-time.After(10 * time.Second):
		slog.Error("Watermark overlay processing timeout — frontend did not respond in time",
			"image", relImagePath, "ratio", targetRatio)
		return ""
	}
}

// getCalendarOverlay 获取当日的特定比例日历叠加层，按日期和分辨率缓存。
// 日历叠加层以日期+分辨率+是否含节假日为 key 缓存，每日首次申请时触发前端渲染。
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

	normRatio, ratioSuffix := normRatioAndSuffix(targetRatio)

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
			slog.Warn("getCalendarOverlay: received empty data from frontend")
			return ""
		}
		if err := overlay.SaveBase64Image(base64Data, absPath); err != nil {
			slog.Error("Failed to save calendar overlay", "path", absPath, "error", err)
			return ""
		}
		slog.Info("Calendar overlay saved", "path", relPath)
		return absPath
	case <-time.After(10 * time.Second):
		slog.Error("Calendar overlay processing timeout — frontend did not respond in time",
			"date", today, "size", fmt.Sprintf("%dx%d", width, height))
		return ""
	}
}

func (a *App) GetMonitors() ([]wallpaper.Monitor, error) {
	return wallpaper.GetMonitors()
}

type MonitorWallpaperInfo struct {
	MonitorID    int               `json:"monitor_id"`
	MonitorName  string            `json:"monitor_name"`
	HistoryItem  store.HistoryItem `json:"history_item"`
	ThumbnailURL string            `json:"thumbnail_url"`
}

func (a *App) GetMonitorWallpapers() ([]MonitorWallpaperInfo, error) {
	monitors, err := wallpaper.GetMonitors()
	if err != nil {
		return nil, err
	}

	idx, _ := store.LoadIndex()
	historyMap := make(map[string]store.HistoryItem)
	for _, item := range idx.Items {
		historyMap[item.Key] = item
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []MonitorWallpaperInfo
	for _, m := range monitors {
		info := MonitorWallpaperInfo{
			MonitorID:   m.ID,
			MonitorName: m.Name,
		}
		if key, ok := a.monitorWallpapers[m.ID]; ok {
			if item, exists := historyMap[key]; exists {
				info.HistoryItem = item
				thumb, _ := a.GetThumbnailURL(item.ImagePath)
				info.ThumbnailURL = thumb
			}
		}

		// 如果没有记录，或者记录的壁纸找不到了，尝试使用最后一次全局设置的壁纸
		if info.HistoryItem.Key == "" && a.lastFetch != nil {
			info.HistoryItem = a.lastFetch.Item
			thumb, _ := a.GetThumbnailURL(info.HistoryItem.ImagePath)
			info.ThumbnailURL = thumb
		}

		result = append(result, info)
	}

	return result, nil
}

func (a *App) ApplyHistory(key string, screenW, screenH int) error {
	return a.ApplyHistoryToMonitor(key, -1, screenW, screenH)
}

func (a *App) ApplyHistoryToMonitor(key string, monitorID int, screenW, screenH int) error {
	if key == "" {
		// key 为空时先获取今日壁纸（由调度器或托盘的「立即刷新」触发）。
		// 此时 screenW/H 为 0 代表使用默认，FetchToday 内部会回退到 1920×1080 并由 GetMonitors() 覆盖。
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
	a.mu.Lock()
	for _, m := range targets {
		m := m // shadow
		// 第一阶段：立即设置原图作为壁纸，减少视觉等待感
		absOriginalPath := filepath.Join(store.GetBaseDir(), target.ImagePath)
		slog.Debug("Phase 1: Setting original image as wallpaper", "id", m.ID, "path", absOriginalPath)
		_ = wallpaper.SetOnMonitor(m.ID, absOriginalPath)

		// 更新状态，以便前端立即显示正确的信息
		a.monitorWallpapers[m.ID] = target.Key

		// 检查是否需要叠加层
		if !cfg.OverlayMetadata && !cfg.EnableCalendar {
			continue
		}

		// 第二阶段：异步合成叠加层并更新
		go func(m wallpaper.Monitor) {
			applyPath, err := a.prepareWallpaperForMonitor(target, m, cfg)
			if err != nil {
				slog.Error("Failed to prepare wallpaper for monitor", "id", m.ID, "error", err)
				return
			}

			if applyPath == absOriginalPath {
				return
			}

			slog.Info("Phase 2: Applying composited wallpaper to monitor", "id", m.ID, "path", applyPath)
			if err := wallpaper.SetOnMonitor(m.ID, applyPath); err != nil {
				if strings.Contains(err.Error(), "IDesktopWallpaper not supported") {
					_ = wallpaper.Set(applyPath)
				} else {
					slog.Error("Failed to set composited wallpaper on monitor", "id", m.ID, "error", err)
					_ = wallpaper.Set(applyPath)
				}
			}

			// 合成完成后，虽然 Key 没变，但通知前端可能有助于同步状态（可选）
			runtime.EventsEmit(a.ctx, "monitor-wallpapers-changed", a.monitorWallpapers)
		}(m)
	}

	// Update last fetch and notify frontend immediately after Phase 1
	a.lastFetch = &CurrentResult{Item: *target, Success: true}
	a.mu.Unlock()

	runtime.EventsEmit(a.ctx, "current-image-changed", *target)
	runtime.EventsEmit(a.ctx, "monitor-wallpapers-changed", a.monitorWallpapers)

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
			relPath = filepath.FromSlash(relPath)
			// 防止路径穿越：清理后禁止包含 ".."
			relPath = filepath.Clean(relPath)
			if strings.Contains(relPath, "..") || filepath.IsAbs(relPath) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			absPath := filepath.Join(store.GetBaseDir(), relPath)
			// 再次确保最终路径在 baseDir 内（防止 Clean 后与 baseDir 组合逃逸）
			baseDir := store.GetBaseDir()
			if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(baseDir)) {
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

// SubmitWatermark 接收前端 Canvas 渲染完成的 Base64 图片数据，并将其投递到水印处理 channel。
// wmChan 容量为 1，此处使用非阻塞发送：若 channel 已满说明上一次渲染结果尚未被消费，直接丢弃并警告。
// 正常情况下 ensureWatermarkOverlay/getCalendarOverlay 会在 EventsEmit 后立即进入 select 等待，channel 不应积压。
func (a *App) SubmitWatermark(base64Data string) {
	select {
	case a.wmChan <- base64Data:
		slog.Debug("SubmitWatermark: data delivered", "size", len(base64Data))
	default:
		slog.Warn("SubmitWatermark: channel full, receiver may not be ready — dropping data")
	}
}
