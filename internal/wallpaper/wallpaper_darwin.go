package wallpaper

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static int GetScreenCount() {
    return [[NSScreen screens] count];
}

static void GetScreenSize(int index, int *width, int *height) {
    NSArray<NSScreen *> *screens = [NSScreen screens];
    if (index >= 0 && index < [screens count]) {
        NSScreen *screen = screens[index];
        NSRect frame = [screen frame];
        *width = (int)frame.size.width;
        *height = (int)frame.size.height;
    } else {
        *width = 0;
        *height = 0;
    }
}

static int SetDesktopWallpaper(int index, const char* path) {
    @autoreleasepool {
        NSString *pathStr = [NSString stringWithUTF8String:path];
        NSURL *url = [NSURL fileURLWithPath:pathStr];
        NSArray<NSScreen *> *screens = [NSScreen screens];
        NSWorkspace *workspace = [NSWorkspace sharedWorkspace];
        NSError *error = nil;

        if (index < 0) {
            // Set for all screens
            BOOL success = YES;
            for (NSScreen *screen in screens) {
                NSDictionary *options = [workspace desktopImageOptionsForScreen:screen];
                if (![workspace setDesktopImageURL:url forScreen:screen options:options error:&error]) {
                    success = NO;
                }
            }
            return success ? 0 : 1;
        } else if (index < [screens count]) {
            NSScreen *screen = screens[index];
            NSDictionary *options = [workspace desktopImageOptionsForScreen:screen];
            if ([workspace setDesktopImageURL:url forScreen:screen options:options error:&error]) {
                return 0;
            }
            return 1;
        }
        return 2; // Index out of range
    }
}
*/
import "C"
import (
	"fmt"
	"path/filepath"
	"unsafe"
)

func getMonitors() ([]Monitor, error) {
	count := int(C.GetScreenCount())
	if count <= 0 {
		count = 1
	}

	var monitors []Monitor
	for i := range count {
		var w, h C.int
		C.GetScreenSize(C.int(i), &w, &h)

		width := int(w)
		height := int(h)
		if width <= 0 {
			width = 1920
		}
		if height <= 0 {
			height = 1080
		}

		monitors = append(monitors, Monitor{
			ID:     i,
			Name:   fmt.Sprintf("Display %d", i+1),
			Width:  width,
			Height: height,
		})
	}

	return monitors, nil
}

func set(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	cPath := C.CString(absPath)
	defer C.free(unsafe.Pointer(cPath))

	res := C.SetDesktopWallpaper(-1, cPath)
	if res != 0 {
		return fmt.Errorf("failed to set wallpaper for all screens (res: %d)", res)
	}
	return nil
}

func setOnMonitor(monitorID int, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	cPath := C.CString(absPath)
	defer C.free(unsafe.Pointer(cPath))

	res := C.SetDesktopWallpaper(C.int(monitorID), cPath)
	if res != 0 {
		return fmt.Errorf("failed to set wallpaper for monitor %d (res: %d)", monitorID, res)
	}
	return nil
}
