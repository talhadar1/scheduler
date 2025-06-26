package mocks

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
	"time"
)

// MockTask Mock task for testing
type MockTask struct {
	id            int
	name          string
	shouldFail    bool
	executionTime time.Duration
	executed      int32
	logger        *log.Logger
	runFunc       func(ctx context.Context) error
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
