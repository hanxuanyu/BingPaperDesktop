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
			// Typical line: "Resolution: 3840 x 2160" or "Resolution: 3840x2160"
			line = strings.TrimSpace(line)
			parts := strings.Split(line, ":")
			if len(parts) < 2 {
				continue
			}
			resPart := strings.TrimSpace(parts[1])

			// Use fields to handle both "3840 x 2160" and "3840x2160"
			fields := strings.Fields(resPart)
			var wStr, hStr string
			if len(fields) >= 3 && fields[1] == "x" {
				wStr = fields[0]
				hStr = fields[2]
			} else if len(fields) >= 1 {
				// Handle "3840x2160" or "3840x2160(xxx)"
				// First field might contain 'x'
				wh := strings.Split(fields[0], "x")
				if len(wh) == 2 {
					wStr = wh[0]
					hStr = wh[1]
				}
			}

			if wStr != "" && hStr != "" {
				w, _ := strconv.Atoi(wStr)
				h, _ := strconv.Atoi(hStr)
				if w > 0 && h > 0 {
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
