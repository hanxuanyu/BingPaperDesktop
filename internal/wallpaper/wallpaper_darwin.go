package wallpaper

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func getMonitors() ([]Monitor, error) {
	// Use system_profiler to get display info
	cmd := exec.Command("system_profiler", "SPDisplaysDataType")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var monitors []Monitor
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Resolution:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				res := parts[1]
				wh := strings.Split(res, "x")
				if len(wh) == 2 {
					w, _ := strconv.Atoi(wh[0])
					h, _ := strconv.Atoi(wh[1])
					monitors = append(monitors, Monitor{
						ID:     len(monitors),
						Name:   fmt.Sprintf("Display %d", len(monitors)+1),
						Width:  w,
						Height: h,
					})
				}
			}
		}
	}

	// Fallback to at least one monitor if something went wrong but we are on Mac
	if len(monitors) == 0 {
		monitors = append(monitors, Monitor{ID: 0, Name: "Main Display", Width: 1920, Height: 1080})
	}

	return monitors, nil
}

func set(path string) error {
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

func setOnMonitor(monitorID int, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// AppleScript index is 1-based
	script := fmt.Sprintf(`tell application "System Events" to set picture of desktop %d to "%s"`, monitorID+1, absPath)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript failed for monitor %d: %v", monitorID, err)
	}
	return nil
}
