package clicky

import (
	"testing"
	"time"
)

func TestTaskManager(t *testing.T) {
	tm := NewTaskManager()

	task1 := tm.Start("Loading data")
	go func() {
		for i := 0; i <= 100; i += 20 {
			task1.SetProgress(i, 100)
			time.Sleep(100 * time.Millisecond)
		}
		task1.Success()
	}()

	task2 := tm.Start("Processing files")
	go func() {
		for i := 0; i <= 50; i += 10 {
			task2.SetProgress(i, 50)
			time.Sleep(150 * time.Millisecond)
		}
		task2.Failed()
	}()

	task3 := tm.Start("Validating results")
	go func() {
		time.Sleep(800 * time.Millisecond)
		task3.Warning()
	}()

	exitCode := tm.Wait()
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 (due to failure), got %d", exitCode)
	}
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