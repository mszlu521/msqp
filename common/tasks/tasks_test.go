package tasks

import (
	"fmt"
	"testing"
	"time"
)

var task *Task

func TestNewTask(t *testing.T) {
	task = NewTask("test", time.Second, func() {
		fmt.Println("执行任务")
		stopTask()
	})
	fmt.Println("开始执行任务")
	time.Sleep(5 * time.Second)
}

func stopTask() {
	if task != nil {
		task.Stop()
		task = nil
	}
	fmt.Println("任务结束")
}
