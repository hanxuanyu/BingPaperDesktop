//go:build darwin
// +build darwin

package util

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static inline void SetActivationPolicy(int policy) {
    NSApplicationActivationPolicy p = (NSApplicationActivationPolicy)policy;
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp setActivationPolicy:p];
    });
}

extern void triggerWake();

static void StartWakeListener() {
    static id observer = nil;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        observer = [[[NSWorkspace sharedWorkspace] notificationCenter] addObserverForName:NSWorkspaceDidWakeNotification
                                                                                  object:NULL
                                                                                   queue:NULL
                                                                              usingBlock:^(NSNotification *note) {
            triggerWake();
        }];
    });
}
*/
import "C"
import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//export triggerWake
func triggerWake() {
	TriggerWake()
}

func init() {
	C.StartWakeListener()
}

func OpenFolder(path string) error {
	return exec.Command("open", path).Start()
}

func OpenURL(url string) error {
	return exec.Command("open", url).Start()
}

func HideDockIcon() {
	C.SetActivationPolicy(C.NSApplicationActivationPolicyAccessory)
}

func ShowDockIcon() {
	C.SetActivationPolicy(C.NSApplicationActivationPolicyRegular)
}

func IsAutoStartEnabled() (bool, error) {
	exePath, err := os.Executable()
	if err != nil {
		return false, err
	}
	// 获取可执行文件名称（不带路径）
	appName := filepath.Base(exePath)
	// 如果在 .app 包内运行，exePath 会是 .../Contents/MacOS/BinaryName
	// 我们希望检查的是这个应用是否在 login items 中

	// 使用 osascript 检查
	script := fmt.Sprintf(`tell application "System Events" to get count of (every login item whose name is "%s")`, appName)
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "0", nil
}

func SetAutoStart(enable bool) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	appName := filepath.Base(exePath)

	var script string
	if enable {
		// 如果已经在列表中则不添加，避免重复（虽然 osascript 也会处理）
		// 为了更好的体验，如果是 .app 包，我们应该添加 .app 路径而不是二进制路径
		appPath := exePath
		if strings.Contains(exePath, ".app/Contents/MacOS") {
			// 向上找 3 层到 .app 目录
			appPath = filepath.Dir(filepath.Dir(filepath.Dir(exePath)))
		}

		script = fmt.Sprintf(`tell application "System Events" to make login item at end with properties {path:"%s", name:"%s", hidden:false}`, appPath, appName)
		// 检查是否已存在
		exists, _ := IsAutoStartEnabled()
		if exists {
			return nil
		}
	} else {
		script = fmt.Sprintf(`tell application "System Events" to delete (every login item whose name is "%s")`, appName)
	}

	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}
