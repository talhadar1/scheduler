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
					assert.Equal(t, tt.hasNames[i], tempTask.name)
				}
			}
		})
	}
}
