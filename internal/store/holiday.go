package store

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HolidayDay struct {
	Name     string `json:"name"`
	Date     string `json:"date"`
	IsOffDay bool   `json:"isOffDay"`
}

type HolidayData struct {
	Year int          `json:"year"`
	Days []HolidayDay `json:"days"`
}

func GetHolidayPath(year int) string {
	return filepath.Join(GetDataDir(), "holiday", fmt.Sprintf("%d.json", year))
}

func EnsureHoliday(year int) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	return EnsureHolidayWithConfig(year, cfg, false)
}

func EnsureHolidayWithConfig(year int, cfg Config, force bool) error {
	cfg = NormalizeConfig(cfg)
	if shouldUseEmbeddedHoliday(cfg) {
		if ok, err := EnsureEmbeddedHolidayCache(year, force); err != nil {
			return err
		} else if ok {
			return nil
		}
	}
	return CheckAndDownloadHolidayWithConfig(year, cfg, force)
}

func CheckAndDownloadHoliday(year int, force bool) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	return CheckAndDownloadHolidayWithConfig(year, cfg, force)
}

func CheckAndDownloadHolidayWithConfig(year int, cfg Config, force bool) error {
	cfg = NormalizeConfig(cfg)
	path := GetHolidayPath(year)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil // 已存在
		}
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	url := BuildHolidayAPIURL(cfg.HolidayApiUrl, year)

	// 使用带超时的 HTTP 客户端，防止网络异常时永久阻塞
	slog.Info("Downloading holiday data", "year", year, "url", url, "force", force, "dest", path)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("holiday download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download holiday data: %s", resp.Status)
	}

	tmpPath := path + ".part"
	_ = os.Remove(tmpPath)

	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err = out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	holiday, err := decodeHolidayData(data, year)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("downloaded holiday data is invalid: %w", err)
	}

	if err := replaceFile(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	slog.Info("Holiday data downloaded", "year", year, "days", len(holiday.Days), "path", path)
	return nil
}

func EnsureEmbeddedHolidayCache(year int, force bool) (bool, error) {
	path := GetHolidayPath(year)
	if !force {
		if data, err := os.ReadFile(path); err == nil {
			holiday, decodeErr := decodeHolidayData(data, year)
			if decodeErr == nil {
				slog.Info("Holiday data cache exists", "year", year, "days", len(holiday.Days), "path", path, "source", "cache")
				return true, nil
			}
			slog.Warn("Holiday data cache invalid, restoring embedded data", "year", year, "path", path, "error", decodeErr)
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}

	holiday, data, err := LoadEmbeddedHoliday(year)
	if err != nil {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	tmpPath := path + ".part"
	_ = os.Remove(tmpPath)
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		_ = os.Remove(tmpPath)
		return false, err
	}
	if err := replaceFile(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return false, err
	}
	slog.Info("Holiday data restored from embedded cache", "year", year, "days", len(holiday.Days), "path", path)
	return true, nil
}

func shouldUseEmbeddedHoliday(cfg Config) bool {
	return NormalizeHolidayAPIURL(cfg.HolidayApiUrl) == DefaultHolidayUrl
}

func BuildHolidayAPIURL(apiURL string, year int) string {
	yearText := fmt.Sprintf("%d", year)
	apiURL = NormalizeHolidayAPIURL(apiURL)
	if strings.Contains(apiURL, "%d") {
		return fmt.Sprintf(apiURL, year)
	}
	replacer := strings.NewReplacer(
		"yyyy", yearText,
		"YYYY", yearText,
		"{year}", yearText,
		"{yyyy}", yearText,
		"{YYYY}", yearText,
	)
	return replacer.Replace(apiURL)
}

func decodeHolidayData(data []byte, year int) (*HolidayData, error) {
	var holiday HolidayData
	if err := json.Unmarshal(data, &holiday); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if holiday.Year != 0 && holiday.Year != year {
		return nil, fmt.Errorf("year mismatch: want %d, got %d", year, holiday.Year)
	}
	return &holiday, nil
}

func replaceFile(tmpPath, path string) error {
	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpPath, path)
}

func LoadHoliday(year int) (*HolidayData, error) {
	path := GetHolidayPath(year)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var holiday HolidayData
	if err := json.Unmarshal(data, &holiday); err != nil {
		return nil, err
	}

	return &holiday, nil
}
