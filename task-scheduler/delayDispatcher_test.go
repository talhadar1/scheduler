package task_scheduler

import (
	"github.com/stretchr/testify/assert"
	"sync/atomic"
	"testing"
	"time"
)

// TestSingleDelayedTask tests that a single delayed task is executed after its scheduled time.
// It ensures that the task is not executed before the delay and that the counter is decremented correctly.
func TestSingleDelayedTask(t *testing.T) {
	taskChan := make(chan task, 1)
	doneChan := make(chan struct{})
	counter := atomic.Int32{}
	counter.Store(1) // Total number of tasks to be scheduled

	dispatcher := newDelayDispatcher(taskChan, doneChan, &counter)
	now := time.Now()
	dispatcher.schedule(task{id: 1}, now.Add(100*time.Millisecond)) // Schedule a task with a 100ms delay

	select {
	case <-taskChan: // Task should not be received yet
		t.Fatal("Task received too early")
	case <-time.After(80 * time.Millisecond):
		// OK: task not sent yet
	}

	select {
	case tsk := <-taskChan: // Task should be received after the delay
		assert.Equal(t, 0, int(counter.Load())) // Counter should be decremented to 0 after the task is sent
		assert.Equal(t, 1, tsk.id)              // Check that the task ID is correct
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Task not received in time")
	}
}

// TestMultipleDelayedTasksOrder tests that multiple delayed task are executed in the correct order based on their execution time.
// It ensures that task with shorter delays are executed before those with longer delays.
// It uses a buffered channel to hold all task and checks the order of their IDs.
func TestMultipleDelayedTasksOrder(t *testing.T) {
	taskChan := make(chan task, 3) // Buffer size of 3 to hold all tasks
	doneChan := make(chan struct{})
	counter := atomic.Int32{}
	counter.Store(3) // Total number of tasks to be scheduled

	dispatcher := newDelayDispatcher(taskChan, doneChan, &counter)

	now := time.Now()
	dispatcher.schedule(task{id: 1}, now.Add(120*time.Millisecond)) // This task has the longest delay
	dispatcher.schedule(task{id: 2}, now.Add(50*time.Millisecond))  // This task has a medium delay
	dispatcher.schedule(task{id: 3}, now.Add(90*time.Millisecond))  // This task has the shortest delay

	var ids []int
	timeout := time.After(1 * time.Second) // Set a timeout to avoid deadlock in case of issues
	for len(ids) < 3 {                     // Loop to receive tasks from the channel
		select {
		case tsk := <-taskChan: // Received task from the channel
			ids = append(ids, tsk.id) // Collect task ID
			if len(ids) == 3 {
				break // Exit the loop when all tasks are received
			}
		case <-timeout:
			t.Fatal("Timeout waiting for tasks")
		}
	}

	assert.Equal(t, []int{2, 3, 1}, ids, "Tasks should arrive in execTime order")
}
