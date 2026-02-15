package app

import (
	"log/slog"
	"os"
	"runtime"

	"BingPaperDesktop/internal/store"
	"BingPaperDesktop/internal/util"

	"github.com/energye/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) SetupTray(appIcon, appIconIco []byte) (func(), func()) {
	onReady := func() {
		systray.CreateMenu()
		slog.Info("Systray onReady start")

		showWindow := func() {
			go func() {
				if a.ctx != nil {
					wruntime.EventsEmit(a.ctx, "prepare-show-window")
					cfg, _ := store.LoadConfig()
					if !cfg.HideDockIcon {
						util.ShowDockIcon()
					}
					wruntime.WindowShow(a.ctx)
				}
			}()
		}

		// 显式设置点击回调
		systray.SetOnClick(func(menu systray.IMenu) {
			slog.Info("Tray: Left click triggered")
			if runtime.GOOS == "darwin" {
				menu.ShowMenu()
			}
		})
		systray.SetOnRClick(func(menu systray.IMenu) {
			slog.Info("Tray: Right click triggered")
			menu.ShowMenu()
		})
		systray.SetOnDClick(func(menu systray.IMenu) {
			slog.Info("Tray: Double click triggered")
			showWindow()
		})

		slog.Info("Adding tray menu items")
		mShow := systray.AddMenuItem("显示界面", "显示界面")
		mShow.Click(func() {
			slog.Info("Tray menu: Show clicked")
			showWindow()
		})
		mFetch := systray.AddMenuItem("立即刷新壁纸", "立即获取并设置今日壁纸")
		mFetch.Click(func() {
			go func() {
				slog.Info("Tray menu: Fetch clicked")
				a.FetchToday(0, 0, 1.0)
			}()
		})
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("退出程序", "彻底退出")
		mQuit.Click(func() {
			go func() {
				slog.Info("Tray menu: Quit clicked")
				if a.ctx != nil {
					wruntime.Quit(a.ctx)
				} else {
					os.Exit(0)
				}
			}()
		})

		if runtime.GOOS == "windows" {
			systray.SetIcon(appIconIco)
		} else if runtime.GOOS == "darwin" {
			// macOS 菜单栏图标推荐尺寸为 22x22
			iconSmall := util.ResizeIcon(appIcon, 22)
			slog.Info("Setting systray icon (macOS)", "original_len", len(appIcon), "new_len", len(iconSmall))
			systray.SetTemplateIcon(iconSmall, iconSmall)
		} else {
			slog.Info("Setting systray icon", "len", len(appIcon))
			systray.SetIcon(appIcon)
		}
		systray.SetTooltip("BingPaperDesktop")
		slog.Info("Systray onReady complete")
	}

	var trayStart, trayEnd func()
	if runtime.GOOS == "darwin" {
		trayStart, trayEnd = systray.RunWithExternalLoop(onReady, nil)
		slog.Info("Darwin: using RunWithExternalLoop")
	} else {
		systray.Register(onReady, nil)
	}
	return trayStart, trayEnd
}
