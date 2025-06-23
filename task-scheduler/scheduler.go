package task_scheduler

import (
	"context"
)

const tasksBufferSize = 100 // Size of the task channel buffer

// The RegisterableTask represents a unit of work that can be scheduled and executed.
type RegisterableTask interface {
	Run(ctx context.Context) error
}

// The task is the unexported implementation of the RegisterableTask interface, allowing additional metadata id and name to be wrapped around an inner RegisterableTask.
type task struct {
	id    int              // Unique identifier for the task
	name  string           // Human-readable name
	inner RegisterableTask // Wrapped RegisterableTask that provides the actual Run logic
}

// Run executes the inner RegisterableTask's Run method.
func (t *task) Run(ctx context.Context) error {
	return t.inner.Run(ctx)
}

// TaskOption is a decorator function that wraps a task, adding or modifying behavior such as retries, delays, or logging.
// It takes an existing RegisterableTask and returns a new, decorated RegisterableTask.
type TaskOption func(*task)

// RegisterTask submits a Task for execution.
func (s *Scheduler) RegisterTask(t RegisterableTask, opts ...TaskOption) {}

// Scheduler manages task execution with controlled concurrency.
type Scheduler struct {
	taskChan chan task     // Channel for tasks to be executed
	doneChan chan struct{} // Channel to signal completion of all tasks
}

// NewScheduler creates a new Scheduler instance.
// It initializes the task channel and done channel, and sets the maximum number of concurrent workers.
func NewScheduler() *Scheduler {
	return &Scheduler{
		taskChan: make(chan task, tasksBufferSize),
		doneChan: make(chan struct{}),
	}
}

// RunScheduler starts the main scheduling loop.
func (s *Scheduler) RunScheduler() {}

// Stop signals that no more tasks will be registered and waits for all tasks to finish.
// Call this *after* all RegisterTask(...) calls.
// Calling Stop() should be done after client has finished registering all tasks, so that the scheduler can process all tasks before closing the task channel.
func (s *Scheduler) Stop() {
}
