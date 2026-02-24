package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	ScheduleMode    string `json:"schedule_mode"` // "off" | "daily" | "interval" | "wakeup"
	DailyTime       string `json:"daily_time"`
	IntervalMinutes int    `json:"interval_minutes"`
	RetainDays      int    `json:"retain_days"`
	RandomHistory   bool   `json:"random_history"`
	LogRetainDays   int    `json:"log_retain_days"`
	LogMaxSize      int    `json:"log_max_size"` // MB
	AutoStart       bool   `json:"auto_start"`
	HideDockIcon    bool   `json:"hide_dock_icon"`
	EnableCalendar  bool   `json:"enable_calendar"`
	EnableHoliday   bool   `json:"enable_holiday"`
	HolidayApiUrl   string `json:"holiday_api_url"`
}

var (
	baseDir    string
	hasUserDir bool
	mu         sync.RWMutex
)

func SetBaseDir(path string) {
	mu.Lock()
	defer mu.Unlock()
	baseDir = path
	hasUserDir = true
}

func Init() error {
	mu.Lock()
	defer mu.Unlock()

	return initLocked()
}

func initLocked() error {
	if !hasUserDir {
		// 检查环境变量是否指定了路径
		if envPath := os.Getenv("BING_PAPER_DATA_PATH"); envPath != "" {
			baseDir = envPath
			hasUserDir = true
		}
	}

	if !hasUserDir {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		realPath, err := filepath.EvalSymlinks(exePath)
		if err != nil {
			realPath = exePath
		}
		exeDir := filepath.Dir(realPath)

		// 默认策略
		// Windows: 默认保存在可执行文件的相对路径
		// macOS: 默认保存在用户配置目录 UserConfigDir
		if runtime.GOOS == "darwin" {
			configDir, err := os.UserConfigDir()
			if err == nil {
				baseDir = filepath.Join(configDir, "BingPaperDesktop")
			} else {
				baseDir = exeDir
			}
		} else {
			// Windows 或其他系统，默认使用可执行文件目录
			baseDir = exeDir

			// 如果是 Windows，但目录不可写，则回退到 UserConfigDir
			// 这是为了防止安装在 C:\Program Files 等受限目录时无法写入
			writable := true
			testFile := filepath.Join(baseDir, ".write_test")
			if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
				writable = false
			} else {
				_ = os.Remove(testFile)
			}

			if !writable {
				configDir, err := os.UserConfigDir()
				if err == nil {
					baseDir = filepath.Join(configDir, "BingPaperDesktop")
				}
			}
		}
	}

	// 确保基础目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	// Ensure directories exist
	dirs := []string{"data", "logs", "thumbnails"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(baseDir, d), 0755); err != nil {
			return err
		}
	}
	return nil
}

func ReInit() error {
	mu.Lock()
	defer mu.Unlock()
	return initLocked()
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

func GetThumbnailsDir() string {
	return filepath.Join(baseDir, "thumbnails")
}

func GetLogsDir() string {
	return filepath.Join(baseDir, "logs")
}

const (
	DefaultBingUrl    = "https://www.bing.com/HPImageArchive.aspx?format=js&idx=0&n=1&uhd=1&mkt=zh-CN"
	DefaultCustomUrl  = "https://bing.coding.icu/api/v1/image/today/meta"
	DefaultHolidayUrl = "https://github.com/NateScarlet/holiday-cn/raw/refs/heads/master/yyyy.json"
)

func DefaultConfig() Config {
	return Config{
		ApiType:         "bing",
		BingApiUrl:      DefaultBingUrl,
		CustomApiUrl:    DefaultCustomUrl,
		ApiMetaUrl:      DefaultBingUrl,
		AutoApply:       true,
		OverlayMetadata: false,
		ForceUHD:        true,
		ScheduleMode:    "daily",
		DailyTime:       "08:30",
		IntervalMinutes: 60,
		RetainDays:      0,
		RandomHistory:   false,
		LogRetainDays:   30,
		LogMaxSize:      10,
		AutoStart:       false,
		HideDockIcon:    false,
		EnableCalendar:  false,
		EnableHoliday:   true,
		HolidayApiUrl:   DefaultHolidayUrl,
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

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	// Migration for older versions
	migrated := false

	// Check if any expected fields are missing in the file
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err == nil {
		newFields := []string{"log_retain_days", "log_max_size", "auto_start", "hide_dock_icon", "random_history", "enable_calendar", "enable_holiday", "holiday_api_url"}
		for _, f := range newFields {
			if _, ok := raw[f]; !ok {
				migrated = true
				break
			}
		}
	}

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
	if cfg.HolidayApiUrl == "" {
		cfg.HolidayApiUrl = DefaultHolidayUrl
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
