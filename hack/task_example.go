package main

import (
	"fmt"
	"os"
	"time"

	"github.com/flanksource/clicky"
)

func main() {
	tm := clicky.NewTaskManager()
	
	// Enable verbose mode if -v flag is passed
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--verbose" {
			tm.SetVerbose(true)
			break
		}
	}

	task1 := tm.Start("Downloading files")
	go func() {
		task1.Infof("Connecting to server...")
		time.Sleep(100 * time.Millisecond)
		task1.Infof("Starting download of 10 files")
		for i := 0; i <= 100; i += 5 {
			task1.SetProgress(i, 100)
			if i == 50 {
				task1.Warnf("Slow connection detected")
			}
			time.Sleep(50 * time.Millisecond)
		}
		task1.Infof("Download complete")
		task1.Success()
	}()

	task2 := tm.Start("Processing data")
	go func() {
		time.Sleep(200 * time.Millisecond)
		task2.Infof("Loading dataset...")
		for i := 0; i <= 75; i += 15 {
			task2.SetProgress(i, 75)
			if i == 45 {
				task2.Errorf("Invalid data in row %d", i)
			}
			time.Sleep(100 * time.Millisecond)
		}
		task2.Errorf("Processing failed due to data errors")
		task2.Failed()
	}()

	task3 := tm.Start("Analyzing results")
	go func() {
		time.Sleep(300 * time.Millisecond)
		task3.Infof("Starting analysis...")
		for i := 0; i <= 50; i += 10 {
			task3.SetProgress(i, 50)
			if i == 20 {
				task3.Warnf("Anomaly detected in dataset")
			}
			time.Sleep(80 * time.Millisecond)
		}
		task3.Warnf("Analysis completed with warnings")
		task3.Warning()
	}()

	// Task with timeout
	task4 := tm.Start("Database query", clicky.WithTimeout(1*time.Second))
	go func() {
		task4.Infof("Connecting to database...")
		time.Sleep(200 * time.Millisecond)
		task4.Infof("Running query...")
		
		// Simulate a long-running query that respects context
		select {
		case <-task4.Context().Done():
			task4.Infof("Query cancelled")
			return
		case <-time.After(2 * time.Second): // This will timeout
			task4.Success()
		}
	}()

	task5 := tm.Start("Generating report")
	go func() {
		time.Sleep(600 * time.Millisecond)
		task5.SetStatus("Compiling results...")
		task5.Infof("Processing %d data points", 1000)
		time.Sleep(500 * time.Millisecond)
		task5.SetStatus("Writing output...")
		task5.Infof("Generating PDF report")
		time.Sleep(300 * time.Millisecond)
		task5.Success()
	}()

	// Demonstrate cancellation
	if len(os.Args) > 1 && os.Args[1] == "cancel" {
		task6 := tm.Start("Background job")
		go func() {
			task6.Infof("Starting background processing...")
			select {
			case <-task6.Context().Done():
				task6.Infof("Job cancelled by user")
				return
			case <-time.After(5 * time.Second):
				task6.Success()
			}
		}()
		
		// Cancel after 1 second
		time.Sleep(1 * time.Second)
		fmt.Println("\nCancelling background job...")
		task6.Cancel()
	}

	// Demonstrate fatal error
	if len(os.Args) > 1 && os.Args[1] == "fatal" {
		task7 := tm.Start("Critical operation")
		go func() {
			time.Sleep(1 * time.Second)
			task7.Errorf("Critical failure detected")
			task7.Fatal(fmt.Errorf("critical system failure"))
		}()
	}

	// Demonstrate cancel all
	if len(os.Args) > 1 && os.Args[1] == "cancelall" {
		// Start some long-running tasks
		for i := 1; i <= 3; i++ {
			taskName := fmt.Sprintf("Long task %d", i)
			task := tm.Start(taskName)
			go func(t *clicky.Task, id int) {
				t.Infof("Starting long operation %d", id)
				select {
				case <-t.Context().Done():
					t.Infof("Task %d cancelled", id)
					return
				case <-time.After(10 * time.Second):
					t.Success()
				}
			}(task, i)
		}
		
		// Cancel all after 2 seconds
		time.Sleep(2 * time.Second)
		fmt.Println("\nCancelling all tasks...")
		tm.CancelAll()
	}

	exitCode := tm.Wait()
	os.Exit(exitCode)
}