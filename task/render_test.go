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
	"github.com/flanksource/clicky/api/tailwind"
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

func TestPretty_TruncatesLongPrompt(t *testing.T) {
	originalWidthFunc := tailwind.TerminalWidthFunc
	tailwind.TerminalWidthFunc = func() int { return 120 }
	t.Cleanup(func() { tailwind.TerminalWidthFunc = originalWidthFunc })

	testManager := newTestManager(1)
	t.Cleanup(func() { close(testManager.shutdown) })

	longPrompt := strings.Repeat("x", 5000)
	task := testManager.newTask("short-name", WithPrompt(longPrompt))
	task.status = StatusSuccess

	check := func(label, rendered string) {
		stripped := text.StripANSI(rendered)
		runeLen := len([]rune(stripped))
		if runeLen > 150 {
			t.Errorf("%s: rendered length %d exceeds 150 runes; output: %q", label, runeLen, stripped)
		}
		if !strings.ContainsRune(stripped, '…') {
			t.Errorf("%s: expected ellipsis in truncated output; got: %q", label, stripped)
		}
	}

	pretty := task.Pretty()
	check("ANSI", pretty.ANSI())
	check("String", pretty.String())
}

func TestStatusApply_PreservesIncomingStyle(t *testing.T) {
	originalWidthFunc := tailwind.TerminalWidthFunc
	tailwind.TerminalWidthFunc = func() int { return 120 }
	t.Cleanup(func() { tailwind.TerminalWidthFunc = originalWidthFunc })

	in := api.Text{Content: "helloworldhello", Style: "max-w-[10] truncate-suffix"}
	out := StatusSuccess.Apply(in)

	classes := map[string]struct{}{}
	for _, c := range strings.Fields(out.Style) {
		classes[c] = struct{}{}
	}
	for _, expected := range []string{"max-w-[10]", "truncate-suffix"} {
		if _, ok := classes[expected]; !ok {
			t.Errorf("expected style to retain %q; got %q", expected, out.Style)
		}
	}
	statusStyle := StatusSuccess.Style()
	if statusStyle != "" {
		if _, ok := classes[statusStyle]; !ok {
			t.Errorf("expected status style %q to be appended; got %q", statusStyle, out.Style)
		}
	}

	stripped := text.StripANSI(out.ANSI())
	if len([]rune(stripped)) > 12 {
		t.Errorf("expected visible length <= 12 after truncation, got %d: %q", len([]rune(stripped)), stripped)
	}
	if !strings.ContainsRune(stripped, '…') {
		t.Errorf("expected ellipsis in truncated ANSI output; got: %q", stripped)
	}

	twice := StatusSuccess.Apply(out)
	classCounts := map[string]int{}
	for _, c := range strings.Fields(twice.Style) {
		classCounts[c]++
	}
	for cls, n := range classCounts {
		if n > 1 {
			t.Errorf("class %q duplicated after repeated Apply: %q", cls, twice.Style)
		}
	}
}

// Failed and warning tasks must never get collapsed into the "... N more"
// summary, even when the success bucket pushes the pane over the terminal
// height budget. Losing a failure behind a summary defeats the whole point
// of the pane.
func TestRenderLineCount_ProblemsAlwaysVisible(t *testing.T) {
	api.SetTerminalLines(10)
	t.Cleanup(func() { api.SetTerminalLines(-1) })

	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })

	addTask := func(name string, status Status) {
		task := tm.newTask(name)
		task.SetStatus(status)
		if status != StatusPending && status != StatusRunning {
			task.completed.Store(true)
		}
		task.dirty.Store(true)
		tm.mu.Lock()
		tm.tasks = append(tm.tasks, task)
		tm.mu.Unlock()
	}

	// 20 successes would normally collapse to first + summary + last.
	for i := 0; i < 20; i++ {
		addTask(fmt.Sprintf("success-%d", i), StatusSuccess)
	}
	addTask("failed-A", StatusFailed)
	addTask("failed-B", StatusFailed)
	addTask("warn-C", StatusWarning)

	stripped := text.StripANSI(tm.Pretty().String())
	for _, want := range []string{"failed-A", "failed-B", "warn-C"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("problem task %q must appear in rendered pane; output:\n%s", want, stripped)
		}
	}
	// Success bucket should still collapse when it overflows.
	if !strings.Contains(stripped, "more") {
		t.Errorf("success bucket should collapse when it overflows; output:\n%s", stripped)
	}
}

