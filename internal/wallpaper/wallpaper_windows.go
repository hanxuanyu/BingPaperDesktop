package wallpaper

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	systemParametersInfo = user32.NewProc("SystemParametersInfoW")
	enumDisplayMonitors  = user32.NewProc("EnumDisplayMonitors")
	getMonitorInfo       = user32.NewProc("GetMonitorInfoW")
)

const (
	spiSetDeskWallpaper = 0x0014
	uiParam             = 0x0000
	spifUpdateIniFile   = 0x01
	spifSendChange      = 0x02
)

type rect struct {
	left, top, right, bottom int32
}

type monitorInfoExW struct {
	cbSize    uint32
	rcMonitor rect
	rcWork    rect
	dwFlags   uint32
	szDevice  [32]uint16
}

func getMonitors() ([]Monitor, error) {
	var monitors []Monitor
	callback := syscall.NewCallback(func(hMonitor syscall.Handle, hdcMonitor syscall.Handle, lprcMonitor *rect, dwData uintptr) uintptr {
		var mi monitorInfoExW
		mi.cbSize = uint32(unsafe.Sizeof(mi))
		ret, _, _ := getMonitorInfo.Call(uintptr(hMonitor), uintptr(unsafe.Pointer(&mi)))
		if ret != 0 {
			monitors = append(monitors, Monitor{
				ID:     len(monitors),
				Name:   syscall.UTF16ToString(mi.szDevice[:]),
				Width:  int(mi.rcMonitor.right - mi.rcMonitor.left),
				Height: int(mi.rcMonitor.bottom - mi.rcMonitor.top),
			})
		}
		return 1
	})

	enumDisplayMonitors.Call(0, 0, callback, 0)
	return monitors, nil
}

func set(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	ptr, err := syscall.UTF16PtrFromString(absPath)
	if err != nil {
		return err
	}

	ret, _, err := systemParametersInfo.Call(
		uintptr(spiSetDeskWallpaper),
		uintptr(uiParam),
		uintptr(unsafe.Pointer(ptr)),
		uintptr(spifUpdateIniFile|spifSendChange),
	)

	if ret == 0 {
		return fmt.Errorf("SystemParametersInfoW failed: %v", err)
	}

	return nil
}

func setOnMonitor(monitorID int, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	err = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		return err
	}
	defer ole.CoUninitialize()

	clsid, err := ole.CLSIDFromString("{C2CF27E3-0791-419B-A244-243CA06BB57D}")
	if err != nil {
		return err
	}
	iid, err := ole.IIDFromString("{BAD9BB81-5140-4614-B1A7-B233D1E646AF}")
	if err != nil {
		return err
	}

	unknown, err := ole.CreateInstance(clsid, iid)
	if err != nil {
		return err
	}
	defer unknown.Release()

	wallpaper := (*IDesktopWallpaper)(unsafe.Pointer(unknown))

	// Get monitor ID (GUID or Index)
	// IDesktopWallpaper uses MonitorID which is a string (GUID or something)
	// We can get it by index
	var monitorCount uint32
	hr, _, _ := syscall.SyscallN(wallpaper.VTable.GetMonitorDevicePathCount, uintptr(unsafe.Pointer(wallpaper)), uintptr(unsafe.Pointer(&monitorCount)))
	if hr != 0 {
		return fmt.Errorf("GetMonitorDevicePathCount failed: %08x", hr)
	}

	if uint32(monitorID) >= monitorCount {
		return fmt.Errorf("monitor index out of range: %d", monitorID)
	}

	var monitorIDStr *uint16
	hr, _, _ = syscall.SyscallN(wallpaper.VTable.GetMonitorDevicePathAt, uintptr(unsafe.Pointer(wallpaper)), uintptr(monitorID), uintptr(unsafe.Pointer(&monitorIDStr)))
	if hr != 0 {
		return fmt.Errorf("GetMonitorDevicePathAt failed: %08x", hr)
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(monitorIDStr)))

	pathPtr, _ := syscall.UTF16PtrFromString(absPath)
	hr, _, _ = syscall.SyscallN(wallpaper.VTable.SetWallpaper, uintptr(unsafe.Pointer(wallpaper)), uintptr(unsafe.Pointer(monitorIDStr)), uintptr(unsafe.Pointer(pathPtr)))
	if hr != 0 {
		return fmt.Errorf("SetWallpaper failed: %08x", hr)
	}

	return nil
}

type IDesktopWallpaper struct {
	VTable *IDesktopWallpaperVtbl
}

type IDesktopWallpaperVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr

	SetWallpaper              uintptr
	GetWallpaper              uintptr
	GetMonitorDevicePathAt    uintptr
	GetMonitorDevicePathCount uintptr
	GetMonitorRECT            uintptr
	SetBackgroundColor        uintptr
	GetBackgroundColor        uintptr
	SetPosition               uintptr
	GetPosition               uintptr
	SetSlideshow              uintptr
	GetSlideshow              uintptr
	SetSlideshowOptions       uintptr
	GetSlideshowOptions       uintptr
	AdvanceSlideshow          uintptr
	GetStatus                 uintptr
	Enable                    uintptr
}
