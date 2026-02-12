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
