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

	"golang.org/x/sys/windows/registry"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	registerClassEx  = user32.NewProc("RegisterClassExW")
	createWindowEx   = user32.NewProc("CreateWindowExW")
	defWindowProc    = user32.NewProc("DefWindowProcW")
	getMessage       = user32.NewProc("GetMessageW")
	translateMessage = user32.NewProc("TranslateMessage")
	dispatchMessage  = user32.NewProc("DispatchMessageW")
	getModuleHandle  = kernel32.NewProc("GetModuleHandleW")
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type point struct {
	x, y int32
}

type msg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

func init() {
	go startWakeListener()
}

func startWakeListener() {
	const (
		WM_POWERBROADCAST      = 0x0218
		PBT_APMRESUMEAUTOMATIC = 0x0012
		PBT_APMRESUMESUSPEND   = 0x0007
		HWND_MESSAGE           = syscall.Handle(^uintptr(2)) // -3
	)

	className, _ := syscall.UTF16PtrFromString("WakeListenerClass")
	windowName, _ := syscall.UTF16PtrFromString("WakeListenerWindow")

	wndProc := syscall.NewCallback(func(hwnd syscall.Handle, msg uint32, wparam uintptr, lparam uintptr) uintptr {
		if msg == WM_POWERBROADCAST {
			if wparam == PBT_APMRESUMESUSPEND || wparam == PBT_APMRESUMEAUTOMATIC {
				slog.Info("Windows wake event detected")
				TriggerWake()
			}
		}
		ret, _, _ := defWindowProc.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
		return ret
	})

	instance, _, _ := getModuleHandle.Call(0)

	wc := wndClassExW{
		lpfnWndProc:   wndProc,
		hInstance:     syscall.Handle(instance),
		lpszClassName: className,
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))

	res, _, err := registerClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	if res == 0 {
		slog.Error("Failed to register window class", "error", err)
		return
	}

	hwnd, _, err := createWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0, 0, 0, 0,
		uintptr(HWND_MESSAGE),
		0,
		instance,
		0,
	)
	if hwnd == 0 {
		slog.Error("Failed to create message-only window", "error", err)
		return
	}

	var m msg
	for {
		res, _, err := getMessage.Call(uintptr(unsafe.Pointer(&m)), uintptr(hwnd), 0, 0)
		if int32(res) <= 0 {
			if int32(res) == -1 {
				slog.Error("GetMessage error", "error", err)
			}
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
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
