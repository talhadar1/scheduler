package task_scheduler

import (
	"sync"
	"sync/atomic"
	"time"
)

// Struct delayedTask couples a task with its desired execution time.
type delayedTask struct {
	task     task
	execTime time.Time
}

// Struct delayDispatcher manages all delayed tasks in a single goroutine.
// It accepts scheduling requests on delayedTaskChan and pushes tasks into the main scheduler taskChan once their execTime arrives.
type delayDispatcher struct {
	taskChan        chan<- task // Scheduler task channel
	doneChan        chan struct{}
	delayedTaskChan chan delayedTask
	delayedCounter  *atomic.Int32 // Scheduler counter for currently delayed tasks
	wg              sync.WaitGroup
}

// Call newDelayDispatcher launches the single dispatcher goroutine.
func newDelayDispatcher(taskChan chan<- task, doneCh chan struct{}, currentDelayedTasks *atomic.Int32) *delayDispatcher {
	d := &delayDispatcher{
		taskChan:        taskChan,
		delayedTaskChan: make(chan delayedTask),
		delayedCounter:  currentDelayedTasks,
		doneChan:        doneCh,
	}
	go d.run()
	return d
}

// Call schedule enqueues task to be dispatched into delayedTaskChan
func (d *delayDispatcher) schedule(t task, execTime time.Time) {
	d.delayedTaskChan <- delayedTask{task: t, execTime: execTime}
	d.wg.Add(1)
}

// Call run() launches the dispatcher event loop.
// It maintains a time-ordered slice of pending scheduleRequests and uses one time.Timer to wait until the next execTime. When the timer fires, it drains all due tasks into taskChan.
func (d *delayDispatcher) run() {
	var queue []delayedTask      // Slice of pending delayed tasks, sorted by execTime
	var timer *time.Timer        // Timer to wait for the next task execution
	var timerCh <-chan time.Time // Channel to receive timer events

	for {
		// Timer management - if we have at least one pending request, (re)arm the timer for the soonest execTime
		if len(queue) > 0 {
			next := queue[0].execTime // The next execution time is the first in the queue
			wait := time.Until(next)  // Calculate how long to wait until the next task should be executed
			if timer == nil {
				timer = time.NewTimer(maxDuration(wait)) // Create a new timer if it doesn't exist
			} else {
				timer.Reset(maxDuration(wait)) // Reset the existing timer to the new wait duration
			}
			timerCh = timer.C
		} else if timer != nil {
			timer.Stop()  // Stop the timer if there are no pending requests
			timer = nil   // Reset the timer
			timerCh = nil // Reset the timer channel
		}

		select {
		case req := <-d.delayedTaskChan: // A new delayed task arrives
			// Insert new delayed task in sorted order
			queue = delayedTaskInsert(queue, req)

		case now := <-timerCh:
			// Dispatch all tasks whose time ≤ now
			for len(queue) > 0 && !queue[0].execTime.After(now) {
				d.taskChan <- queue[0].task // Send the task to the scheduler's task channel
				d.wg.Done()                 // Decrement the wait group for the delayed tasks dispatcher
				d.delayedCounter.Add(-1)    // Decrement the delayed tasks counter
				queue = queue[1:]           // Remove the first task from the queue
			}

		case <-d.doneChan: // Scheduler is closing, close dispatcher gracefully
			if timer != nil {
				timer.Stop() // Stop the timer if it exists
				break
			}
		}
	}
}

// The maxDuration guards against negative durations.
func maxDuration(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return d
}

// Insert new delayed task in sorted order
func delayedTaskInsert(queue []delayedTask, req delayedTask) []delayedTask {
	i := len(queue)
	for j, existing := range queue {
		if req.execTime.Before(existing.execTime) { // Find the first task with execTime after the new task's execTime
			i = j
			break
		}
	}
	queue = append(queue, delayedTask{}) // Make space
	copy(queue[i+1:], queue[i:])         // Shift elements to the right
	queue[i] = req                       // Insert the new task
	return queue
}
