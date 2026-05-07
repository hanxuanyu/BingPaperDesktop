package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"BingPaperDesktop/internal/store"
	"BingPaperDesktop/internal/util"
)

var logUpdateFunc func(store.Config)

// RegisterLogUpdate 供 main 注册日志配置更新回调。
func RegisterLogUpdate(fn func(store.Config)) {
	logUpdateFunc = fn
}

func (a *App) GetConfig() (store.Config, error) {
	return store.LoadConfig()
}

func (a *App) SaveConfig(cfg store.Config) error {
	if cfg.IntervalMinutes < 1 {
		cfg.IntervalMinutes = 1
	}
	cfg = store.NormalizeConfig(cfg)
	oldCfg, _ := store.LoadConfig()

	// 同步开机启动设置
	if oldCfg.AutoStart != cfg.AutoStart {
		if err := util.SetAutoStart(cfg.AutoStart); err != nil {
			slog.Error("Failed to set auto start", "enable", cfg.AutoStart, "error", err)
		}
	}

	// 同步 macOS Dock 图标显示设置
	if oldCfg.HideDockIcon != cfg.HideDockIcon {
		if cfg.HideDockIcon {
			util.HideDockIcon()
		} else {
			util.ShowDockIcon()
		}
	}

	err := store.SaveConfig(cfg)
	if err == nil {
		if cfg.EnableHoliday {
			go func() {
				year := time.Now().Year()
				force := (!oldCfg.EnableHoliday && cfg.EnableHoliday) || (oldCfg.HolidayApiUrl != cfg.HolidayApiUrl)
				if err := store.EnsureHolidayWithConfig(year, cfg, force); err != nil {
					slog.Error("Failed to ensure holiday data", "year", year, "error", err, "force", force)
				}
			}()
		}
		a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
		if logUpdateFunc != nil {
			logUpdateFunc(cfg)
		}
	}
	return err
}

func (a *App) IsAutoStartEnabled() (bool, error) {
	return util.IsAutoStartEnabled()
}

func (a *App) ResetSettings() error {
	slog.Info("Reset: only settings")
	a.sched.Stop()

	cfg := store.DefaultConfig()
	if err := store.SaveConfig(cfg); err != nil {
		slog.Error("Reset: saving default config failed", "error", err)
		return fmt.Errorf("failed to save default config: %w", err)
	}

	a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	a.sched.Start()

	slog.Info("Reset settings completed")
	return nil
}

func (a *App) ResetApplication() error {
	slog.Info("!!! AUTOMATIC RESET TRIGGERED !!!")
	a.sched.Stop()

	base := store.GetBaseDir()
	dataDir := filepath.Join(base, "data")
	configFile := filepath.Join(base, "config.json")

	slog.Info("Reset: cleaning up data and config", "dataDir", dataDir, "configFile", configFile)

	if err := os.RemoveAll(dataDir); err != nil {
		slog.Warn("Reset: failed to remove data directory", "error", err)
	}

	if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
		slog.Warn("Reset: failed to remove config file", "error", err)
	}

	if err := store.Init(); err != nil {
		slog.Error("Reset: store.Init failed", "error", err)
		return fmt.Errorf("failed to re-initialize storage: %w", err)
	}

	cfg := store.DefaultConfig()
	if err := store.SaveConfig(cfg); err != nil {
		slog.Error("Reset: saving default config failed", "error", err)
		return fmt.Errorf("failed to save default config: %w", err)
	}

	a.sched.Update(cfg.ScheduleMode, cfg.DailyTime, cfg.IntervalMinutes)
	a.sched.Start()

	a.mu.Lock()
	a.lastFetch = nil
	a.mu.Unlock()

	slog.Info("!!! AUTOMATIC RESET COMPLETED !!!")
	return nil
}
