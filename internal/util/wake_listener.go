package util

import (
	"log/slog"
	"sync"
)

var (
	wakeCallbacks []func()
	wakeMu        sync.Mutex
)

// OnWake 注册一个系统唤醒时的回调函数
func OnWake(callback func()) {
	wakeMu.Lock()
	defer wakeMu.Unlock()
	wakeCallbacks = append(wakeCallbacks, callback)
}

// TriggerWake 由平台相关的实现调用，通知系统已唤醒
func TriggerWake() {
	wakeMu.Lock()
	callbacks := make([]func(), len(wakeCallbacks))
	copy(callbacks, wakeCallbacks)
	wakeMu.Unlock()

	slog.Info("System wake detected, triggering callbacks", "count", len(callbacks))
	for _, cb := range callbacks {
		go cb()
	}
}
