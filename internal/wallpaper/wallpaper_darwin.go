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

	script := fmt.Sprintf(`tell application "System Events" to set picture of every desktop to "%s"`, absPath)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript failed: %v", err)
	}
	return nil
}
