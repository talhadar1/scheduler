package task_scheduler

import (
	"fmt"
	"log"
	"os"
	"time"
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

// WithDelay creates a task decorator that delays execution by the specified duration.
func WithDelay(delay time.Duration) TaskOption {
	return func(task *task) {
		task.delay = delay
	}
}

// WithRetry creates a task decorator that retries the task on failure.
// It accepts a number of retries, which must be between 1 and 100.
// If the number of retries is outside this range, it will not change the default value (1).
func WithRetry(retries int) TaskOption {
	return func(task *task) {
		if 1 <= retries && retries <= 100 {
			task.NumOfRetries = retries
		}
	}
}
