package task_scheduler

import (
	"fmt"
	"log"
	"os"
)

// WithLogging creates a task decorator that adds logging functionality.
func WithLogging() TaskOption {
	return func(task *task) {
		if task.logger == nil {
			task.logger = log.New(os.Stdout, fmt.Sprintf("task %d (%s): ", task.id, task.name), log.LstdFlags)
		}
	}
}

// WithName creates a task decorator that overrides the task's name.
func WithName(name string) TaskOption {
	return func(task *task) {
		task.name = name
		if task.logger != nil { // Update logger with name if it exists
			task.logger = log.New(os.Stdout, fmt.Sprintf("task %d (%s): ", task.id, task.name), log.LstdFlags)
		}
	}
}
