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
	tick := 30
	_ = NewTask("test", time.Second, func() {
		tick--
		if tick < 0 {
			fmt.Println("tick")
		}
	})
	select {
	case <-time.After(100 * time.Second):
	}
}
