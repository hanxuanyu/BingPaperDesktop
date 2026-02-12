package store

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"BingPaperDesktop/internal/util"
)

type HistoryItem struct {
	Key           string    `json:"key"`
	Date          string    `json:"date"`
	Title         string    `json:"title"`
	Copyright     string    `json:"copyright"`
	ChosenVariant string    `json:"chosen_variant"`
	ImagePath     string    `json:"image_path"`
	WatermarkPath string    `json:"watermark_path"`
	CreatedAt     time.Time `json:"created_at"`
}

func (item *HistoryItem) Normalize() {
	item.Date = util.NormalizeDate(item.Date)
	// Key format is usually "YYYYMMDD_hash" or "YYYY-MM-DD_hash"
	parts := strings.Split(item.Key, "_")
	if len(parts) == 2 {
		parts[0] = util.NormalizeDate(parts[0])
		item.Key = strings.Join(parts, "_")
	}
}

type Index struct {
	Items []HistoryItem `json:"items"`
}

func GetIndexPath() string {
	return filepath.Join(GetDataDir(), "index.json")
}

func LoadIndex() (Index, error) {
	mu.RLock()
	defer mu.RUnlock()

	path := GetIndexPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Index{Items: []HistoryItem{}}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Index{}, err
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return Index{}, err
	}
	for i := range idx.Items {
		idx.Items[i].Normalize()
	}
	return idx, nil
}

func SaveIndex(idx Index) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GetIndexPath(), data, 0644)
}

func AddToHistory(item HistoryItem) error {
	item.Normalize()
	idx, err := LoadIndex()
	if err != nil {
		return err
	}

	// Dedup
	for i, existing := range idx.Items {
		if existing.Key == item.Key {
			// Keep original CreatedAt to avoid resetting retention timer
			item.CreatedAt = existing.CreatedAt
			idx.Items[i] = item
			return SaveIndex(idx)
		}
	}

	idx.Items = append(idx.Items, item)
	// Sort by CreatedAt descending
	sort.Slice(idx.Items, func(i, j int) bool {
		return idx.Items[i].CreatedAt.After(idx.Items[j].CreatedAt)
	})

	return SaveIndex(idx)
}

func DeleteFromHistory(key string) error {
	idx, err := LoadIndex()
	if err != nil {
		return err
	}

	newItems := []HistoryItem{}
	var pathToDelete string
	for _, item := range idx.Items {
		if item.Key == key {
			pathToDelete = item.ImagePath
			continue
		}
		newItems = append(newItems, item)
	}

	if pathToDelete != "" {
		// Delete directory
		dir := filepath.Dir(filepath.Join(GetBaseDir(), pathToDelete))
		slog.Info("Deleting history directory", "dir", dir)
		os.RemoveAll(dir)
		idx.Items = newItems
		return SaveIndex(idx)
	}

	return nil
}

func ClearHistory() error {
	idx, err := LoadIndex()
	if err != nil {
		return err
	}

	for _, item := range idx.Items {
		if item.ImagePath == "" {
			continue
		}
		dir := filepath.Dir(filepath.Join(GetBaseDir(), item.ImagePath))
		// 只有当路径包含 data 目录时才执行删除，防止误删基础目录
		if strings.Contains(dir, filepath.Join(GetBaseDir(), "data")) {
			os.RemoveAll(dir)
		}
	}

	return SaveIndex(Index{Items: []HistoryItem{}})
}

func CleanupByRetainDays(days int) (int, error) {
	if days <= 0 {
		return 0, nil
	}

	idx, err := LoadIndex()
	if err != nil {
		return 0, err
	}

	now := time.Now()
	threshold := now.AddDate(0, 0, -days)
	slog.Info("Starting cleanup", "days", days, "threshold", threshold.Format("2006-01-02"))

	newItems := []HistoryItem{}
	deletedCount := 0
	deletedDirs := make(map[string]bool)

	for _, item := range idx.Items {
		shouldDelete := false
		itemDate, err := time.ParseInLocation("2006-01-02", item.Date, time.Local)
		if err == nil {
			// Compare using image date
			if itemDate.Before(time.Date(threshold.Year(), threshold.Month(), threshold.Day(), 0, 0, 0, 0, time.Local)) {
				shouldDelete = true
			}
		} else {
			// Fallback to CreatedAt if date parsing fails
			if item.CreatedAt.Before(threshold) {
				shouldDelete = true
			}
		}

		if shouldDelete {
			if item.ImagePath != "" {
				dir := filepath.Dir(filepath.Join(GetBaseDir(), item.ImagePath))
				// Ensure we are only deleting within data directory
				if !deletedDirs[dir] && strings.Contains(dir, filepath.Join(GetBaseDir(), "data")) {
					slog.Info("Cleaning up old wallpaper directory", "dir", dir, "itemDate", item.Date)
					os.RemoveAll(dir)
					deletedDirs[dir] = true
				}
			}
			deletedCount++
			continue
		}
		newItems = append(newItems, item)
	}

	if deletedCount > 0 {
		idx.Items = newItems
		if err := SaveIndex(idx); err != nil {
			return deletedCount, err
		}
		slog.Info("Cleanup completed", "deletedCount", deletedCount)
	} else {
		slog.Info("No records found to clean up")
	}

	return deletedCount, nil
}
