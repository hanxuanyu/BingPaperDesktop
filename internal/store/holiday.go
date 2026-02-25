package store

import (
	"encoding/json"
	"fmt"
	"io"
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

func CheckAndDownloadHoliday(year int, force bool) error {
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

	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	url := strings.ReplaceAll(cfg.HolidayApiUrl, "yyyy", fmt.Sprintf("%d", year))

	// 使用带超时的 HTTP 客户端，防止网络异常时永久阻塞
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("holiday download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download holiday data: %s", resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
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
