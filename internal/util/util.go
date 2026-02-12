package util

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

func Retry(attempts int, sleep time.Duration, f func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = f(); err == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(sleep)
			sleep *= 2 // Exponential backoff
		}
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

func OpenFolder(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default: // Linux
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func NormalizeDate(d string) string {
	// If it's already YYYY-MM-DD, return as is
	if len(d) == 10 && d[4] == '-' && d[7] == '-' {
		return d
	}
	// If it's YYYYMMDD, convert to YYYY-MM-DD
	if len(d) == 8 {
		// Check if it's all digits
		isAllDigits := true
		for _, r := range d {
			if r < '0' || r > '9' {
				isAllDigits = false
				break
			}
		}
		if isAllDigits {
			return fmt.Sprintf("%s-%s-%s", d[0:4], d[4:6], d[6:8])
		}
	}
	return d
}
