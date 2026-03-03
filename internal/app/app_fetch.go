package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"BingPaperDesktop/internal/bing"
	"BingPaperDesktop/internal/store"
	"BingPaperDesktop/internal/util"
)

type HistoryFetchProgress struct {
	Total       int    `json:"total"`
	Completed   int    `json:"completed"`
	Success     int    `json:"success"`
	Skipped     int    `json:"skipped"`
	Failed      int    `json:"failed"`
	CurrentDate string `json:"current_date"`
	Status      string `json:"status"` // running | skipped | success | failed | done
	Message     string `json:"message"`
}

// FetchToday fetches today's wallpaper and applies it based on current config.
func (a *App) FetchToday(screenW, screenH int, dpr float64) (CurrentResult, error) {
	a.fetchMu.Lock()
	defer a.fetchMu.Unlock()

	cfg, _ := store.LoadConfig()
	logicalW, logicalH, realW, realH := resolveScreenSize(screenW, screenH, dpr)

	slog.Info("FetchToday started",
		"logicalScreen", fmt.Sprintf("%dx%d", logicalW, logicalH),
		"physicalScreen", fmt.Sprintf("%dx%d", realW, realH),
		"dpr", dpr,
		"api", cfg.ApiType,
		"forceUHD", cfg.ForceUHD,
	)

	apiURL := cfg.BingApiUrl
	if cfg.ApiType == "custom" {
		apiURL = cfg.CustomApiUrl
	}
	meta, err := bing.FetchMeta(cfg.ApiType, apiURL)
	if err != nil {
		slog.Error("Failed to fetch meta", "error", err)
		return CurrentResult{Error: err.Error()}, err
	}

	item, err := a.upsertHistoryItemFromMeta(meta, realW, realH, cfg.ForceUHD)
	if err != nil {
		return CurrentResult{Error: err.Error()}, err
	}

	if cfg.AutoApply {
		if cfg.RandomHistory {
			slog.Info("Random history enabled, picking a random wallpaper from history")
			err := a.ApplyRandomHistory(realW, realH)
			if err == nil {
				a.mu.RLock()
				defer a.mu.RUnlock()
				return *a.lastFetch, nil
			}
			slog.Error("Apply random history failed, fallback to today", "error", err)
		}

		slog.Info("Auto applying wallpaper")
		_ = a.ApplyHistoryToMonitor(item.Key, -1, realW, realH)
	} else {
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

// FetchHistoryByDays pulls and stores historical wallpapers from BingPaper API.
// It builds date endpoints from today's meta endpoint and fetches [today ... today-days+1].
func (a *App) FetchHistoryByDays(days, screenW, screenH int, dpr float64, customAPIURL string) (map[string]int, error) {
	a.fetchMu.Lock()
	defer a.fetchMu.Unlock()

	result := map[string]int{
		"requested": days,
		"completed": 0,
		"success":   0,
		"skipped":   0,
		"failed":    0,
	}

	if days <= 0 {
		return result, fmt.Errorf("days must be greater than 0")
	}
	if days > 365 {
		return result, fmt.Errorf("days must be <= 365")
	}

	cfg, _ := store.LoadConfig()
	logicalW, logicalH, realW, realH := resolveScreenSize(screenW, screenH, dpr)

	apiURL := strings.TrimSpace(customAPIURL)
	if apiURL == "" {
		apiURL = strings.TrimSpace(cfg.CustomApiUrl)
	}
	if apiURL == "" {
		return result, fmt.Errorf("custom api url is empty")
	}

	existingByDate := map[string]store.HistoryItem{}
	if idx, err := store.LoadIndex(); err == nil {
		for _, item := range idx.Items {
			if item.Date == "" {
				continue
			}
			if _, exists := existingByDate[item.Date]; !exists {
				existingByDate[item.Date] = item
			}
		}
	}

	slog.Info("FetchHistoryByDays started",
		"days", days,
		"logicalScreen", fmt.Sprintf("%dx%d", logicalW, logicalH),
		"physicalScreen", fmt.Sprintf("%dx%d", realW, realH),
		"dpr", dpr,
		"customApiURL", apiURL,
		"forceUHD", cfg.ForceUHD,
	)
	a.emitHistoryFetchProgress(result, "", "running", "开始拉取历史壁纸")

	now := time.Now()
	for i := 0; i < days; i++ {
		targetDate := now.AddDate(0, 0, -i).Format("2006-01-02")
		result["completed"] = i + 1

		if hasLocalHistoryForDate(existingByDate, targetDate) {
			result["skipped"]++
			a.emitHistoryFetchProgress(result, targetDate, "skipped", "该日期已存在，已跳过")
			continue
		}

		meta, err := bing.FetchMetaByDate(apiURL, targetDate)
		if err != nil {
			result["failed"]++
			slog.Warn("Fetch history meta failed", "date", targetDate, "error", err)
			a.emitHistoryFetchProgress(result, targetDate, "failed", "获取元数据失败")
			continue
		}

		if _, err := a.upsertHistoryItemFromMeta(meta, realW, realH, cfg.ForceUHD); err != nil {
			result["failed"]++
			slog.Warn("Fetch history image failed", "date", targetDate, "error", err)
			a.emitHistoryFetchProgress(result, targetDate, "failed", "下载或保存失败")
			continue
		}

		result["success"]++
		a.emitHistoryFetchProgress(result, targetDate, "success", "拉取成功")
	}

	finalMsg := "历史壁纸拉取完成"
	if result["success"] == 0 && result["skipped"] == 0 {
		finalMsg = "历史壁纸拉取失败"
	}
	a.emitHistoryFetchProgress(result, "", "done", finalMsg)

	if result["success"] > 0 || result["skipped"] > 0 {
		runtime.EventsEmit(a.ctx, "history-updated", result)
		return result, nil
	}
	return result, fmt.Errorf("failed to fetch history for %d days", days)
}

func (a *App) emitHistoryFetchProgress(result map[string]int, currentDate, status, message string) {
	runtime.EventsEmit(a.ctx, "history-fetch-progress", HistoryFetchProgress{
		Total:       result["requested"],
		Completed:   result["completed"],
		Success:     result["success"],
		Skipped:     result["skipped"],
		Failed:      result["failed"],
		CurrentDate: currentDate,
		Status:      status,
		Message:     message,
	})
}

func hasLocalHistoryForDate(existingByDate map[string]store.HistoryItem, date string) bool {
	if item, ok := existingByDate[date]; ok && item.ImagePath != "" {
		absPath := filepath.Join(store.GetBaseDir(), item.ImagePath)
		if st, err := os.Stat(absPath); err == nil && st.Size() > 0 {
			return true
		}
	}

	dayDir := filepath.Join(store.GetBaseDir(), "data", date)
	if matches, err := filepath.Glob(filepath.Join(dayDir, "original.*")); err == nil && len(matches) > 0 {
		return true
	}
	return false
}

func resolveScreenSize(screenW, screenH int, dpr float64) (logicalW, logicalH, realW, realH int) {
	if screenW <= 0 || screenH <= 0 {
		screenW = 1920
		screenH = 1080
	}
	if dpr <= 0 {
		dpr = 1
	}

	realW = int(float64(screenW) * dpr)
	realH = int(float64(screenH) * dpr)
	if realW <= 0 || realH <= 0 {
		realW = 1920
		realH = 1080
	}
	return screenW, screenH, realW, realH
}

func (a *App) upsertHistoryItemFromMeta(meta *bing.Meta, realW, realH int, forceUHD bool) (store.HistoryItem, error) {
	if meta == nil {
		return store.HistoryItem{}, fmt.Errorf("meta is nil")
	}

	meta.Date = util.NormalizeDate(meta.Date)
	if _, err := time.Parse("2006-01-02", meta.Date); err != nil {
		return store.HistoryItem{}, fmt.Errorf("invalid meta date %q: %w", meta.Date, err)
	}

	chosen := bing.SelectVariant(meta, realW, realH, forceUHD)
	if chosen.URL == "" {
		return store.HistoryItem{}, fmt.Errorf("no downloadable image url for date %s", meta.Date)
	}
	slog.Info("Selected variant", "date", meta.Date, "variant", chosen.Variant, "url", chosen.URL)

	key := fmt.Sprintf("%s_%s", meta.Date, meta.Hsh)
	dayDir := filepath.Join("data", meta.Date)
	absDayDir := filepath.Join(store.GetBaseDir(), dayDir)

	a.migrateOldDataDir(meta.Date, absDayDir)

	if err := os.MkdirAll(absDayDir, 0755); err != nil {
		slog.Error("Failed to create day directory", "dir", absDayDir, "error", err)
		return store.HistoryItem{}, err
	}

	ext := ".jpg"
	if chosen.Format != "" {
		ext = "." + strings.TrimPrefix(strings.ToLower(chosen.Format), ".")
	}
	relImagePath := filepath.Join(dayDir, "original"+ext)
	absImagePath := filepath.Join(store.GetBaseDir(), relImagePath)

	a.saveMetaJson(meta, absDayDir)

	if _, err := os.Stat(absImagePath); os.IsNotExist(err) {
		slog.Info("Downloading image", "url", chosen.URL, "dest", absImagePath)
		if err := bing.DownloadImage(chosen.URL, absImagePath); err != nil {
			slog.Error("Download failed", "date", meta.Date, "error", err)
			return store.HistoryItem{}, err
		}
	}

	createdAt := time.Now()
	if parsed, err := time.ParseInLocation("2006-01-02", meta.Date, time.Local); err == nil {
		createdAt = parsed
	}

	item := store.HistoryItem{
		Key:           key,
		Date:          meta.Date,
		Title:         meta.Title,
		Copyright:     meta.Copyright,
		ChosenVariant: chosen.Variant,
		ImagePath:     relImagePath,
		CreatedAt:     createdAt,
	}

	if err := store.AddToHistory(item); err != nil {
		slog.Error("Failed to save history", "key", key, "error", err)
	}

	return item, nil
}

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
