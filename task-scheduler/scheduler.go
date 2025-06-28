package task_scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultNumOfWorkers = 50  // Default number of workers to run concurrently
	tasksBufferSize     = 100 // Size of the task channel buffer
)

// The RegisterableTask represents a unit of work that can be scheduled and executed.
type RegisterableTask interface {
	Run(ctx context.Context) error
}

// The task is the unexported implementation of the RegisterableTask interface, allowing additional metadata id and name to be wrapped around an inner RegisterableTask.
type task struct {
	id           int              // Unique identifier for the task
	name         string           // Human-readable name
	inner        RegisterableTask // Wrapped RegisterableTask that provides the actual Run logic
	logger       *log.Logger      // Optional logger for Task-specific output
	delay        time.Duration    // Optional delay duration before execution
	NumOfRetries int              // Number of times to retry on failure
}

// The newTask creates a new task with the given parameters.
// It initializes the task with an id, name, and an inner RegisterableTask.
func newTask(inner RegisterableTask, id int, opts ...TaskOption) *task {
	t := &task{
		id:           id,
		name:         fmt.Sprintf("task-%d", id), // Default name format
		inner:        inner,
		logger:       nil, // Default no logger
		delay:        0,   // Default no delay
		NumOfRetries: 1,   // Default only one try
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

// TaskOption is a decorator function that wraps a task, adding or modifying behavior such as retries, delays, or logging.
// It takes an existing RegisterableTask and returns a new, decorated RegisterableTask.
type TaskOption func(*task)

// RegisterTask submits a Task for execution.
// This method is blocking as it sends the task to the task channel.
// In-order to maintain the order of task execution, the client should ensure that tasks are registered sequentially.
func (s *Scheduler) RegisterTask(t RegisterableTask, id int, opts ...TaskOption) {
	task := newTask(t, id, opts...)
	s.registeredTasks.Add(1)
	if task.logger != nil {
		task.logger.Printf("Registered task - ID: %d, Name: %s", task.id, task.name)
	}
	if task.delay > 0 {
		s.currentDelayedTasks.Add(1)
		if task.logger != nil {
			task.logger.Printf("⏳ Delaying task - ID: %d for %s", task.id, task.delay)
		}
		s.delayer.schedule(*task, time.Now().Add(task.delay))
	} else {
		s.taskChan <- *task // Send the task to the channel for immediate execution
	}
}

// Scheduler manages task execution with controlled concurrency.
type Scheduler struct {
	taskChan            chan task        // Channel for tasks to be executed
	doneChan            chan struct{}    // Channel to signal completion of all tasks
	registeredTasks     atomic.Int32     // Total number of tasks registered
	wg                  sync.WaitGroup   // Wait group to synchronize worker goroutines
	ongoingTasks        atomic.Int32     // Count of currently running tasks
	failedTasks         atomic.Int32     // Count of tasks that failed
	finishedTasks       atomic.Int32     // Count of tasks that finished successfully
	delayer             *delayDispatcher // The delayDispatcher manages delayed tasks
	currentDelayedTasks atomic.Int32     // Count of currently delayed tasks used by the delayDispatcher
}

// NewScheduler creates a new Scheduler instance.
// It initializes the task channel and done channel, and sets the maximum number of concurrent workers.
func NewScheduler() *Scheduler {
	s := &Scheduler{
		taskChan: make(chan task, tasksBufferSize),
		doneChan: make(chan struct{}),
	}
	// Point dispatcher at the scheduler’s task channel
	s.delayer = newDelayDispatcher(s.taskChan, s.doneChan, &s.currentDelayedTasks)
	return s
}

// RunScheduler starts the main scheduling loop.
// It drains Scheduler taskChan until closed, then waits for all workers.
// Workers run tasks concurrently, updating the counts of ongoing, finished, and failed tasks.
// This method is blocking, the client should call it in a separate goroutine.
func (s *Scheduler) RunScheduler() {
	// Start workers
	log.Println("💼 Starting Scheduler with", defaultNumOfWorkers, "workers")
	for i := 0; i < defaultNumOfWorkers; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for task := range s.taskChan {
				if task.logger != nil {
					task.logger.Printf("🚀 Starting execution of task - ID: %d, Name: %s", task.id, task.name)
				}
				// Run the task and handle retries
				var err error
				for i := 0; i < task.NumOfRetries; i++ {
					if task.logger != nil {
						task.logger.Printf("🔄 Attempt %d for task - ID: %d", i+1, task.id)
					}
					s.ongoingTasks.Add(1)                // Increment ongoing tasks count
					err = task.Run(context.Background()) // Execute the task's Run method
					s.ongoingTasks.Add(-1)               // Decrement ongoing tasks count
					if err == nil {
						break // Exit loop if task succeeded
					}
				}
				if task.logger != nil {
					if err != nil {
						task.logger.Printf("[task ID:%d] Error: %v", task.id, err)
					} else {
						task.logger.Printf("[task ID:%d] Finished successfully", task.id)
					}
				}
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
// Calling Stop() should be done after client has finished registering all tasks, so that the scheduler can process all tasks before closing the task channel.
// This method is blocking, as it waits for all tasks to finish processing before returning.
func (s *Scheduler) Stop() {
	s.delayer.wg.Wait() // Wait for all delayed tasks to return to the main task channel
	log.Println("🔒 Closing task channel; no more tasks will be registered")
	close(s.taskChan)
	<-s.doneChan // Wait for the scheduler to finish processing all tasks
}
