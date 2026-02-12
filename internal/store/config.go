package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Config struct {
	ApiMetaUrl        string `json:"api_meta_url"`
	AutoApply         bool   `json:"auto_apply"`
	OverlayMetadata   bool   `json:"overlay_metadata"`
	PreferAspectMatch bool   `json:"prefer_aspect_match"`
	ForceUHD          bool   `json:"force_uhd"`
	ScheduleMode      string `json:"schedule_mode"` // "off" | "daily" | "interval"
	DailyTime         string `json:"daily_time"`
	IntervalMinutes   int    `json:"interval_minutes"`
	RetainDays        int    `json:"retain_days"`
}

var (
	baseDir string
	mu      sync.RWMutex
)

func Init() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	realPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		realPath = exePath
	}
	baseDir = filepath.Dir(realPath)

	// 在 macOS 上，如果运行在 .app 包内，或者当前目录不可写，则切换到用户配置目录
	// 这是为了防止在只读目录（如 /Applications 或 DMG）下运行时闪退
	isInsideApp := runtime.GOOS == "darwin" && strings.Contains(baseDir, ".app/Contents/MacOS")

	writable := true
	testFile := filepath.Join(baseDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		writable = false
	} else {
		_ = os.Remove(testFile)
	}

	if isInsideApp || !writable {
		configDir, err := os.UserConfigDir()
		if err == nil {
			baseDir = filepath.Join(configDir, "BingPaperDesktop")
		}
	}

	// 确保基础目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	// Ensure directories exist
	dirs := []string{"data", "logs"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(baseDir, d), 0755); err != nil {
			return err
		}
	}
	return nil
}

func GetBaseDir() string {
	return baseDir
}

func GetConfigPath() string {
	return filepath.Join(baseDir, "config.json")
}

func GetDataDir() string {
	return filepath.Join(baseDir, "data")
}

func GetLogsDir() string {
	return filepath.Join(baseDir, "logs")
}

func DefaultConfig() Config {
	return Config{
		ApiMetaUrl:        "https://bing.coding.icu/api/v1/image/today/meta",
		AutoApply:         false,
		OverlayMetadata:   false,
		PreferAspectMatch: true,
		ForceUHD:          false,
		ScheduleMode:      "daily",
		DailyTime:         "08:30",
		IntervalMinutes:   60,
		RetainDays:        0,
	}
}

func LoadConfig() (Config, error) {
	mu.RLock()
	defer mu.RUnlock()

	path := GetConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := saveConfigLocked(cfg); err != nil {
			return cfg, err
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func SaveConfig(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()
	return saveConfigLocked(cfg)
}

func saveConfigLocked(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(GetConfigPath(), data, 0644)
}
