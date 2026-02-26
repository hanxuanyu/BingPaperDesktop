package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

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

// GetCurrentItem 获取当前应用的壁纸信息，通常由前端主动拉取以进行同步。
func (a *App) GetCurrentItem() (CurrentResult, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.lastFetch == nil {
		return CurrentResult{Success: false, Error: "no item fetched yet"}, nil
	}
	return *a.lastFetch, nil
}

func (a *App) ListHistory() ([]store.HistoryItem, error) {
	idx, err := store.LoadIndex()
	if err != nil {
		return nil, err
	}
	return idx.Items, nil
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

func (a *App) Quit() {
	runtime.Quit(a.ctx)
}
