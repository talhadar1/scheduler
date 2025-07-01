package task_scheduler

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"scheduler/mocks"
	"testing"
	"time"
)

func TestScheduler_RunScheduler(t *testing.T) {
	tests := []struct {
		name                string
		tasks               []RegisterableTask
		expectedFinished    int
		expectedFailed      int
		checkConcurrency    bool
		maxExpectedDuration time.Duration
		testTimeout         time.Duration
	}{
		{
			name:             "successful tasks",
			tasks:            []RegisterableTask{mocks.NewMockTask(1, "task-1"), mocks.NewMockTask(2, "task-2")},
			expectedFinished: 2,
			expectedFailed:   0,
		},
		{
			name:             "tasks with failures",
			tasks:            []RegisterableTask{mocks.NewMockTask(1, "task-1"), mocks.NewMockTask(2, "task-2").SetShouldFail(true)},
			expectedFinished: 1,
			expectedFailed:   1,
		},
		{
			name:                "concurrent execution",
			tasks:               createMultipleTimedTasks(10, 50*time.Millisecond),
			expectedFinished:    10,
			expectedFailed:      0,
			checkConcurrency:    true,
			maxExpectedDuration: 100 * time.Millisecond, // Sufficient time to process 10 tasks with 50 ms each with default 50 workers concurrently
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScheduler()
			// Register tasks
			for i, task := range tt.tasks {
				s.RegisterTask(task, i)
			}

			start := time.Now()

			go s.RunScheduler()
			// Wait for tasks to finish
			s.Stop()

			duration := time.Since(start)

			// Check results
			assert.Equal(t, tt.expectedFinished, int(s.finishedTasks.Load()))
			assert.Equal(t, tt.expectedFailed, int(s.failedTasks.Load()))

			if tt.checkConcurrency {
				assert.Less(t, duration, tt.maxExpectedDuration, "Tasks should execute concurrently")
			}
		})
	}
}

// Helper function to create mock tasks with a fixed execution time
func createMultipleTimedTasks(count int, duration time.Duration) []RegisterableTask {
	tasks := make([]RegisterableTask, count)
	for i := 0; i < count; i++ {
		tasks[i] = mocks.NewMockTimedTask(i, fmt.Sprintf("task-%d", i), duration)
	}
	return tasks
}

func TestScheduler_RegisterTask(t *testing.T) {
	tests := []struct {
		name                    string
		tasks                   []RegisterableTask
		options                 [][]TaskOption
		expectedRegisteredTasks int32
		hasNames                []string
		hasDelays               []time.Duration
	}{
		{
			name:                    "register single task",
			tasks:                   []RegisterableTask{mocks.NewMockTask(1, "test-task")},
			options:                 [][]TaskOption{},
			expectedRegisteredTasks: 1,
		},
		{
			name: "register multiple tasks",
			tasks: []RegisterableTask{
				mocks.NewMockTask(1, "task-1"),
				mocks.NewMockTask(2, "task-2"),
				mocks.NewMockTask(3, "task-3"),
			},
			expectedRegisteredTasks: 3,
		},
		{
			name:  "register task with name decorator",
			tasks: []RegisterableTask{mocks.NewMockTask(1, "")},
			options: [][]TaskOption{{
				WithName("test-task"),
			}},
			expectedRegisteredTasks: 1,
			hasNames:                []string{"test-task"},
		},
		{
			name:  "register task with 2 name decorators - last one should be used",
			tasks: []RegisterableTask{mocks.NewMockTask(1, "test-task")},
			options: [][]TaskOption{{
				WithName("test-wrong-name"),
				WithName("test-correct-name"),
			}},
			expectedRegisteredTasks: 1,
			hasNames:                []string{"test-correct-name"},
		},
		{
			name:  "register task with delay decorator",
			tasks: []RegisterableTask{mocks.NewMockTask(1, "")},
			options: [][]TaskOption{{
				WithDelay(100 * time.Millisecond),
			}},
			expectedRegisteredTasks: 1,
			hasDelays:               []time.Duration{100 * time.Millisecond},
		},
		{
			name:  "register task with name & delay decorators",
			tasks: []RegisterableTask{mocks.NewMockTask(1, "")},
			options: [][]TaskOption{{
				WithName("test-task"),
				WithDelay(100 * time.Millisecond),
			}},
			expectedRegisteredTasks: 1,
			hasDelays:               []time.Duration{100 * time.Millisecond},
			hasNames:                []string{"test-task"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewScheduler()

			for i, task := range tt.tasks {
				if len(tt.options) > 0 {
					s.RegisterTask(task, i, tt.options[i]...)
				} else {
					s.RegisterTask(task, i)
				}
			}
			assert.Equal(t, tt.expectedRegisteredTasks, s.registeredTasks.Load())
			if len(tt.hasNames) > 0 {
				for i := range tt.tasks {
					// Pop a task from the taskChan check name
					tempTask := <-s.taskChan
					s.taskChan <- tempTask // push it back to the channel
					assert.Equal(t, tt.hasNames[i], tempTask.name)
				}
			}
			if len(tt.hasDelays) > 0 {
				for i := range tt.tasks {
					// pop a task from the taskChan check delay
					tempTask := <-s.taskChan // pop a task from the channel
					assert.Equal(t, tt.hasDelays[i], tempTask.delay)
					s.taskChan <- tempTask // push it back to the channel
				}
			}
		})
	}
}

