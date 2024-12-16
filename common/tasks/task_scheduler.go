package tasks

import (
	"fmt"
	"sync"
	"time"
)

type TaskScheduler struct {
	timer *time.Timer
	stop  chan struct{}
	wg    sync.WaitGroup
	once  sync.Once
}

func NewTaskScheduler() *TaskScheduler {
	return &TaskScheduler{
		stop: make(chan struct{}),
	}
}

// StartTask 启动定时任务
func (ts *TaskScheduler) StartTask(duration time.Duration, task func()) {
	// 设置定时器
	ts.timer = time.NewTimer(duration)

	// 启动一个 Goroutine 执行任务
	ts.wg.Add(1)
	go func() {
		defer ts.wg.Done()
		select {
		case <-ts.timer.C:
			// 任务超时执行
			task()
		case <-ts.stop:
			// 如果接收到停止信号，取消任务
			fmt.Println("Task was stopped before execution.")
		}
	}()
}

// StopTask 停止任务的执行
func (ts *TaskScheduler) StopTask() {
	if ts.timer != nil {
		// 停止定时器
		ts.timer.Stop()
		// 发送停止信号
		ts.once.Do(func() {
			close(ts.stop)
		})
	}
}

// Wait 等待任务完成
func (ts *TaskScheduler) Wait() {
	ts.wg.Wait()
}
