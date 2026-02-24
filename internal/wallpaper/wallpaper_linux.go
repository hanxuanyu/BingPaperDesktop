package wallpaper

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func getMonitors() ([]Monitor, error) {
	// For Linux, simplified implementation.
	// You might want to use 'xrandr' or 'gnome-randr' for more detail.
	return []Monitor{{ID: 0, Name: "Default Monitor", Width: 1920, Height: 1080}}, nil
}

func set(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Try GNOME
	if _, err := exec.LookPath("gsettings"); err == nil {
		uri := "file://" + absPath
		// Set both light and dark mode for GNOME 42+
		_ = exec.Command("gsettings", "set", "org.gnome.desktop.background", "picture-uri", uri).Run()
		_ = exec.Command("gsettings", "set", "org.gnome.desktop.background", "picture-uri-dark", uri).Run()
		return nil
	}

	return fmt.Errorf("no supported desktop environment found")
}

func setOnMonitor(monitorID int, path string) error {
	// GNOME gsettings doesn't easily support different wallpapers per monitor via CLI
	// It's usually one wallpaper for all.
	return set(path)
}
