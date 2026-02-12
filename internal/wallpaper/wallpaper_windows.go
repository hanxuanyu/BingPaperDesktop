package wallpaper

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	systemParametersInfo = user32.NewProc("SystemParametersInfoW")
)

const (
	spiSetDeskWallpaper = 0x0014
	uiParam             = 0x0000
	spifUpdateIniFile   = 0x01
	spifSendChange      = 0x02
)

func setWallpaper(path string) error {
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

func Set(path string) error {
	return setWallpaper(path)
}
