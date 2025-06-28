package example

import (
	"context"
	"fmt"
	"time"
)

type Task struct {
	Id int
}

func (t *Task) ID() int {
	return t.Id
}

func (t *Task) Run(ctx context.Context) error {
	time.Sleep(1 * time.Second) // simulate work
	return nil
}

type ErrorTask struct {
	Id int
}

func (t *ErrorTask) ID() int {
	return t.Id
}
func (t *ErrorTask) Run(ctx context.Context) error {
	time.Sleep(5 * time.Second) // simulate work
	return fmt.Errorf("simulated error in task %d", t.Id)
}
