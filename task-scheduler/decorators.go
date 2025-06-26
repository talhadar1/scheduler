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
