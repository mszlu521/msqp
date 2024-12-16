package tasks

import (
	"fmt"
	"testing"
	"time"
)

var task *Task

func TestNewTask(t *testing.T) {
}

func TestNewTaskScheduler(t *testing.T) {
	// 创建一个 TaskScheduler
	scheduler := NewTaskScheduler()

	// 定义要执行的任务
	task1 := func() {
		fmt.Println("Task is executing...")
	}
	// 启动任务，设置等待 5 秒后执行
	scheduler.StartTask(5*time.Second, task1)
	// 模拟在 3 秒后停止任务
	time.Sleep(3 * time.Second)
	scheduler.StopTask()
	// 等待任务完成
	go scheduler.Wait()
	// 退出
	fmt.Println("Program finished.")

	time.Sleep(10 * time.Second)
}
