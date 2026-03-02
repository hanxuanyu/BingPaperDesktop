package app

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"time"

	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"BingPaperDesktop/internal/bing"
	"BingPaperDesktop/internal/overlay"
	"BingPaperDesktop/internal/store"
)

// ensureWatermarkOverlay 确保特定屏幕分辨率的元数据水印叠加图（PNG）已生成并保存。
func (a *App) ensureWatermarkOverlay(meta *bing.Meta, chosen bing.Variant, dayDir string, screenW, screenH int) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

	if screenW <= 0 {
		screenW = 1920
	}
	if screenH <= 0 {
		screenH = 1080
	}

	targetRatio := float64(screenW) / float64(screenH)

	relPath := filepath.Join(dayDir, "watermark_cache_"+fmt.Sprintf("%dx%d", screenW, screenH)+".png")
	absPath := filepath.Join(store.GetBaseDir(), relPath)

	if _, err := os.Stat(absPath); err == nil {
		logOverlayImageInfo(absPath, "watermark", screenW, screenH, targetRatio, true)
		return relPath
	}

	slog.Info("Requesting frontend to render watermark overlay",
		"size", fmt.Sprintf("%dx%d", screenW, screenH),
		"ratio", targetRatio)

	runtime.EventsEmit(a.ctx, "render-watermark", OverlayRequest{
		Title:           meta.Title,
		Date:            meta.Date,
		Copyright:       meta.Copyright,
		Variant:         chosen.Variant,
		EnableWatermark: true,
		EnableCalendar:  false,
		OnlyOverlay:     true,
		Width:           screenW,
		Height:          screenH,
		TargetRatio:     targetRatio,
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
		logOverlayImageInfo(absPath, "watermark", screenW, screenH, targetRatio, false)
		return relPath
	case <-time.After(10 * time.Second):
		slog.Error("Watermark overlay processing timeout — frontend did not respond in time",
			"size", fmt.Sprintf("%dx%d", screenW, screenH), "ratio", targetRatio)
		return ""
	}
}

// getCalendarOverlay 获取当日的日历叠加层，按日期和屏幕分辨率缓存。
func (a *App) getCalendarOverlay(width, height int, cfg store.Config) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

	if width <= 0 {
		width = 1920
	}
	if height <= 0 {
		height = 1080
	}

	targetRatio := float64(width) / float64(height)

	today := time.Now().Format("2006-01-02")
	dayDir := filepath.Join("data", today)
	absDayDir := filepath.Join(store.GetBaseDir(), dayDir)
	_ = os.MkdirAll(absDayDir, 0755)

	suffix := ""
	if cfg.EnableHoliday {
		suffix = "h"
	}

	relPath := filepath.Join(dayDir, "calendar_cache_"+fmt.Sprintf("%dx%d", width, height)+suffix+".png")
	absPath := filepath.Join(store.GetBaseDir(), relPath)

	if _, err := os.Stat(absPath); err == nil {
		logOverlayImageInfo(absPath, "calendar", width, height, targetRatio, true)
		return absPath
	}

	slog.Info("Requesting frontend to render calendar overlay",
		"date", today,
		"size", fmt.Sprintf("%dx%d", width, height),
		"ratio", targetRatio)

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
		TargetRatio:     targetRatio,
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
		logOverlayImageInfo(absPath, "calendar", width, height, targetRatio, false)
		return absPath
	case <-time.After(10 * time.Second):
		slog.Error("Calendar overlay processing timeout — frontend did not respond in time",
			"date", today, "size", fmt.Sprintf("%dx%d", width, height))
		return ""
	}
}

func logOverlayImageInfo(absPath, kind string, requestedW, requestedH int, targetRatio float64, cacheHit bool) {
	file, err := os.Open(absPath)
	if err != nil {
		slog.Warn("Overlay image info skipped: open failed", "kind", kind, "path", absPath, "error", err)
		return
	}
	defer file.Close()

	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		slog.Warn("Overlay image info skipped: decode failed", "kind", kind, "path", absPath, "error", err)
		return
	}

	status := "generated"
	if cacheHit {
		status = "cache-hit"
	}

	slog.Info("Overlay image ready",
		"kind", kind,
		"status", status,
		"path", absPath,
		"format", format,
		"requestedSize", fmt.Sprintf("%dx%d", requestedW, requestedH),
		"actualSize", fmt.Sprintf("%dx%d", cfg.Width, cfg.Height),
		"targetRatio", targetRatio,
	)
}

// SubmitWatermark 接收前端 Canvas 渲染完成的 Base64 图片数据，投递到水印处理 channel。
func (a *App) SubmitWatermark(base64Data string) {
	select {
	case a.wmChan <- base64Data:
		slog.Debug("SubmitWatermark: data delivered", "size", len(base64Data))
	default:
		slog.Warn("SubmitWatermark: channel full, receiver may not be ready — dropping data")
	}
}
