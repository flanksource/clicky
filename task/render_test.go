package task

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	flanksourceContext "github.com/flanksource/commons/context"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/text"
)

// newTestManager creates a manager suitable for testing: no progress display,
// no color, and no background render loop. This prevents data races on
// os.Stderr when tests redirect it for output capture.
func newTestManager(concurrency int) *Manager {
	tm := newManagerWithConcurrency(concurrency)
	tm.stopRender()
	tm.noProgress.Store(true)
	tm.noColor.Store(true)
	return tm
}

func countNonEmptyLines(s string) int {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func TestRenderLineCount_NTasksProduceNLines(t *testing.T) {
	tests := []struct {
		name          string
		numTasks      int
		expectedLines int
	}{
		{"1 task = 1 line", 1, 1},
		{"3 tasks = 3 lines", 3, 3},
		{"5 tasks = 5 lines", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original stderr and global state
			originalStderr := os.Stderr

			// Create pipe for capturing stderr
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("failed to create pipe: %v", err)
			}

			// Ensure cleanup happens even on test failure
			t.Cleanup(func() {
				os.Stderr = originalStderr
				_ = w.Close()
				_ = r.Close()
			})

			// Redirect stderr to our pipe
			os.Stderr = w

			// Create buffer and copy goroutine
			stderrCapture := &bytes.Buffer{}
			done := make(chan struct{})
			go func() {
				io.Copy(stderrCapture, r)
				close(done)
			}()

			// Create a fresh manager for this test (render loop already stopped)
			testManager := newTestManager(4)

			// Create tasks that complete immediately
			for i := 0; i < tt.numTasks; i++ {
				taskName := fmt.Sprintf("task-%d", i)
				task := testManager.newTask(taskName)
				task.runFunc = func(ctx flanksourceContext.Context, t *Task) error {
					t.Success()
					return nil
				}
				testManager.enqueue(task)
			}

			// Wait for tasks to complete
			timeout := time.After(5 * time.Second)
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

		waitLoop:
			for {
				select {
				case <-timeout:
					t.Fatal("timeout waiting for tasks to complete")
				case <-ticker.C:
					if testManager.taskQueue.Empty() && testManager.workersActive.Load() == 0 {
						allComplete := true
						testManager.mu.RLock()
						for _, task := range testManager.tasks {
							if !task.completed.Load() {
								allComplete = false
								break
							}
						}
						testManager.mu.RUnlock()
						if allComplete {
							break waitLoop
						}
					}
				}
			}

			// Force render
			testManager.PlainRender()

			// Close write end and wait for reader to finish
			w.Close()
			<-done

			// Restore stderr
			os.Stderr = originalStderr

			// Analyze captured output
			output := stderrCapture.String()
			stripped := text.StripANSI(output)
			lines := countNonEmptyLines(stripped)

			if lines != tt.expectedLines {
				t.Errorf("expected %d lines, got %d\nOutput:\n%s", tt.expectedLines, lines, stripped)
			}

			// Cleanup
			close(testManager.shutdown)
		})
	}
}

func TestRenderLineCount_PendingTaskLimit(t *testing.T) {
	// Create a manager with pending tasks that exceed the limit (>5)
	testManager := newTestManager(1)

	// Create 10 pending tasks without running them
	// We do this by creating tasks with dependencies that will never be met
	for i := 0; i < 10; i++ {
		taskName := fmt.Sprintf("pending-task-%d", i)
		task := testManager.newTask(taskName)
		// Don't enqueue - just add to the task list directly to keep them pending
		testManager.mu.Lock()
		testManager.tasks = append(testManager.tasks, task)
		task.dirty.Store(true)
		testManager.mu.Unlock()
	}

	// Get the pretty output
	output := testManager.Pretty()
	rendered := output.String()
	stripped := text.StripANSI(rendered)
	lines := countNonEmptyLines(stripped)

	// Should show 5 tasks + 1 summary line = 6 lines
	expectedLines := 6
	if lines != expectedLines {
		t.Errorf("expected %d lines (5 pending + 1 summary), got %d\nOutput:\n%s", expectedLines, lines, stripped)
	}

	// Verify the summary message is present
	if !strings.Contains(stripped, "5 more pending") {
		t.Errorf("expected '5 more pending' in output, got:\n%s", stripped)
	}

	// Cleanup
	close(testManager.shutdown)
}

