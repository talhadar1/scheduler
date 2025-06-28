package mocks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"
)

// MockTask Mock task for testing
type MockTask struct {
	id              int
	name            string
	shouldFail      bool
	executionTime   time.Duration
	executed        int32
	logger          *log.Logger
	runFunc         func(ctx context.Context) error
	failureCount    int   // How many times to fail before succeeding
	currentAttempts int32 // Thread-safe attempt counter
}

func (m *MockTask) Run(ctx context.Context) error {
	atomic.AddInt32(&m.executed, 1)

	if m.executionTime > 0 {
		select {
		case <-time.After(m.executionTime):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.shouldFail {
		return errors.New("mock task failed")
	}
	return nil
}

func NewMockTask(id int, name string) *MockTask {
	return &MockTask{id: id, name: name}
}

func NewMockTimedTask(id int, name string, duration time.Duration) *MockTask {
	return &MockTask{id: id, name: name, executionTime: duration}
}

func (m *MockTask) SetExecutionTime(executionTime time.Duration) *MockTask {
	m.executionTime = executionTime
	return m
}

func (m *MockTask) SetShouldFail(shouldFail bool) *MockTask {
	m.shouldFail = shouldFail
	return m
}

// ExecutionCount safely returns the number of times a mock task has been executed (i.e., how many times its Run() method was called), using atomic operations to avoid race conditions in concurrent tests.
func (m *MockTask) ExecutionCount() int {
	return int(atomic.LoadInt32(&m.executed))
}

// Test helper: Mock task with configurable retry behavior
type RetryTestTask struct {
	id              int
	name            string
	NumOfRetries    int   // Number of retries to attempt on failure (default 1)
	failureCount    int   // How many times to fail before succeeding (-1 = always fail)
	currentAttempts int32 // Thread-safe attempt counter
	logger          *log.Logger
}

func NewRetryTestTask(id int, name string, retries int, failuresBeforeSuccess int) *RetryTestTask {
	return &RetryTestTask{
		id:           id,
		name:         name,
		NumOfRetries: retries,
		failureCount: failuresBeforeSuccess,
		logger:       log.New(os.Stdout, fmt.Sprintf("[RetryTask-%s] ", name), log.LstdFlags),
	}
}

func (r *RetryTestTask) Run(ctx context.Context) error {
	attempt := atomic.AddInt32(&r.currentAttempts, 1)

	// Simulate some work
	time.Sleep(5 * time.Millisecond)

	// Always fail if failureCount is -1
	if r.failureCount == -1 {
		return fmt.Errorf("task %d always fails (attempt %d)", r.id, attempt)
	}

	// Fail for the specified number of attempts, then succeed
	if int(attempt) <= r.failureCount {
		return fmt.Errorf("task %d failed on attempt %d", r.id, attempt)
	}
	r.logger.Printf("task %d succeeded on attempt %d", r.id, attempt)
	// Success
	return nil
}

func (r *RetryTestTask) GetAttemptCount() int {
	return int(atomic.LoadInt32(&r.currentAttempts))
}