// Running tasks never get collapsed either — the user cares about what's
// in flight right now.
func TestRenderLineCount_RunningAlwaysVisible(t *testing.T) {
	api.SetTerminalLines(8)
	t.Cleanup(func() { api.SetTerminalLines(-1) })

	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })

	addTask := func(name string, status Status) {
		task := tm.newTask(name)
		task.SetStatus(status)
		if status != StatusPending && status != StatusRunning {
			task.completed.Store(true)
		}
		task.dirty.Store(true)
		tm.mu.Lock()
		tm.tasks = append(tm.tasks, task)
		tm.mu.Unlock()
	}

	for i := 0; i < 15; i++ {
		addTask(fmt.Sprintf("success-%d", i), StatusSuccess)
	}
	addTask("running-A", StatusRunning)
	addTask("running-B", StatusRunning)

	stripped := text.StripANSI(tm.Pretty().String())
	for _, want := range []string{"running-A", "running-B"} {
		if !strings.Contains(stripped, want) {
			t.Errorf("running task %q must appear in rendered pane; output:\n%s", want, stripped)
		}
	}
}

// Problems sort before running, which sort before successes. Verifies the
// pane ordering matches the priority contract.
func TestRenderOrder_ProblemsThenRunningThenSuccess(t *testing.T) {
	api.SetTerminalLines(40)
	t.Cleanup(func() { api.SetTerminalLines(-1) })

	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })

	addTask := func(name string, status Status) {
		task := tm.newTask(name)
		task.SetStatus(status)
		if status != StatusPending && status != StatusRunning {
			task.completed.Store(true)
		}
		task.dirty.Store(true)
		tm.mu.Lock()
		tm.tasks = append(tm.tasks, task)
		tm.mu.Unlock()
	}

	// Insertion order intentionally mixes the buckets so we verify the
	// render re-groups rather than keeping raw insertion order.
	addTask("success-early", StatusSuccess)
	addTask("running-mid", StatusRunning)
	addTask("failed-late", StatusFailed)

	stripped := text.StripANSI(tm.Pretty().String())
	idxFailed := strings.Index(stripped, "failed-late")
	idxRunning := strings.Index(stripped, "running-mid")
	idxSuccess := strings.Index(stripped, "success-early")
	if idxFailed < 0 || idxRunning < 0 || idxSuccess < 0 {
		t.Fatalf("all three tasks must appear; output:\n%s", stripped)
	}
	if !(idxFailed < idxRunning && idxRunning < idxSuccess) {
		t.Errorf("expected order failed < running < success, got failed=%d running=%d success=%d\noutput:\n%s",
			idxFailed, idxRunning, idxSuccess, stripped)
	}
}

func TestPhysicalRows_AccountsForSoftWrap(t *testing.T) {
	const width = 40
	cases := []struct {
		name string
		out  string
		want int
	}{
		{"empty", "", 1},
		{"single short line", "hello", 1},
		{"three logical lines no wrap", "a\nb\nc", 3},
		{"exactly width does not wrap", strings.Repeat("x", width), 1},
		{"one over width wraps to two", strings.Repeat("x", width+1), 2},
		{"long line wraps to three", strings.Repeat("x", 2*width+1), 3},
		{"wrapped line plus short line", strings.Repeat("x", width+5) + "\nshort", 3},
		{"ansi codes do not inflate width", "\x1b[31m" + strings.Repeat("x", width) + "\x1b[0m", 1},
		{"blank logical lines each count once", "\n\n", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := physicalRows(tc.out, width); got != tc.want {
				t.Errorf("physicalRows(%q, %d) = %d, want %d", tc.out, width, got, tc.want)
			}
		})
	}
}

func TestPhysicalRows_NonWrappingMatchesNewlineCount(t *testing.T) {
	// Backward-compat guard: when no line exceeds the width, the returned
	// ClearLines count (physicalRows-1) must equal the old strings.Count(\n).
	out := "line one\nline two\nline three"
	if got, want := physicalRows(out, 120)-1, strings.Count(out, "\n"); got != want {
		t.Errorf("non-wrapping count drifted from newline count: got %d want %d", got, want)
	}
}
