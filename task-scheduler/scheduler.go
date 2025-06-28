package task_scheduler

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
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
	id           int              // unique identifier for the task
	name         string           // human-readable name
	inner        RegisterableTask // wrapped RegisterableTask that provides the actual Run logic
	logger       *log.Logger      // optional logger for Task-specific output
	delay        time.Duration    // delay duration before execution
	NumOfRetries int              // number of times to retry on failure
}

// newTask creates a new task with the given parameters.
// It initializes the task with an id, name, and an inner RegisterableTask.
func newTask(inner RegisterableTask, id int, opts ...TaskOption) *task {
	t := &task{
		id:           id,
		name:         fmt.Sprintf("task-%d", id), // default name format
		inner:        inner,
		logger:       nil, // default no logger
		delay:        0,   // default no delay
		NumOfRetries: 1,   // default no retries
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

// RegisterTask submits a task for execution.
func (s *Scheduler) RegisterTask(t RegisterableTask, id int, opts ...TaskOption) {
	task := newTask(t, id, opts...)
	s.registeredTasks++
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
	taskChan            chan task        // channel for tasks to be executed
	doneChan            chan struct{}    // channel to signal completion of all tasks
	registeredTasks     int              // total number of tasks registered
	wg                  sync.WaitGroup   // wait group to synchronize worker goroutines
	ongoingTasks        atomic.Int32     // count of currently running tasks
	failedTasks         atomic.Int32     // count of tasks that failed
	finishedTasks       atomic.Int32     // count of tasks that finished successfully
	delayer             *delayDispatcher // delayDispatcher manages delayed tasks
	currentDelayedTasks atomic.Int32     // count of currently delayed tasks used by the delayDispatcher
	maxWorkers          int              // maximum number of concurrent workers
}

// NewScheduler creates a new Scheduler instance.
// It initializes the task channel and done channel,
// and sets the maximum number of concurrent workers.
func NewScheduler() *Scheduler {
	s := &Scheduler{
		taskChan: make(chan task, 100),
		doneChan: make(chan struct{}),
	}
	// point dispatcher at the scheduler’s task channel
	s.delayer = newDelayDispatcher(s.taskChan, s.doneChan, &s.currentDelayedTasks)
	return s
}

// RunScheduler starts the main scheduling loop.
// It drains Scheduler taskChan until closed, then waits for all workers.
// Workers run tasks concurrently, updating the counts of ongoing, finished, and failed tasks.
// It uses a mutex to protect access to the Scheduler counters.
func (s *Scheduler) RunScheduler() {
	if s.maxWorkers == 0 {
		s.maxWorkers = defaultNumOfWorkers // Set default number of workers if not specified
	}
	log.Println("💼 Starting Scheduler with", s.maxWorkers, "workers")
	//start workers
	for i := 0; i < s.maxWorkers; i++ {
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
// calling Stop() should be done after client has finished registering all tasks,
// so that the scheduler can process all tasks before closing the task channel.
func (s *Scheduler) Stop() {
	s.delayer.wg.Wait() // Wait for all delayed tasks to return to the main task channel
	log.Println("🔒 Closing task channel; no more tasks will be registered")
	close(s.taskChan)
	<-s.doneChan // Wait for the scheduler to finish processing all tasks
}

// SetMaxWorkers configures the maximum number of concurrent tasks.
// It allows dynamic adjustment of the worker count.
// If the value is set to 0 or less, it defaults to the predefined defaultNumOfWorkers.
// This method can be called at any time before or during the execution of tasks.
func (s *Scheduler) SetMaxWorkers(maxWorkers int) {
	if maxWorkers <= 0 || 100 < maxWorkers {
		s.maxWorkers = defaultNumOfWorkers // Set default number of workers if not specified
		log.Printf("⚠️ Invalid maxWorkers value %d; using default: %d", maxWorkers, defaultNumOfWorkers)
	} else {
		s.maxWorkers = maxWorkers
		log.Printf("✅ Using configured maxWorkers: %d", s.maxWorkers)
	}
}

// SchedulerMonitor prints the current status of the Scheduler at client defined intervals.
func (s *Scheduler) SchedulerMonitor(d time.Duration) {
	const (
		Cyan  = "\033[36m"
		Reset = "\033[0m"
	)
	for {
		fmt.Printf(
			Cyan+"🌀 Ongoing: %d | Registered: %d | Delayed: %d | Finished: %d | Failed: %d | Goroutines: %d\n"+Reset,
			s.ongoingTasks.Load(), s.registeredTasks, s.currentDelayedTasks.Load(), s.finishedTasks.Load(), s.failedTasks.Load(), runtime.NumGoroutine(),
		)
		time.Sleep(d)
	}
}
