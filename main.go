package main

import (
	"scheduler/example"
	"scheduler/mocks"
	scheduler "scheduler/task-scheduler"
	"time"
)

func main() {
	// Initialize the task scheduler
	s := scheduler.NewScheduler()
	s.SetMaxWorkers(40)

	// Monitor ongoing tasks every 3 seconds
	go s.SchedulerMonitor(3 * time.Second)

	// Start the scheduler to process registered tasks
	go s.RunScheduler()

	// Register tasks with various decorators prior running the scheduler

	// Default task with no decorators
	s.RegisterTask(&example.Task{}, 100)
	// Task with logging enabled
	s.RegisterTask(&example.Task{}, 200, scheduler.WithLogging())
	// Task with delay, name and logging enabled
	s.RegisterTask(&example.Task{}, 300, scheduler.WithDelay(5*time.Second), scheduler.WithName("TAL-Task-3"), scheduler.WithLogging())
	// Task with retry, delay, name and logging enabled
	s.RegisterTask(&example.Task{}, 400, scheduler.WithRetry(2), scheduler.WithDelay(3*time.Second), scheduler.WithName("TAL-Task-4"), scheduler.WithLogging())
	// Task that simulates an error
	s.RegisterTask(&example.ErrorTask{}, 500, scheduler.WithRetry(3), scheduler.WithName("TAL-Error-Task"), scheduler.WithLogging())
	// Task that simulates task success on the 3rd attempt
	retryTask := mocks.NewRetryTestTask(600, "retryTask", 3, 2)
	s.RegisterTask(retryTask, 600, scheduler.WithRetry(3), scheduler.WithLogging(), scheduler.WithName("retryTask"))

	// Register a range of tasks with different configurations
	for i := 0; i <= 99; i++ {
		if i%10 == 0 { // Every 10th task has logging enabled
			s.RegisterTask(&example.Task{}, i, scheduler.WithLogging(), scheduler.WithRetry(3))
		} else {
			s.RegisterTask(&example.Task{}, i, scheduler.WithRetry(3))

		}
	}

	// Close the scheduler to stop accepting new tasks and wait for all tasks to finish
	s.Stop()

	s.SummaryReport()
}
