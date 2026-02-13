//go:build !darwin && !windows
// +build !darwin,!windows

package util

func HideDockIcon() {}
func ShowDockIcon() {}

func IsAutoStartEnabled() (bool, error) {
	return false, nil
}

func SetAutoStart(enable bool) error {
	return nil
}