func TestScheduler_RunScheduler_RetryMechanism(t *testing.T) {
	tests := []struct {
		name             string
		description      string
		task             *mocks.RetryTestTask
		expectedFinished int32
		expectedFailed   int32
		expectedAttempts int
	}{
		{
			name:             "task_succeeds_first_attempt",
			description:      "Task succeeds immediately, no retries needed",
			task:             mocks.NewRetryTestTask(1, "success-first", 3, 0), // 0 failures before success
			expectedFinished: 1,
			expectedFailed:   0,
			expectedAttempts: 1,
		},
		{
			name:             "task_succeeds_after_failures",
			description:      "Task fails twice, succeeds on third attempt",
			task:             mocks.NewRetryTestTask(2, "success-after-retries", 5, 2), // 2 failures before success
			expectedFinished: 1,
			expectedFailed:   0,
			expectedAttempts: 3,
		},
		{
			name:             "task_fails_all_attempts",
			description:      "Task fails all retry attempts",
			task:             mocks.NewRetryTestTask(3, "always-fails", 3, -1), // -1 means always fail
			expectedFinished: 0,
			expectedFailed:   1,
			expectedAttempts: 3,
		},
		{
			name:             "task_succeeds_on_last_attempt",
			description:      "Task succeeds exactly on the last allowed attempt",
			task:             mocks.NewRetryTestTask(5, "last-chance-success", 4, 3), // 3 failures, success on 4th
			expectedFinished: 1,
			expectedFailed:   0,
			expectedAttempts: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing: %s", tt.description)

			// ARRANGE: Create scheduler and register task
			s := NewScheduler()
			s.RegisterTask(tt.task, 1, WithRetry(tt.task.NumOfRetries))

			// ACT: Run the scheduler
			schedulerDone := make(chan bool)
			go func() {
				defer close(schedulerDone)
				s.RunScheduler()
			}()

			// Stop the scheduler after a brief delay
			go func() {
				time.Sleep(100 * time.Millisecond) // Allow task to complete
				s.Stop()
			}()

			// Wait for scheduler completion
			<-schedulerDone

			// ASSERT: Verify retry mechanism behavior

			// Verify final task counts
			assert.Equal(t, tt.expectedFinished, s.finishedTasks.Load(),
				"Finished task count should match expected")
			assert.Equal(t, tt.expectedFailed, s.failedTasks.Load(),
				"Failed task count should match expected")

			// Verify retry attempts
			assert.Equal(t, tt.expectedAttempts, tt.task.GetAttemptCount(),
				"Task should be attempted exactly %d times", tt.expectedAttempts)

		})
	}
}
