package scheduler

import (
	"log/slog"
	"sync"
	"time"

	"BingPaperDesktop/internal/util"
)

type TaskFunc func() error

type Scheduler struct {
	task        TaskFunc
	mode        string // "off" | "daily" | "interval"
	dailyTime   string // "HH:MM"
	interval    time.Duration
	stopChan    chan struct{}
	mu          sync.Mutex
	isRunning   bool
	isExecuting bool
}

// New 创建一个新的调度器实例。task 是被周期性执行的函数。
// 无论调度模式如何，都注册了系统唤醒回调，但仅在 mode=="wakeup" 时才执行。
func New(task TaskFunc) *Scheduler {
	s := &Scheduler{
		task:     task,
		mode:     "off",
		stopChan: make(chan struct{}),
	}
	util.OnWake(func() {
		s.mu.Lock()
		mode := s.mode
		s.mu.Unlock()
		if mode == "wakeup" {
			slog.Info("Scheduler: system wake event received, executing task")
			s.execute()
		}
	})
	return s
}

func (s *Scheduler) Update(mode, dailyTime string, intervalMinutes int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mode = mode
	s.dailyTime = dailyTime
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}
	s.interval = time.Duration(intervalMinutes) * time.Minute

	if s.isRunning {
		s.stopLocked()
		s.startLocked()
	}
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startLocked()
}

func (s *Scheduler) startLocked() {
	if s.isRunning || s.mode == "off" {
		return
	}

	s.isRunning = true
	s.stopChan = make(chan struct{})

	go s.run()
	slog.Info("Scheduler started", "mode", s.mode, "dailyTime", s.dailyTime, "interval", s.interval)
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *Scheduler) stopLocked() {
	if !s.isRunning {
		return
	}
	close(s.stopChan)
	s.isRunning = false
	slog.Info("Scheduler stopped")
}

func (s *Scheduler) run() {
	for {
		var nextRun time.Duration

		s.mu.Lock()
		mode := s.mode
		s.mu.Unlock()

		if mode == "daily" {
			nextRun = s.calculateNextDaily()
		} else if mode == "interval" {
			nextRun = s.interval
		} else {
			return
		}

		select {
		case <-time.After(nextRun):
			s.execute()
		case <-s.stopChan:
			return
		}
	}
}

func (s *Scheduler) execute() {
	s.mu.Lock()
	if s.isExecuting {
		slog.Warn("Scheduler skip: task is already running")
		s.mu.Unlock()
		return
	}
	s.isExecuting = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isExecuting = false
		s.mu.Unlock()
	}()

	slog.Info("Scheduler trigger: start task")
	if err := s.task(); err != nil {
		slog.Error("Scheduler task failed", "error", err)
	}
}

func (s *Scheduler) calculateNextDaily() time.Duration {
	now := time.Now()
	target, err := time.ParseInLocation("15:04", s.dailyTime, time.Local)
	if err != nil {
		slog.Error("Invalid daily time format", "value", s.dailyTime)
		return 24 * time.Hour
	}

	next := time.Date(now.Year(), now.Month(), now.Day(), target.Hour(), target.Minute(), 0, 0, time.Local)
	if next.Before(now) {
		next = next.Add(24 * time.Hour)
	}

	return next.Sub(now)
}
