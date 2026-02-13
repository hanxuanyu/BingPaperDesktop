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
	ApiType         string `json:"api_type"` // "custom" | "bing"
	BingApiUrl      string `json:"bing_api_url"`
	CustomApiUrl    string `json:"custom_api_url"`
	ApiMetaUrl      string `json:"api_meta_url"` // Deprecated, kept for compatibility
	AutoApply       bool   `json:"auto_apply"`
	OverlayMetadata bool   `json:"overlay_metadata"`
	ForceUHD        bool   `json:"force_uhd"`
	ScheduleMode    string `json:"schedule_mode"` // "off" | "daily" | "interval"
	DailyTime       string `json:"daily_time"`
	IntervalMinutes int    `json:"interval_minutes"`
	RetainDays      int    `json:"retain_days"`
	RandomHistory   bool   `json:"random_history"`
	AutoStart       bool   `json:"auto_start"`
	HideDockIcon    bool   `json:"hide_dock_icon"`
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

const (
	DefaultBingUrl   = "https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=1&uhd=1&mkt=zh-CN"
	DefaultCustomUrl = "https://bing.coding.icu/api/v1/image/today/meta"
)

func DefaultConfig() Config {
	return Config{
		ApiType:         "bing",
		BingApiUrl:      DefaultBingUrl,
		CustomApiUrl:    DefaultCustomUrl,
		ApiMetaUrl:      DefaultBingUrl,
		AutoApply:       false,
		OverlayMetadata: false,
		ForceUHD:        false,
		ScheduleMode:    "daily",
		DailyTime:       "08:30",
		IntervalMinutes: 60,
		RetainDays:      0,
		RandomHistory:   false,
		AutoStart:       false,
		HideDockIcon:    false,
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

	// Migration for older versions
	migrated := false
	if cfg.BingApiUrl == "" {
		if cfg.ApiType == "bing" && cfg.ApiMetaUrl != "" {
			cfg.BingApiUrl = cfg.ApiMetaUrl
		} else {
			cfg.BingApiUrl = DefaultBingUrl
		}
		migrated = true
	}
	if cfg.CustomApiUrl == "" {
		if cfg.ApiType == "custom" && cfg.ApiMetaUrl != "" {
			cfg.CustomApiUrl = cfg.ApiMetaUrl
		} else {
			cfg.CustomApiUrl = DefaultCustomUrl
		}
		migrated = true
	}

	if migrated {
		// Save migrated config to avoid re-migration
		go func(c Config) {
			SaveConfig(c)
		}(cfg)
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
