package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"BingPaperDesktop/internal/bing"
	"BingPaperDesktop/internal/overlay"
	"BingPaperDesktop/internal/store"
	"BingPaperDesktop/internal/wallpaper"
)

// MonitorWallpaperInfo 单个显示器的壁纸信息，供前端展示。
type MonitorWallpaperInfo struct {
	MonitorID    int               `json:"monitor_id"`
	MonitorName  string            `json:"monitor_name"`
	HistoryItem  store.HistoryItem `json:"history_item"`
	ThumbnailURL string            `json:"thumbnail_url"`
}

func (a *App) GetMonitors() ([]wallpaper.Monitor, error) {
	return wallpaper.GetMonitors()
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

	monitors, err := wallpaper.GetMonitors()
	if err != nil || len(monitors) == 0 {
		slog.Warn("Failed to get monitors, falling back to single monitor", "error", err)
		monitors = []wallpaper.Monitor{{ID: 0, Width: screenW, Height: screenH}}
	}

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

	a.mu.Lock()
	for _, m := range targets {
		m := m
		absOriginalPath := filepath.Join(store.GetBaseDir(), target.ImagePath)
		currentKey := a.monitorWallpapers[m.ID]
		sameImage := (currentKey == target.Key)
		monitorRatio := 0.0
		if m.Height > 0 {
			monitorRatio = float64(m.Width) / float64(m.Height)
		}

		slog.Info("ApplyHistory target prepared",
			"monitorID", m.ID,
			"monitorName", m.Name,
			"monitorSize", fmt.Sprintf("%dx%d", m.Width, m.Height),
			"monitorRatio", monitorRatio,
			"historyKey", target.Key,
			"originalWallpaper", absOriginalPath,
			"overlayMetadata", cfg.OverlayMetadata,
			"enableCalendar", cfg.EnableCalendar,
		)

		if sameImage {
			if !cfg.OverlayMetadata && !cfg.EnableCalendar {
				// 两个叠加都关闭：需要把壁纸从合成图换回原图
				slog.Debug("Same image, overlays off: setting original as wallpaper", "id", m.ID, "path", absOriginalPath)
				if err := wallpaper.SetOnMonitor(m.ID, absOriginalPath); err != nil {
					if strings.Contains(err.Error(), "IDesktopWallpaper not supported") {
						_ = wallpaper.Set(absOriginalPath)
					} else {
						slog.Error("Failed to set original wallpaper on monitor", "id", m.ID, "error", err)
						_ = wallpaper.Set(absOriginalPath)
					}
				}
				runtime.EventsEmit(a.ctx, "monitor-wallpapers-changed", a.monitorWallpapers)
				continue
			}
			go func(m wallpaper.Monitor, absOriginal string) {
				applyPath, err := a.prepareWallpaperForMonitor(target, m, cfg)
				if err != nil {
					slog.Error("Failed to prepare wallpaper for monitor", "id", m.ID, "error", err)
					return
				}
				if applyPath == absOriginal {
					return
				}
				slog.Info("Applying composited wallpaper (same image, overlay updated)", "id", m.ID, "path", applyPath)
				if err := wallpaper.SetOnMonitor(m.ID, applyPath); err != nil {
					if strings.Contains(err.Error(), "IDesktopWallpaper not supported") {
						_ = wallpaper.Set(applyPath)
					} else {
						slog.Error("Failed to set composited wallpaper on monitor", "id", m.ID, "error", err)
						_ = wallpaper.Set(applyPath)
					}
				}
				runtime.EventsEmit(a.ctx, "monitor-wallpapers-changed", a.monitorWallpapers)
			}(m, absOriginalPath)
			continue
		}

		slog.Debug("Phase 1: Setting original image as wallpaper", "id", m.ID, "path", absOriginalPath)
		_ = wallpaper.SetOnMonitor(m.ID, absOriginalPath)
		a.monitorWallpapers[m.ID] = target.Key

		if !cfg.OverlayMetadata && !cfg.EnableCalendar {
			continue
		}

		go func(m wallpaper.Monitor, absOriginal string) {
			applyPath, err := a.prepareWallpaperForMonitor(target, m, cfg)
			if err != nil {
				slog.Error("Failed to prepare wallpaper for monitor", "id", m.ID, "error", err)
				return
			}

			if applyPath == absOriginal {
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
			runtime.EventsEmit(a.ctx, "monitor-wallpapers-changed", a.monitorWallpapers)
		}(m, absOriginalPath)
	}

	a.lastFetch = &CurrentResult{Item: *target, Success: true}
	a.mu.Unlock()

	runtime.EventsEmit(a.ctx, "current-image-changed", *target)
	runtime.EventsEmit(a.ctx, "monitor-wallpapers-changed", a.monitorWallpapers)

	return nil
}

func (a *App) prepareWallpaperForMonitor(target *store.HistoryItem, m wallpaper.Monitor, cfg store.Config) (string, error) {
	absOriginalPath := filepath.Join(store.GetBaseDir(), target.ImagePath)

	renderW, renderH := m.Width, m.Height
	if renderW <= 0 {
		renderW = 1920
	}
	if renderH <= 0 {
		renderH = 1080
	}

	renderRatio := float64(renderW) / float64(renderH)

	slog.Info("Preparing wallpaper for monitor",
		"monitorID", m.ID,
		"monitorName", m.Name,
		"renderSize", fmt.Sprintf("%dx%d", renderW, renderH),
		"renderRatio", renderRatio,
		"overlayMetadata", cfg.OverlayMetadata,
		"enableCalendar", cfg.EnableCalendar,
		"imagePath", target.ImagePath,
		"originalWallpaper", absOriginalPath,
	)

	showOverlay := cfg.OverlayMetadata || cfg.EnableCalendar
	if !showOverlay {
		slog.Info("Wallpaper selected (no overlay)",
			"monitorID", m.ID,
			"path", absOriginalPath,
			"renderRatio", renderRatio,
		)
		return absOriginalPath, nil
	}

	var overlays []string

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
		wmPath := a.ensureWatermarkOverlay(tempMeta, tempChosen, dayDir, renderW, renderH)
		if wmPath != "" {
			overlays = append(overlays, filepath.Join(store.GetBaseDir(), wmPath))
		}
	}

	if cfg.EnableCalendar {
		calPath := a.getCalendarOverlay(renderW, renderH, cfg)
		if calPath != "" {
			overlays = append(overlays, calPath)
		}
	}

	if len(overlays) > 0 {
		suffix := time.Now().Unix()
		tempWallpaperPath := filepath.Join(store.GetBaseDir(), fmt.Sprintf("current_wallpaper_%d_%d.jpg", m.ID, suffix))

		if matches, err := filepath.Glob(filepath.Join(store.GetBaseDir(), fmt.Sprintf("current_wallpaper_%d_*.jpg", m.ID))); err == nil {
			for _, oldPath := range matches {
				_ = os.Remove(oldPath)
			}
		}

		if compositeErr := overlay.Composite(absOriginalPath, overlays, tempWallpaperPath); compositeErr != nil {
			return "", fmt.Errorf("failed to composite image: %w", compositeErr)
		}
		slog.Info("Wallpaper selected (composited)",
			"monitorID", m.ID,
			"path", tempWallpaperPath,
			"baseWallpaper", absOriginalPath,
			"overlayCount", len(overlays),
			"renderRatio", renderRatio,
		)
		return tempWallpaperPath, nil
	}

	slog.Info("Wallpaper selected (fallback original)",
		"monitorID", m.ID,
		"path", absOriginalPath,
		"renderRatio", renderRatio,
	)
	return absOriginalPath, nil
}