func TestRenderLineCount_MixedStatus(t *testing.T) {
	testManager := newTestManager(4)

	// Create 2 completed tasks
	for i := 0; i < 2; i++ {
		taskName := fmt.Sprintf("completed-%d", i)
		task := testManager.newTask(taskName)
		task.SetStatus(StatusSuccess)
		task.completed.Store(true)
		task.dirty.Store(true)
		testManager.mu.Lock()
		testManager.tasks = append(testManager.tasks, task)
		testManager.mu.Unlock()
	}

	// Create 3 pending tasks
	for i := 0; i < 3; i++ {
		taskName := fmt.Sprintf("pending-%d", i)
		task := testManager.newTask(taskName)
		task.dirty.Store(true)
		testManager.mu.Lock()
		testManager.tasks = append(testManager.tasks, task)
		testManager.mu.Unlock()
	}

	// Get the pretty output
	output := testManager.Pretty()
	rendered := output.String()
	stripped := text.StripANSI(rendered)
	lines := countNonEmptyLines(stripped)

	// Should show 2 completed + 3 pending = 5 lines
	expectedLines := 5
	if lines != expectedLines {
		t.Errorf("expected %d lines (2 completed + 3 pending), got %d\nOutput:\n%s", expectedLines, lines, stripped)
	}

	// Cleanup
	close(testManager.shutdown)
}

