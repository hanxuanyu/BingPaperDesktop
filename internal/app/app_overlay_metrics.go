package app

import (
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"BingPaperDesktop/internal/bing"
	"BingPaperDesktop/internal/store"
)

func (a *App) ensureCombinedOverlay(meta *bing.Meta, chosen bing.Variant, dayDir string, canvasW, canvasH, targetW, targetH int, targetRatio float64, cfg store.Config) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

	canvasW, canvasH, targetW, targetH, targetRatio = normalizeOverlayGeometry(canvasW, canvasH, targetW, targetH, targetRatio)

	suffix := ""
	if cfg.EnableHoliday {
		suffix = "_h"
	}
	today := time.Now().Format("2006-01-02")
	cacheName := "overlay_cache_v2_" + fmt.Sprintf("c%dx%d_t%dx%d_cal%s", canvasW, canvasH, targetW, targetH, today) + suffix + ".png"
	relPath := filepath.Join(dayDir, cacheName)
	absPath := filepath.Join(store.GetBaseDir(), relPath)

	if _, err := os.Stat(absPath); err == nil {
		logOverlayImageInfo(absPath, "combined", canvasW, canvasH, targetRatio, true)
		return absPath
	}

	var holidayData []store.HolidayDay
	holidayLoadStart := time.Now()
	if cfg.EnableHoliday {
		hData, err := store.LoadHoliday(time.Now().Year())
		if err == nil {
			holidayData = hData.Days
		}
	}
	holidayLoadDuration := time.Since(holidayLoadStart)

	requestID := newOverlayRequestID("combined")
	slog.Info("Requesting frontend to render combined overlay",
		"requestID", requestID,
		"canvasSize", fmt.Sprintf("%dx%d", canvasW, canvasH),
		"targetSize", fmt.Sprintf("%dx%d", targetW, targetH),
		"ratio", targetRatio,
		"holidayLoadDuration", holidayLoadDuration,
		"holidayDataCount", len(holidayData),
	)

	result, ok := a.requestOverlayRender("combined", requestID, OverlayRequest{
		RequestID:       requestID,
		Title:           meta.Title,
		Date:            meta.Date,
		CalendarDate:    today,
		Copyright:       meta.Copyright,
		Variant:         chosen.Variant,
		EnableWatermark: cfg.OverlayMetadata,
		EnableCalendar:  cfg.EnableCalendar,
		HolidayData:     holidayData,
		OnlyOverlay:     true,
		Width:           canvasW,
		Height:          canvasH,
		TargetRatio:     targetRatio,
	})
	if !ok {
		return ""
	}
	if !a.saveOverlayResult("combined", result, absPath, canvasW, canvasH, targetRatio) {
		return ""
	}
	return absPath
}

func normalizeOverlayGeometry(canvasW, canvasH, targetW, targetH int, targetRatio float64) (int, int, int, int, float64) {
	if canvasW <= 0 {
		canvasW = 1920
	}
	if canvasH <= 0 {
		canvasH = 1080
	}
	if targetW <= 0 {
		targetW = canvasW
	}
	if targetH <= 0 {
		targetH = canvasH
	}
	if targetRatio <= 0 && targetH > 0 {
		targetRatio = float64(targetW) / float64(targetH)
	}
	return canvasW, canvasH, targetW, targetH, targetRatio
}

func newOverlayRequestID(kind string) string {
	return fmt.Sprintf("%s-%d-%06d", kind, time.Now().UnixNano(), rand.Intn(1000000))
}

func logFrontendOverlayMetrics(kind string, metrics OverlayRenderMetrics) {
	slog.Info("Frontend overlay render metrics",
		"kind", kind,
		"requestID", metrics.RequestID,
		"totalMs", metrics.TotalMs,
		"setupMs", metrics.SetupMs,
		"imageLoadMs", metrics.ImageLoadMs,
		"drawWatermarkMs", metrics.DrawWatermarkMs,
		"drawCalendarMs", metrics.DrawCalendarMs,
		"encodeMs", metrics.EncodeMs,
		"size", fmt.Sprintf("%dx%d", metrics.Width, metrics.Height),
		"pixelCount", metrics.PixelCount,
		"dataURLBytes", metrics.DataURLBytes,
		"onlyOverlay", metrics.OnlyOverlay,
		"enableWatermark", metrics.EnableWatermark,
		"enableCalendar", metrics.EnableCalendar,
		"hasHolidayData", metrics.HasHolidayData,
		"holidayDataCount", metrics.HolidayDataCount,
	)
}
