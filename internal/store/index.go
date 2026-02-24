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

// AddToHistory 将新的壁纸条目添加到历史记录中。
// 如果 key 已存在，则更新现有条目，但保留其原始创建时间。
func AddToHistory(item HistoryItem) error {
	item.Normalize()
	idx, err := LoadIndex()
	if err != nil {
		slog.Error("Failed to load index during AddToHistory", "error", err)
		return err
	}

	// 查找是否已存在相同 Key 的记录（去重）
	for i, existing := range idx.Items {
		if existing.Key == item.Key {
			slog.Info("Updating existing history item", "key", item.Key)
			// 保留原始创建时间，以确保清理逻辑（RetainDays）能正确执行
			item.CreatedAt = existing.CreatedAt
			idx.Items[i] = item
			return SaveIndex(idx)
		}
	}

	slog.Info("Adding new item to history", "key", item.Key, "title", item.Title)
	idx.Items = append(idx.Items, item)
	// 按创建时间倒序排序（最新的在前）
	sort.Slice(idx.Items, func(i, j int) bool {
		return idx.Items[i].CreatedAt.After(idx.Items[j].CreatedAt)
	})

	return SaveIndex(idx)
}

// DeleteFromHistory 根据 Key 删除单条历史记录及其关联的文件。
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
		// 删除图片及其所在目录（整个日期的目录）
		dir := filepath.Dir(filepath.Join(GetBaseDir(), pathToDelete))
		slog.Info("Deleting history directory", "key", key, "dir", dir)
		if err := os.RemoveAll(dir); err != nil {
			slog.Warn("Failed to remove directory", "dir", dir, "error", err)
		}

		// 同时删除缩略图目录
		thumbDir := filepath.Dir(filepath.Join(GetThumbnailsDir(), pathToDelete))
		if _, err := os.Stat(thumbDir); err == nil {
			slog.Info("Deleting thumbnail directory", "dir", thumbDir)
			os.RemoveAll(thumbDir)
		}

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

		// 清理缩略图
		thumbDir := filepath.Dir(filepath.Join(GetThumbnailsDir(), item.ImagePath))
		if strings.Contains(thumbDir, GetThumbnailsDir()) {
			os.RemoveAll(thumbDir)
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

					// 同时清理对应的缩略图
					thumbDir := filepath.Dir(filepath.Join(GetThumbnailsDir(), item.ImagePath))
					if strings.Contains(thumbDir, GetThumbnailsDir()) {
						os.RemoveAll(thumbDir)
					}
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
