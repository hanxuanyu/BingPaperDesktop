//go:build windows
// +build windows

package util

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func init() {
	go startWakeListener()
}

func startWakeListener() {
	const (
		WM_POWERBROADCAST      = 0x0218
		PBT_APMRESUMEAUTOMATIC = 0x0012
		PBT_APMRESUMESUSPEND   = 0x0007
	)

	className, _ := syscall.UTF16PtrFromString("WakeListenerClass")
	windowName, _ := syscall.UTF16PtrFromString("WakeListenerWindow")

	wndProc := syscall.NewCallback(func(hwnd windows.HWND, msg uint32, wparam uintptr, lparam uintptr) uintptr {
		if msg == WM_POWERBROADCAST {
			if wparam == PBT_APMRESUMESUSPEND || wparam == PBT_APMRESUMEAUTOMATIC {
				slog.Info("Windows wake event detected")
				TriggerWake()
			}
		}
		return windows.DefWindowProc(hwnd, msg, wparam, lparam)
	})

	instance, _ := windows.GetModuleHandle(nil)

	wc := windows.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(windows.WNDCLASSEX{})),
		LpfnWndProc:   wndProc,
		HInstance:     instance,
		LpszClassName: className,
	}

	if _, err := windows.RegisterClassEx(&wc); err != nil {
		slog.Error("Failed to register window class", "error", err)
		return
	}

	hwnd, err := windows.CreateWindowEx(
		0,
		className,
		windowName,
		0,
		0, 0, 0, 0,
		windows.HWND_MESSAGE,
		0,
		instance,
		nil,
	)
	if err != nil {
		slog.Error("Failed to create message-only window", "error", err)
		return
	}

	var msg windows.Msg
	for {
		res, err := windows.GetMessage(&msg, hwnd, 0, 0)
		if res == 0 || res == -1 {
			if err != nil {
				slog.Error("GetMessage error", "error", err)
			}
			break
		}
		windows.TranslateMessage(&msg)
		windows.DispatchMessage(&msg)
	}
}

func OpenFolder(path string) error {
	return exec.Command("explorer", path).Start()
}

func OpenURL(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func HideDockIcon() {}
func ShowDockIcon() {}

func IsAutoStartEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()

	exePath, err := os.Executable()
	if err != nil {
		return false, err
	}
	appName := filepath.Base(exePath)

	val, _, err := k.GetStringValue(appName)
	if err != nil {
		return false, nil // Assume not found
	}

	return val == exePath, nil
}

func SetAutoStart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	appName := filepath.Base(exePath)

	if enable {
		return k.SetStringValue(appName, exePath)
	} else {
		return k.DeleteValue(appName)
	}
}
