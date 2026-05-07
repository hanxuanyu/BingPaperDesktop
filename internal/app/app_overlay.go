package app

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"BingPaperDesktop/internal/bing"
	"BingPaperDesktop/internal/overlay"
	"BingPaperDesktop/internal/store"
)

func (a *App) ensureWatermarkOverlay(meta *bing.Meta, chosen bing.Variant, dayDir string, canvasW, canvasH, targetW, targetH int, targetRatio float64) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

	canvasW, canvasH, targetW, targetH, targetRatio = normalizeOverlayGeometry(canvasW, canvasH, targetW, targetH, targetRatio)

	relPath := filepath.Join(dayDir, "watermark_cache_v2_"+fmt.Sprintf("c%dx%d_t%dx%d", canvasW, canvasH, targetW, targetH)+".png")
	absPath := filepath.Join(store.GetBaseDir(), relPath)

	if _, err := os.Stat(absPath); err == nil {
		logOverlayImageInfo(absPath, "watermark", canvasW, canvasH, targetRatio, true)
		return relPath
	}

	requestID := newOverlayRequestID("watermark")
	slog.Info("Requesting frontend to render watermark overlay",
		"requestID", requestID,
		"canvasSize", fmt.Sprintf("%dx%d", canvasW, canvasH),
		"targetSize", fmt.Sprintf("%dx%d", targetW, targetH),
		"ratio", targetRatio,
	)

	result, ok := a.requestOverlayRender("watermark", requestID, OverlayRequest{
		RequestID:       requestID,
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
	if !ok {
		return ""
	}

	if !a.saveOverlayResult("watermark", result, absPath, canvasW, canvasH, targetRatio) {
		return ""
	}
	return relPath
}

func (a *App) getCalendarOverlay(canvasW, canvasH, targetW, targetH int, targetRatio float64, cfg store.Config) string {
	a.wmMu.Lock()
	defer a.wmMu.Unlock()

	canvasW, canvasH, targetW, targetH, targetRatio = normalizeOverlayGeometry(canvasW, canvasH, targetW, targetH, targetRatio)

	today := time.Now().Format("2006-01-02")
	dayDir := filepath.Join("data", today)
	absDayDir := filepath.Join(store.GetBaseDir(), dayDir)
	_ = os.MkdirAll(absDayDir, 0755)

	suffix := ""
	if cfg.EnableHoliday {
		suffix = "h"
	}

	relPath := filepath.Join(dayDir, "calendar_cache_v2_"+fmt.Sprintf("c%dx%d_t%dx%d", canvasW, canvasH, targetW, targetH)+suffix+".png")
	absPath := filepath.Join(store.GetBaseDir(), relPath)

	if _, err := os.Stat(absPath); err == nil {
		logOverlayImageInfo(absPath, "calendar", canvasW, canvasH, targetRatio, true)
		return absPath
	}

	holidayData, holidayLoadDuration := loadHolidayDataForOverlay(cfg)
	requestID := newOverlayRequestID("calendar")
	slog.Info("Requesting frontend to render calendar overlay",
		"requestID", requestID,
		"date", today,
		"canvasSize", fmt.Sprintf("%dx%d", canvasW, canvasH),
		"targetSize", fmt.Sprintf("%dx%d", targetW, targetH),
		"ratio", targetRatio,
		"holidayLoadDuration", holidayLoadDuration,
		"holidayDataCount", len(holidayData),
	)

	result, ok := a.requestOverlayRender("calendar", requestID, OverlayRequest{
		RequestID:       requestID,
		Date:            today,
		CalendarDate:    today,
		EnableWatermark: false,
		EnableCalendar:  true,
		HolidayData:     holidayData,
		OnlyOverlay:     true,
		Width:           canvasW,
		Height:          canvasH,
		TargetRatio:     targetRatio,
	})
	if !ok {
		return ""
	}

	if !a.saveOverlayResult("calendar", result, absPath, canvasW, canvasH, targetRatio) {
		return ""
	}
	return absPath
}

func (a *App) requestOverlayRender(kind, requestID string, request OverlayRequest) (OverlayRenderResult, bool) {
	requestStart := time.Now()
	runtime.EventsEmit(a.ctx, "render-watermark", request)

	select {
	case result := <-a.wmChan:
		waitDuration := time.Since(requestStart)
		if result.RequestID != "" && result.RequestID != requestID {
			slog.Warn("Overlay render response request ID mismatch",
				"kind", kind,
				"expectedRequestID", requestID,
				"actualRequestID", result.RequestID,
			)
		}
		if result.Base64Data == "" {
			slog.Warn("Overlay render returned empty data", "kind", kind, "requestID", requestID, "waitDuration", waitDuration)
			return OverlayRenderResult{}, false
		}

		logFrontendOverlayMetrics(kind, result.Metrics)
		slog.Info("Overlay frontend roundtrip finished",
			"kind", kind,
			"requestID", requestID,
			"waitDuration", waitDuration,
			"dataURLBytes", len(result.Base64Data),
		)
		return result, true
	case <-time.After(10 * time.Second):
		slog.Error("Overlay processing timeout",
			"kind", kind,
			"requestID", requestID,
			"canvasSize", fmt.Sprintf("%dx%d", request.Width, request.Height),
			"targetRatio", request.TargetRatio,
			"duration", time.Since(requestStart),
		)
		return OverlayRenderResult{}, false
	}
}

func (a *App) saveOverlayResult(kind string, result OverlayRenderResult, absPath string, requestedW, requestedH int, targetRatio float64) bool {
	saveStart := time.Now()
	if err := overlay.SaveBase64Image(result.Base64Data, absPath); err != nil {
		slog.Error("Failed to save overlay", "kind", kind, "requestID", result.RequestID, "path", absPath, "error", err)
		return false
	}
	saveDuration := time.Since(saveStart)

	logOverlayImageInfo(absPath, kind, requestedW, requestedH, targetRatio, false)
	slog.Info("Overlay file save finished",
		"kind", kind,
		"requestID", result.RequestID,
		"path", absPath,
		"saveDuration", saveDuration,
		"dataURLBytes", len(result.Base64Data),
	)
	return true
}

func logOverlayImageInfo(absPath, kind string, requestedW, requestedH int, targetRatio float64, cacheHit bool) {
	statStart := time.Now()
	info, statErr := os.Stat(absPath)
	statDuration := time.Since(statStart)
	if statErr != nil {
		slog.Warn("Overlay image info skipped: stat failed", "kind", kind, "path", absPath, "duration", statDuration, "error", statErr)
		return
	}

	openStart := time.Now()
	file, err := os.Open(absPath)
	openDuration := time.Since(openStart)
	if err != nil {
		slog.Warn("Overlay image info skipped: open failed", "kind", kind, "path", absPath, "duration", openDuration, "error", err)
		return
	}
	defer file.Close()

	decodeStart := time.Now()
	cfg, format, err := image.DecodeConfig(file)
	decodeDuration := time.Since(decodeStart)
	if err != nil {
		slog.Warn("Overlay image info skipped: decode failed", "kind", kind, "path", absPath, "duration", decodeDuration, "error", err)
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
		"fileBytes", info.Size(),
		"requestedSize", fmt.Sprintf("%dx%d", requestedW, requestedH),
		"actualSize", fmt.Sprintf("%dx%d", cfg.Width, cfg.Height),
		"targetRatio", targetRatio,
		"actualRatio", actualRatio,
		"ratioDelta", ratioDelta,
		"statDuration", statDuration,
		"openDuration", openDuration,
		"decodeDuration", decodeDuration,
	)
}

func readImageSize(absPath string) (int, int, error) {
	openStart := time.Now()
	file, err := os.Open(absPath)
	openDuration := time.Since(openStart)
	if err != nil {
		slog.Warn("Image size open failed", "path", absPath, "duration", openDuration, "error", err)
		return 0, 0, err
	}
	defer file.Close()

	decodeStart := time.Now()
	cfg, format, err := image.DecodeConfig(file)
	decodeDuration := time.Since(decodeStart)
	if err != nil {
		slog.Warn("Image size decode failed", "path", absPath, "format", format, "duration", decodeDuration, "error", err)
		return 0, 0, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		err := fmt.Errorf("invalid image size: %dx%d", cfg.Width, cfg.Height)
		slog.Warn("Image size decode invalid", "path", absPath, "format", format, "duration", decodeDuration, "error", err)
		return 0, 0, err
	}

	slog.Info("Image size decoded",
		"path", absPath,
		"format", format,
		"size", fmt.Sprintf("%dx%d", cfg.Width, cfg.Height),
		"openDuration", openDuration,
		"decodeDuration", decodeDuration,
		"totalDuration", openDuration+decodeDuration,
	)
	return cfg.Width, cfg.Height, nil
}

func (a *App) SubmitWatermark(base64Data string) {
	result := OverlayRenderResult{Base64Data: base64Data}
	trimmed := strings.TrimSpace(base64Data)
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
			slog.Warn("SubmitWatermark: failed to parse structured render result, using raw data", "error", err)
			result = OverlayRenderResult{Base64Data: base64Data}
		}
	}
	if result.RequestID == "" {
		result.RequestID = result.Metrics.RequestID
	}
	if result.Base64Data == "" && !strings.HasPrefix(trimmed, "{") {
		result.Base64Data = base64Data
	}

	select {
	case a.wmChan <- result:
		slog.Debug("SubmitWatermark: data delivered",
			"requestID", result.RequestID,
			"size", len(result.Base64Data),
			"frontendTotalMs", result.Metrics.TotalMs,
		)
	default:
		slog.Warn("SubmitWatermark: channel full, receiver may not be ready, dropping data", "requestID", result.RequestID, "size", len(result.Base64Data))
	}
}

func loadHolidayDataForOverlay(cfg store.Config) ([]store.HolidayDay, time.Duration) {
	start := time.Now()
	if !cfg.EnableHoliday {
		return nil, time.Since(start)
	}
	hData, err := store.LoadHoliday(time.Now().Year())
	if err != nil {
		slog.Warn("Failed to load holiday data for overlay", "duration", time.Since(start), "error", err)
		return nil, time.Since(start)
	}
	return hData.Days, time.Since(start)
}
