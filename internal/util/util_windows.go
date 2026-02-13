//go:build windows
// +build windows

package util

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

func HideDockIcon() {}
func ShowDockIcon() {}

func IsAutoStartEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()

	exePath, err := os.Executable()
	if err != nil {
		return false, err
	}
	appName := filepath.Base(exePath)

	val, _, err := k.GetStringValue(appName)
	if err != nil {
		return false, nil // Assume not found
	}

	return val == exePath, nil
}

func SetAutoStart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	appName := filepath.Base(exePath)

	if enable {
		return k.SetStringValue(appName, exePath)
	} else {
		return k.DeleteValue(appName)
	}
}
