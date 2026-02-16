//go:build !darwin && !windows
// +build !darwin,!windows

package util

import "os/exec"

func OpenFolder(path string) error {
	return exec.Command("xdg-open", path).Start()
}

func OpenURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}

func HideDockIcon() {}
func ShowDockIcon() {}

func IsAutoStartEnabled() (bool, error) {
	return false, nil
}

func SetAutoStart(enable bool) error {
	return nil
}
