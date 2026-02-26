package app

import (
	"fmt"
	"path/filepath"
	"time"

	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"BingPaperDesktop/internal/bing"
	"BingPaperDesktop/internal/overlay"
	"BingPaperDesktop/internal/store"
)

// normRatioAndSuffix 将目标比例归一化为 16:9 或 4:3，返回用于文件名的后缀（_16_9 或 _4_3）。
func normRatioAndSuffix(targetRatio float64) (normRatio float64, ratioSuffix string) {
	if targetRatio < 1.5 {
		return 1.333333, "_4_3"
	}
	return 1.777777, "_16_9"
}

// ensureWatermarkOverlay 确保特定比例（16:9 或 4:3）的元数据水印叠加图（PNG）已生成并保存。
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

// SubmitWatermark 接收前端 Canvas 渲染完成的 Base64 图片数据，投递到水印处理 channel。
func (a *App) SubmitWatermark(base64Data string) {
	select {
	case a.wmChan <- base64Data:
		slog.Debug("SubmitWatermark: data delivered", "size", len(base64Data))
	default:
		slog.Warn("SubmitWatermark: channel full, receiver may not be ready — dropping data")
	}
}
