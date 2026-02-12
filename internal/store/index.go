package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
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
	idx, err := LoadIndex()
	if err != nil {
		return err
	}

	// Dedup
	for i, existing := range idx.Items {
		if existing.Key == item.Key {
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
	var itemToDelete *HistoryItem
	for _, item := range idx.Items {
		if item.Key == key {
			itemToDelete = &item
			continue
		}
		newItems = append(newItems, item)
	}

	if itemToDelete != nil {
		// Delete directory
		dir := filepath.Dir(filepath.Join(GetBaseDir(), itemToDelete.ImagePath))
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
		dir := filepath.Dir(filepath.Join(GetBaseDir(), item.ImagePath))
		os.RemoveAll(dir)
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

	threshold := time.Now().AddDate(0, 0, -days)
	newItems := []HistoryItem{}
	deletedCount := 0

	for _, item := range idx.Items {
		if item.CreatedAt.Before(threshold) {
			dir := filepath.Dir(filepath.Join(GetBaseDir(), item.ImagePath))
			os.RemoveAll(dir)
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
	}

	return deletedCount, nil
}
