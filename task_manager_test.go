package clicky

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestTaskManager(t *testing.T) {
	tm := NewTaskManager()

	task1 := tm.Start("Loading data", WithFunc(func(task *Task) error {
		task.Infof("Starting data load from database")
		for i := 0; i <= 100; i += 20 {
			task.SetProgress(i, 100)
			time.Sleep(100 * time.Millisecond)
		}
		task.Infof("Data load complete")
		return nil
	}))

	task2 := tm.Start("Processing files", WithFunc(func(task *Task) error {
		task.Infof("Processing %d files", 10)
		for i := 0; i <= 50; i += 10 {
			task.SetProgress(i, 50)
			if i == 30 {
				task.Errorf("Failed to process file %d", i/10)
			}
			time.Sleep(150 * time.Millisecond)
		}
		return errors.New("processing failed")
	}))

	task3 := tm.Start("Validating results", WithFunc(func(task *Task) error {
		task.Warnf("Validation issues detected")
		time.Sleep(800 * time.Millisecond)
		task.Warning()
		return nil
	}))

	// Wait for specific tasks to ensure they're done
	time.Sleep(1 * time.Second)

	exitCode := tm.Wait()
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 (due to failure), got %d", exitCode)
	}

	// Check individual task statuses
	if task1.Status() != StatusSuccess {
		t.Errorf("Task1 should be Success, got %v", task1.Status())
	}
	if task2.Status() != StatusFailed {
		t.Errorf("Task2 should be Failed, got %v", task2.Status())
	}
	if task3.Status() != StatusWarning {
		t.Errorf("Task3 should be Warning, got %v", task3.Status())
	}
}

func TestTaskTimeout(t *testing.T) {
	tm := NewTaskManager()
	
	task := tm.Start("Timeout test", 
		WithTimeout(200*time.Millisecond),
		WithFunc(func(task *Task) error {
			time.Sleep(500 * time.Millisecond) // This will timeout
			return nil
		}))
	
	exitCode := tm.Wait()
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 (due to timeout), got %d", exitCode)
	}
	
	// Check that task status is failed
	if task.Status() != StatusFailed {
		t.Errorf("Expected task status to be Failed, got %v", task.Status())
	}
}

func TestTaskCancel(t *testing.T) {
	tm := NewTaskManager()
	
	task1 := tm.Start("Task 1", WithFunc(func(task *Task) error {
		select {
		case <-task.Context().Done():
			return nil
		case <-time.After(1 * time.Second):
			return nil
		}
	}))
	
	task2 := tm.Start("Task 2", WithFunc(func(task *Task) error {
		select {
		case <-task.Context().Done():
			return nil
		case <-time.After(1 * time.Second):
			return nil
		}
	}))
	
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
	
	if task1.Status() != StatusCancelled {
		t.Errorf("Expected task1 status to be Cancelled, got %v", task1.Status())
	}
	if task2.Status() != StatusCancelled {
		t.Errorf("Expected task2 status to be Cancelled, got %v", task2.Status())
	}
}

func TestTaskLogging(t *testing.T) {
	tm := NewTaskManager()
	tm.SetVerbose(true)
	
	task := tm.Start("Log test", WithFunc(func(task *Task) error {
		task.Infof("This is an info message")
		task.Warnf("This is a warning: %s", "be careful")
		task.Errorf("This is an error: %v", "something went wrong")
		return nil
	}))
	
	// Let task run
	tm.Wait()
	
	if len(task.logs) != 3 {
		t.Errorf("Expected 3 log entries, got %d", len(task.logs))
	}
	
	if len(task.logs) >= 3 {
		if task.logs[0].Level != "info" {
			t.Errorf("Expected first log to be info, got %s", task.logs[0].Level)
		}
		if task.logs[1].Level != "warning" {
			t.Errorf("Expected second log to be warning, got %s", task.logs[1].Level)
		}
		if task.logs[2].Level != "error" {
			t.Errorf("Expected third log to be error, got %s", task.logs[2].Level)
		}
	}
}

func TestTaskProgress(t *testing.T) {
	tm := NewTaskManager()
	
	task := tm.Start("Test task", WithFunc(func(task *Task) error {
		task.SetProgress(50, 100)
		task.SetStatus("Updated task")
		return nil
	}))
	
	tm.Wait()
	
	if task.progress != 50 || task.maxValue != 100 {
		t.Errorf("Progress not set correctly: %d/%d", task.progress, task.maxValue)
	}
	
	if task.name != "Updated task" {
		t.Errorf("Status not updated correctly: %s", task.name)
	}
	
	if task.Status() != StatusSuccess {
		t.Errorf("Task status should be Success, got %v", task.Status())
	}
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
	
	task := tm.Start("Context test", WithFunc(func(task *Task) error {
		select {
		case <-task.Context().Done():
			return nil
		case <-time.After(1 * time.Second):
			return errors.New("should have been cancelled")
		}
	}))
	
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

func TestConcurrencyLimit(t *testing.T) {
	tm := NewTaskManagerWithConcurrency(2) // Max 2 concurrent tasks
	
	startTimes := make([]time.Time, 5)
	endTimes := make([]time.Time, 5)
	
	// Start 5 tasks but only 2 should run at a time
	for i := 0; i < 5; i++ {
		idx := i
		tm.Start(fmt.Sprintf("Task %d", i+1), WithFunc(func(task *Task) error {
			startTimes[idx] = time.Now()
			time.Sleep(200 * time.Millisecond)
			endTimes[idx] = time.Now()
			return nil
		}))
	}
	
	tm.Wait()
	
	// Check that no more than 2 tasks ran simultaneously
	maxConcurrent := 0
	for i := 0; i < 5; i++ {
		concurrent := 0
		for j := 0; j < 5; j++ {
			if i != j && startTimes[i].Before(endTimes[j]) && endTimes[i].After(startTimes[j]) {
				concurrent++
			}
		}
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
	}
	
	// Max concurrent should be 1 (since each task sees at most 1 other)
	if maxConcurrent > 1 {
		t.Errorf("Expected max 1 other concurrent task, but got %d", maxConcurrent)
	}
}

func TestTaskError(t *testing.T) {
	tm := NewTaskManager()
	
	expectedErr := errors.New("test error")
	task := tm.Start("Error task", WithFunc(func(task *Task) error {
		return expectedErr
	}))
	
	tm.Wait()
	
	if task.Status() != StatusFailed {
		t.Errorf("Expected task status to be Failed, got %v", task.Status())
	}
	
	if task.Error() != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, task.Error())
	}
}

func TestPendingStatus(t *testing.T) {
	tm := NewTaskManagerWithConcurrency(1) // Only 1 task at a time
	
	// Start first task that blocks
	task1 := tm.Start("Blocking task", WithFunc(func(task *Task) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}))
	
	// Start second task that should be pending
	task2 := tm.Start("Pending task", WithFunc(func(task *Task) error {
		return nil
	}))
	
	// Check status immediately
	time.Sleep(50 * time.Millisecond)
	if task1.Status() != StatusRunning {
		t.Errorf("Task1 should be Running, got %v", task1.Status())
	}
	if task2.Status() != StatusPending {
		t.Errorf("Task2 should be Pending, got %v", task2.Status())
	}
	
	tm.Wait()
	
	// Both should be successful now
	if task1.Status() != StatusSuccess {
		t.Errorf("Task1 should be Success, got %v", task1.Status())
	}
	if task2.Status() != StatusSuccess {
		t.Errorf("Task2 should be Success, got %v", task2.Status())
	}
}