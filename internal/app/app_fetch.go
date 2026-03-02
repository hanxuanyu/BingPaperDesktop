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

// FetchToday 获取今日壁纸并根据配置决定是否应用。
// 由调度器或手动触发。
func (a *App) FetchToday(screenW, screenH int, dpr float64) (CurrentResult, error) {
	a.fetchMu.Lock()
	defer a.fetchMu.Unlock()

	cfg, _ := store.LoadConfig()

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

	key := fmt.Sprintf("%s_%s", meta.Date, meta.Hsh)
	dayDir := filepath.Join("data", meta.Date)
	absDayDir := filepath.Join(store.GetBaseDir(), dayDir)

	a.migrateOldDataDir(meta.Date, absDayDir)

	if err := os.MkdirAll(absDayDir, 0755); err != nil {
		slog.Error("Failed to create day directory", "dir", absDayDir, "error", err)
		return CurrentResult{Error: err.Error()}, err
	}

	ext := ".jpg"
	if chosen.Format != "" {
		ext = "." + chosen.Format
	}
	relImagePath := filepath.Join(dayDir, "original"+ext)
	absImagePath := filepath.Join(store.GetBaseDir(), relImagePath)

	a.saveMetaJson(meta, absDayDir)

	if _, err := os.Stat(absImagePath); os.IsNotExist(err) {
		slog.Info("Downloading image", "url", chosen.URL, "dest", absImagePath)
		if err := bing.DownloadImage(chosen.URL, absImagePath); err != nil {
			slog.Error("Download failed", "error", err)
			return CurrentResult{Error: err.Error()}, err
		}
	}

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
