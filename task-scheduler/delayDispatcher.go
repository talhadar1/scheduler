package task_scheduler

import (
	"sync"
	"sync/atomic"
	"time"
)

// delayedTask couples a task with its desired execution time.
type delayedTask struct {
	task     task
	execTime time.Time
}

// delayDispatcher manages all delayed tasks in a single goroutine.
// It accepts scheduling requests on delayedTaskChan and pushes tasks into the main scheduler taskChan
// once their execTime arrives.
type delayDispatcher struct {
	taskChan        chan<- task // Scheduler task channel
	doneChan        chan struct{}
	delayedTaskChan chan delayedTask
	delayedCounter  *atomic.Int32 // Scheduler counter for currently delayed tasks
	wg              sync.WaitGroup
}

// newDelayDispatcher launches the single dispatcher goroutine.
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

// schedule enqueues task to be dispatched into delayedTaskChan
func (d *delayDispatcher) schedule(t task, execTime time.Time) {
	d.delayedTaskChan <- delayedTask{task: t, execTime: execTime}
	d.wg.Add(1)
}

// run is the dispatcher’s event loop.
// It maintains a time-ordered slice of pending scheduleRequests and uses one
// time.Timer to wait until the next execTime. When the timer fires, it drains
// all due tasks into taskChan.
func (d *delayDispatcher) run() {
	var queue []delayedTask
	var timer *time.Timer
	var timerCh <-chan time.Time

	for {
		// Timer management - if we have at least one pending request, (re)arm the timer for the soonest execTime
		if len(queue) > 0 {
			next := queue[0].execTime
			wait := time.Until(next)
			if timer == nil {
				timer = time.NewTimer(maxDuration(wait))
			} else {
				timer.Reset(maxDuration(wait))
			}
			timerCh = timer.C
		} else if timer != nil {
			timer.Stop()
			timer = nil
			timerCh = nil
		}

		select {
		case req := <-d.delayedTaskChan: // a new delayed task arrives
			// Insert new delayed task in sorted order
			i := len(queue)
			for j, existing := range queue {
				if req.execTime.Before(existing.execTime) {
					i = j
					break
				}
			}
			queue = append(queue, delayedTask{}) // make space
			copy(queue[i+1:], queue[i:])         // shift elements to the right
			queue[i] = req                       // insert the new task

		case now := <-timerCh:
			// Dispatch all tasks whose time ≤ now
			for len(queue) > 0 && !queue[0].execTime.After(now) {
				d.taskChan <- queue[0].task
				d.wg.Done()              // decrement the wait group for the delayed tasks dispatcher
				d.delayedCounter.Add(-1) // Decrement the delayed tasks counter
				queue = queue[1:]
			}

		case <-d.doneChan: // Scheduler is closing, close dispatcher gracefully
			if timer != nil {
				timer.Stop() // Stop the timer if it exists
				break
			}
		}
	}
}

// maxDuration guards against negative or huge waits.
func maxDuration(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return d
}
