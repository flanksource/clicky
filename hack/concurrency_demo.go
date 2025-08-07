package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/flanksource/clicky"
)

func main() {
	maxConcurrent := 3
	if len(os.Args) > 1 {
		if c, err := strconv.Atoi(os.Args[1]); err == nil {
			maxConcurrent = c
		}
	}

	fmt.Printf("=== Concurrency Demo (Max %d concurrent tasks) ===\n\n", maxConcurrent)

	tm := clicky.NewTaskManagerWithConcurrency(maxConcurrent)
	tm.SetVerbose(true)

	// Create 10 tasks that take different amounts of time
	for i := 1; i <= 10; i++ {
		taskNum := i
		duration := time.Duration(100+taskNum*50) * time.Millisecond
		
		tm.Start(fmt.Sprintf("Task %d", taskNum), 
			clicky.WithFunc(func(task *clicky.Task) error {
				task.Infof("Starting work (duration: %v)", duration)
				
				// Simulate work with progress updates
				steps := 10
				for step := 1; step <= steps; step++ {
					time.Sleep(duration / time.Duration(steps))
					task.SetProgress(step*10, 100)
				}
				
				task.Infof("Completed successfully")
				return nil
			}),
		)
	}

	fmt.Printf("Started 10 tasks with max concurrency of %d\n", maxConcurrent)
	fmt.Printf("Watch how tasks queue up when concurrency limit is reached...\n\n")

	startTime := time.Now()
	exitCode := tm.Wait()
	totalTime := time.Since(startTime)

	fmt.Printf("\nCompleted in %v with exit code %d\n", totalTime, exitCode)
	
	if maxConcurrent == 1 {
		fmt.Printf("With concurrency=1, tasks ran sequentially\n")
	} else {
		fmt.Printf("With concurrency=%d, tasks ran in batches\n", maxConcurrent)
	}
}