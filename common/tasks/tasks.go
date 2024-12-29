package tasks

import (
	"sync"
	"time"
)

// Task 定时任务结构体
type Task struct {
	Name     string
	ticker   *time.Ticker
	stopChan chan struct{}
	once     sync.Once
}

// NewTask 创建一个新的定时任务
func NewTask(name string, interval time.Duration, job func()) *Task {
	task := &Task{
		ticker:   time.NewTicker(interval),
		stopChan: make(chan struct{}),
		Name:     name,
	}

	go func() {
		for {
			select {
			case <-task.ticker.C:
				job() // 执行定时任务
			case <-task.stopChan:
				return // 停止任务
			}
		}
	}()
	return task
}

// Stop 停止定时任务（非阻塞）
func (t *Task) Stop() {
	t.once.Do(func() {
		t.ticker.Stop()
		close(t.stopChan)
	})
}
