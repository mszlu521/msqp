package tasks

import (
	"testing"
	"time"
)

var task *Task

func TestNewTask(t *testing.T) {
}

func TestNewTaskScheduler(t *testing.T) {
	run := make(chan struct{}, 1)
	task := NewTask("test", 10*time.Millisecond, func() {
		select {
		case run <- struct{}{}:
		default:
		}
	})
	defer task.Stop()

	select {
	case <-run:
	case <-time.After(time.Second):
		t.Fatal("scheduled task did not run")
	}
}
