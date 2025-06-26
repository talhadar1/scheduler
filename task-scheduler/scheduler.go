package task_scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

const defaultNumOfWorkers = 50 // Default number of workers to run concurrently

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

// newTask creates a new task with the given parameters.
// It initializes the task with an id, name, and an inner RegisterableTask.
func newTask(inner RegisterableTask, id int, opts ...TaskOption) *task {
	t := &task{
		id:    id,
		name:  fmt.Sprintf("task-%d", id), // default name format
		inner: inner,
	}
	// Apply any taskOption to the new task
	for _, option := range opts {
		option(t)
	}
	return t
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
func (s *Scheduler) RegisterTask(t RegisterableTask, id int, opts ...TaskOption) {
	task := newTask(t, id, opts...)
	s.taskChan <- *task
	s.registeredTasks++
}

// Scheduler manages task execution with controlled concurrency.
type Scheduler struct {
	taskChan        chan task      // channel for tasks to be executed
	doneChan        chan struct{}  // channel to signal completion of all tasks
	registeredTasks int            // total number of tasks registered
	wg              sync.WaitGroup // wait group to synchronize worker goroutines
	ongoingTasks    atomic.Int32   // count of currently running tasks
	failedTasks     atomic.Int32   // count of tasks that failed
	finishedTasks   atomic.Int32   // count of tasks that finished successfully
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
// It drains Scheduler taskChan until closed, then waits for all workers.
// Workers run tasks concurrently, updating the counts of ongoing, finished, and failed tasks.
// It uses a mutex to protect access to the Scheduler counters.
func (s *Scheduler) RunScheduler() {
	//start workers
	log.Println("💼 Starting Scheduler with", defaultNumOfWorkers, "workers")
	for i := 0; i < defaultNumOfWorkers; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for task := range s.taskChan {
				s.ongoingTasks.Add(1)                 // Increment ongoing tasks count
				err := task.Run(context.Background()) // Execute the task's Run method
				s.ongoingTasks.Add(-1)                // Decrement ongoing tasks count
				if err != nil {
					s.failedTasks.Add(1)
				} else {
					s.finishedTasks.Add(1)
				}
			}
		}()
	}
	s.wg.Wait()

	// Send signal for completion
	close(s.doneChan)
}

// Stop signals that no more tasks will be registered and waits for all tasks to finish.
// Call this *after* all RegisterTask(...) calls.
// calling Stop() should be done after client has finished registering all tasks,
// so that the scheduler can process all tasks before closing the task channel.
func (s *Scheduler) Stop() {
	log.Println("🔒 Closing task channel; no more tasks will be registered")
	close(s.taskChan)
	<-s.doneChan // Wait for the scheduler to finish processing all tasks
}
