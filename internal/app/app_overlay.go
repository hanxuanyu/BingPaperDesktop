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

// ensureWatermarkOverlay 确保元数据水印叠加图（PNG）已生成并保存。
// canvasW/canvasH: 叠加层渲染尺寸（应与原图一致）
// targetW/targetH: 目标显示器尺寸（用于计算可见安全区域）
func (a *App) ensureWatermarkOverlay(meta *bing.Meta, chosen bing.Variant, dayDir string, canvasW, canvasH, targetW, targetH int, targetRatio float64) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

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
	if targetRatio <= 0 {
		targetRatio = float64(targetW) / float64(targetH)
	}

	relPath := filepath.Join(dayDir, "watermark_cache_"+fmt.Sprintf("c%dx%d_t%dx%d", canvasW, canvasH, targetW, targetH)+".png")
	absPath := filepath.Join(store.GetBaseDir(), relPath)

	if _, err := os.Stat(absPath); err == nil {
		logOverlayImageInfo(absPath, "watermark", canvasW, canvasH, targetRatio, true)
		return relPath
	}

	slog.Info("Requesting frontend to render watermark overlay",
		"canvasSize", fmt.Sprintf("%dx%d", canvasW, canvasH),
		"targetSize", fmt.Sprintf("%dx%d", targetW, targetH),
		"ratio", targetRatio)

	runtime.EventsEmit(a.ctx, "render-watermark", OverlayRequest{
		Title:           meta.Title,
		Date:            meta.Date,
		Copyright:       meta.Copyright,
		Variant:         chosen.Variant,
		EnableWatermark: true,
		EnableCalendar:  false,
		OnlyOverlay:     true,
		Width:           canvasW,
		Height:          canvasH,
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
		logOverlayImageInfo(absPath, "watermark", canvasW, canvasH, targetRatio, false)
		return relPath
	case <-time.After(10 * time.Second):
		slog.Error("Watermark overlay processing timeout — frontend did not respond in time",
			"canvasSize", fmt.Sprintf("%dx%d", canvasW, canvasH),
			"targetSize", fmt.Sprintf("%dx%d", targetW, targetH),
			"ratio", targetRatio)
		return ""
	}
}

// getCalendarOverlay 获取当日的日历叠加层，按日期和尺寸缓存。
// canvasW/canvasH: 叠加层渲染尺寸（应与原图一致）
// targetW/targetH: 目标显示器尺寸（用于计算可见安全区域）
func (a *App) getCalendarOverlay(canvasW, canvasH, targetW, targetH int, targetRatio float64, cfg store.Config) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

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
	if targetRatio <= 0 {
		targetRatio = float64(targetW) / float64(targetH)
	}

	today := time.Now().Format("2006-01-02")
	dayDir := filepath.Join("data", today)
	absDayDir := filepath.Join(store.GetBaseDir(), dayDir)
	_ = os.MkdirAll(absDayDir, 0755)

	suffix := ""
	if cfg.EnableHoliday {
		suffix = "h"
	}

	relPath := filepath.Join(dayDir, "calendar_cache_"+fmt.Sprintf("c%dx%d_t%dx%d", canvasW, canvasH, targetW, targetH)+suffix+".png")
	absPath := filepath.Join(store.GetBaseDir(), relPath)

	if _, err := os.Stat(absPath); err == nil {
		logOverlayImageInfo(absPath, "calendar", canvasW, canvasH, targetRatio, true)
		return absPath
	}

	slog.Info("Requesting frontend to render calendar overlay",
		"date", today,
		"canvasSize", fmt.Sprintf("%dx%d", canvasW, canvasH),
		"targetSize", fmt.Sprintf("%dx%d", targetW, targetH),
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
		Width:           canvasW,
		Height:          canvasH,
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
		logOverlayImageInfo(absPath, "calendar", canvasW, canvasH, targetRatio, false)
		return absPath
	case <-time.After(10 * time.Second):
		slog.Error("Calendar overlay processing timeout — frontend did not respond in time",
			"date", today,
			"canvasSize", fmt.Sprintf("%dx%d", canvasW, canvasH),
			"targetSize", fmt.Sprintf("%dx%d", targetW, targetH))
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

	actualRatio := 0.0
	if cfg.Height > 0 {
		actualRatio = float64(cfg.Width) / float64(cfg.Height)
	}
	ratioDelta := actualRatio - targetRatio

	slog.Info("Overlay image ready",
		"kind", kind,
		"status", status,
		"path", absPath,
		"format", format,
		"requestedSize", fmt.Sprintf("%dx%d", requestedW, requestedH),
		"actualSize", fmt.Sprintf("%dx%d", cfg.Width, cfg.Height),
		"targetRatio", targetRatio,
		"actualRatio", actualRatio,
		"ratioDelta", ratioDelta,
	)
}

func readImageSize(absPath string) (int, int, error) {
	file, err := os.Open(absPath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, fmt.Errorf("invalid image size: %dx%d", cfg.Width, cfg.Height)
	}

	return cfg.Width, cfg.Height, nil
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
