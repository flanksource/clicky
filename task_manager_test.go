package clicky

import (
	"testing"
	"time"
)

func TestTaskManager(t *testing.T) {
	tm := NewTaskManager()

	task1 := tm.Start("Loading data")
	go func() {
		task1.Infof("Starting data load from database")
		for i := 0; i <= 100; i += 20 {
			task1.SetProgress(i, 100)
			time.Sleep(100 * time.Millisecond)
		}
		task1.Infof("Data load complete")
		task1.Success()
	}()

	task2 := tm.Start("Processing files")
	go func() {
		task2.Infof("Processing %d files", 10)
		for i := 0; i <= 50; i += 10 {
			task2.SetProgress(i, 50)
			if i == 30 {
				task2.Errorf("Failed to process file %d", i/10)
			}
			time.Sleep(150 * time.Millisecond)
		}
		task2.Failed()
	}()

	task3 := tm.Start("Validating results")
	go func() {
		task3.Warnf("Validation issues detected")
		time.Sleep(800 * time.Millisecond)
		task3.Warning()
	}()

	exitCode := tm.Wait()
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 (due to failure), got %d", exitCode)
	}
}

func TestTaskTimeout(t *testing.T) {
	tm := NewTaskManager()
	
	task := tm.Start("Timeout test", WithTimeout(200*time.Millisecond))
	
	go func() {
		time.Sleep(500 * time.Millisecond) // This will timeout
		task.Success() // This should not execute
	}()
	
	exitCode := tm.Wait()
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 (due to timeout), got %d", exitCode)
	}
	
	// Check that task status is failed
	if task.status != StatusFailed {
		t.Errorf("Expected task status to be Failed, got %v", task.status)
	}
}

func TestTaskCancel(t *testing.T) {
	tm := NewTaskManager()
	
	task1 := tm.Start("Task 1")
	task2 := tm.Start("Task 2")
	
	go func() {
		select {
		case <-task1.Context().Done():
			return
		case <-time.After(1 * time.Second):
			task1.Success()
		}
	}()
	
	go func() {
		select {
		case <-task2.Context().Done():
			return
		case <-time.After(1 * time.Second):
			task2.Success()
		}
	}()
	
	// Cancel task1 specifically
	time.Sleep(100 * time.Millisecond)
	task1.Cancel()
	
	// Cancel all remaining tasks
	time.Sleep(100 * time.Millisecond)
	tm.CancelAll()
	
	exitCode := tm.Wait()
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 (due to cancellation), got %d", exitCode)
	}
	
	if task1.status != StatusCancelled {
		t.Errorf("Expected task1 status to be Cancelled, got %v", task1.status)
	}
	if task2.status != StatusCancelled {
		t.Errorf("Expected task2 status to be Cancelled, got %v", task2.status)
	}
}

func TestTaskLogging(t *testing.T) {
	tm := NewTaskManager()
	tm.SetVerbose(true)
	
	task := tm.Start("Log test")
	
	task.Infof("This is an info message")
	task.Warnf("This is a warning: %s", "be careful")
	task.Errorf("This is an error: %v", "something went wrong")
	
	if len(task.logs) != 3 {
		t.Errorf("Expected 3 log entries, got %d", len(task.logs))
	}
	
	if task.logs[0].Level != "info" {
		t.Errorf("Expected first log to be info, got %s", task.logs[0].Level)
	}
	if task.logs[1].Level != "warning" {
		t.Errorf("Expected second log to be warning, got %s", task.logs[1].Level)
	}
	if task.logs[2].Level != "error" {
		t.Errorf("Expected third log to be error, got %s", task.logs[2].Level)
	}
	
	task.Success()
	tm.Wait()
}

func TestTaskProgress(t *testing.T) {
	tm := NewTaskManager()
	
	task := tm.Start("Test task")
	
	task.SetProgress(50, 100)
	if task.progress != 50 || task.maxValue != 100 {
		t.Errorf("Progress not set correctly")
	}
	
	task.SetStatus("Updated task")
	if task.name != "Updated task" {
		t.Errorf("Status not updated correctly")
	}
	
	task.Success()
	if task.status != StatusSuccess {
		t.Errorf("Task status should be Success")
	}
	
	tm.Wait()
}

func TestTaskDuration(t *testing.T) {
	task := &Task{
		startTime: time.Now(),
		endTime:   time.Now().Add(1500 * time.Millisecond),
	}
	
	duration := task.getDuration()
	if duration != "1.5s" {
		t.Errorf("Expected duration '1.5s', got '%s'", duration)
	}
	
	task.endTime = task.startTime.Add(500 * time.Millisecond)
	duration = task.getDuration()
	if duration != "500ms" {
		t.Errorf("Expected duration '500ms', got '%s'", duration)
	}
}

func TestContextCancellation(t *testing.T) {
	tm := NewTaskManager()
	
	task := tm.Start("Context test")
	
	ctx := task.Context()
	if ctx == nil {
		t.Error("Task context should not be nil")
	}
	
	// Start a goroutine that waits on context
	done := make(chan bool)
	go func() {
		select {
		case <-ctx.Done():
			done <- true
		case <-time.After(1 * time.Second):
			done <- false
		}
	}()
	
	// Cancel the task
	task.Cancel()
	
	// Check if context was cancelled
	result := <-done
	if !result {
		t.Error("Context should have been cancelled")
	}
	
	tm.Wait()
}

func TestVerboseMode(t *testing.T) {
	tm := NewTaskManager()
	
	// Test verbose is picked up from env var
	originalVerbose := tm.verbose
	
	// Test SetVerbose
	tm.SetVerbose(true)
	if !tm.verbose {
		t.Error("Verbose should be true after SetVerbose(true)")
	}
	
	tm.SetVerbose(false)
	if tm.verbose {
		t.Error("Verbose should be false after SetVerbose(false)")
	}
	
	// Restore original state
	tm.SetVerbose(originalVerbose)
}