func TestRenderLineCount_CompletedTaskCollapse(t *testing.T) {
	// Set terminal height to 10 lines so 15 completed tasks must collapse
	api.SetTerminalLines(10)
	t.Cleanup(func() { api.SetTerminalLines(-1) })

	testManager := newTestManager(1)

	for i := 0; i < 15; i++ {
		task := testManager.newTask(fmt.Sprintf("completed-%d", i))
		task.SetStatus(StatusSuccess)
		task.completed.Store(true)
		task.dirty.Store(true)
		testManager.mu.Lock()
		testManager.tasks = append(testManager.tasks, task)
		testManager.mu.Unlock()
	}

	output := testManager.Pretty()
	rendered := output.String()
	stripped := text.StripANSI(rendered)
	lines := countNonEmptyLines(stripped)

	// Should show first + summary + last = 3 lines
	if lines != 3 {
		t.Errorf("expected 3 lines (first + summary + last), got %d\nOutput:\n%s", lines, stripped)
	}

	// First and last tasks should be visible
	if !strings.Contains(stripped, "completed-0") {
		t.Errorf("expected first task 'completed-0' in output, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "completed-14") {
		t.Errorf("expected last task 'completed-14' in output, got:\n%s", stripped)
	}
	// Summary should show 13 collapsed
	if !strings.Contains(stripped, "13 more") {
		t.Errorf("expected '13 more' in output, got:\n%s", stripped)
	}

	close(testManager.shutdown)
}

func TestRenderLineCount_CompletedCollapseWithPending(t *testing.T) {
	// Terminal height 10, with 3 pending tasks leaves 7 for completed
	api.SetTerminalLines(10)
	t.Cleanup(func() { api.SetTerminalLines(-1) })

	testManager := newTestManager(1)

	// 12 completed tasks (exceeds 7 available)
	for i := 0; i < 12; i++ {
		task := testManager.newTask(fmt.Sprintf("done-%d", i))
		task.SetStatus(StatusSuccess)
		task.completed.Store(true)
		task.dirty.Store(true)
		testManager.mu.Lock()
		testManager.tasks = append(testManager.tasks, task)
		testManager.mu.Unlock()
	}

	// 3 pending tasks
	for i := 0; i < 3; i++ {
		task := testManager.newTask(fmt.Sprintf("pending-%d", i))
		task.dirty.Store(true)
		testManager.mu.Lock()
		testManager.tasks = append(testManager.tasks, task)
		testManager.mu.Unlock()
	}

	output := testManager.Pretty()
	rendered := output.String()
	stripped := text.StripANSI(rendered)
	lines := countNonEmptyLines(stripped)

	// 3 completed (first+summary+last) + 3 pending = 6 lines
	if lines != 6 {
		t.Errorf("expected 6 lines (3 collapsed completed + 3 pending), got %d\nOutput:\n%s", lines, stripped)
	}

	if !strings.Contains(stripped, "done-0") {
		t.Errorf("expected first completed task in output, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "done-11") {
		t.Errorf("expected last completed task in output, got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "10 more") {
		t.Errorf("expected '10 more' in output, got:\n%s", stripped)
	}

	close(testManager.shutdown)
}

func TestViewHeightConstraint(t *testing.T) {
	testManager := newTestManager(1)

	// Create 15 completed tasks
	for i := 0; i < 15; i++ {
		taskName := fmt.Sprintf("task-%d", i)
		task := testManager.newTask(taskName)
		task.SetStatus(StatusSuccess)
		task.completed.Store(true)
		task.dirty.Store(true)
		testManager.mu.Lock()
		testManager.tasks = append(testManager.tasks, task)
		testManager.mu.Unlock()
	}

	rendered := testManager.Pretty()
	out := rendered.String()

	// Apply MaxHeight constraint directly (same as interactiveRender)
	constrained := lipgloss.NewStyle().MaxHeight(5).Render(out)
	stripped := text.StripANSI(constrained)
	lines := countNonEmptyLines(stripped)

	if lines > 5 {
		t.Errorf("expected lines <= 5 due to height constraint, got %d\nOutput:\n%s", lines, stripped)
	}

	// Cleanup
	close(testManager.shutdown)
}

func TestPlainRenderOnlyDirtyTasks(t *testing.T) {
	// Save original stderr
	originalStderr := os.Stderr

	// Create pipe for capturing stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	// Redirect stderr to our pipe
	os.Stderr = w

	// Create buffer and copy goroutine
	stderrCapture := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		io.Copy(stderrCapture, r)
		close(done)
	}()

	// Create a fresh manager for this test (render loop already stopped)
	testManager := newTestManager(1)

	// Create 3 tasks, mark only 1 as dirty
	for i := 0; i < 3; i++ {
		taskName := fmt.Sprintf("task-%d", i)
		task := testManager.newTask(taskName)
		task.SetStatus(StatusSuccess)
		task.completed.Store(true)
		if i == 1 {
			task.dirty.Store(true) // Only task-1 is dirty
		} else {
			task.dirty.Store(false)
		}
		testManager.mu.Lock()
		testManager.tasks = append(testManager.tasks, task)
		testManager.mu.Unlock()
	}

	// Plain render should only output dirty tasks
	testManager.PlainRender()

	// Close write end and wait for reader to finish
	w.Close()
	<-done

	// Restore stderr
	os.Stderr = originalStderr

	// Analyze captured output
	output := stderrCapture.String()
	stripped := text.StripANSI(output)
	lines := countNonEmptyLines(stripped)

	// Only 1 dirty task should be rendered
	if lines != 1 {
		t.Errorf("expected 1 line (only dirty task), got %d\nOutput:\n%s", lines, stripped)
	}

	// Verify task-1 is in the output
	if !strings.Contains(stripped, "task-1") {
		t.Errorf("expected 'task-1' in output, got:\n%s", stripped)
	}

	// Cleanup
	close(testManager.shutdown)
}
