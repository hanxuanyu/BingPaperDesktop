package wallpaper

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func Set(path string) error {
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
