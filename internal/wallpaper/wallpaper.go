package wallpaper

import (
	"fmt"
	"os/exec"
	"runtime"
)

type Monitor struct {
	ID     int
	Name   string
	Width  int
	Height int
}

func GetMonitors() ([]Monitor, error) {
	return getMonitors()
}

func Set(path string) error {
	return set(path)
}

func SetOnMonitor(monitorID int, path string) error {
	return setOnMonitor(monitorID, path)
}

func Supported() (bool, string) {
	switch runtime.GOOS {
	case "windows", "darwin":
		return true, ""
	case "linux":
		if _, err := exec.LookPath("gsettings"); err == nil {
			return true, "Supports GNOME (gsettings)"
		}
		return false, "Only GNOME (gsettings) is supported on Linux"
	default:
		return false, fmt.Sprintf("OS %s not supported", runtime.GOOS)
	}
}
