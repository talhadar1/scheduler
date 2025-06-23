package task_scheduler

import (
	"context"
)

// The RegisterableTask represents a unit of work that can be scheduled and executed.
type RegisterableTask interface {
	Run(ctx context.Context) error
}

// task is the unexported implementation of the RegisterableTask interface,
// allowing additional metadata id and name
// to be wrapped around an inner RegisterableTask.
type task struct {
	id    int              // unique identifier for the task
	name  string           // human-readable name
	inner RegisterableTask // wrapped RegisterableTask that provides the actual Run logic
}

// Run executes the inner RegisterableTask's Run method.
func (t *task) Run(ctx context.Context) error {
	return t.inner.Run(ctx)
}

// TaskOption is a decorator function that wraps a task,
// adding or modifying behavior such as retries, delays, or logging.
// It takes an existing RegisterableTask and returns a new, decorated RegisterableTask.
type TaskOption func(*task)

// RegisterTask submits a Task for execution.
func (s *Scheduler) RegisterTask(t RegisterableTask, opts ...TaskOption) {}

// Scheduler manages task execution with controlled concurrency.
type Scheduler struct {
	taskChan chan task     // channel for tasks to be executed
	doneChan chan struct{} // channel to signal completion of all tasks
}

// NewScheduler creates a new Scheduler instance.
// It initializes the task channel and done channel,
// and sets the maximum number of concurrent workers.
func NewScheduler() *Scheduler {
	return &Scheduler{
		taskChan: make(chan task, 100),
		doneChan: make(chan struct{}),
	}
}

// RunScheduler starts the main scheduling loop.
func (s *Scheduler) RunScheduler() {}

// Stop signals that no more tasks will be registered and waits for all tasks to finish.
// Call this *after* all RegisterTask(...) calls.
// calling Stop() should be done after client has finished registering all tasks,
// so that the scheduler can process all tasks before closing the task channel.
func (s *Scheduler) Stop() {
}
