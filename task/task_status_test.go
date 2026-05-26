package task

import (
	"testing"
	"time"
)

// Regression: StatusWarning must be terminal — endTime is frozen so the
// rendered duration stops ticking and the task context is cancelled.
// Before this fix, Warning() left endTime zero, so getDuration() kept
// returning time.Now()-startTime on every render, producing an ever-growing
// elapsed counter even after the task's goroutine had returned.
func TestSetStatus_TerminalStatesFreezeEndTime(t *testing.T) {
	cases := []struct {
		name   string
		mark   func(*Task)
		status Status
	}{
		{"Success", func(tk *Task) { tk.Success() }, StatusSuccess},
		{"Failed", func(tk *Task) { tk.Failed() }, StatusFailed},
		{"Warning", func(tk *Task) { tk.Warning() }, StatusWarning},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tm := newTestManager(1)
			t.Cleanup(func() { close(tm.shutdown) })

			tk := tm.newTask(tc.name + "-task")
			tk.SetStatus(StatusRunning)

			cancelCh := tk.ctx.Done()

			tc.mark(tk)

			tk.mu.Lock()
			end := tk.endTime
			tk.mu.Unlock()
			if end.IsZero() {
				t.Fatalf("%s: endTime is zero — task not marked terminal", tc.name)
			}

			select {
			case <-cancelCh:
			default:
				t.Fatalf("%s: task ctx not cancelled — SetStatus skipped the terminal cleanup branch", tc.name)
			}

			d1 := tk.WaitTime()
			time.Sleep(20 * time.Millisecond)
			d2 := tk.WaitTime()
			if d1 != d2 {
				t.Fatalf("%s: WaitTime kept ticking after terminal status: %v -> %v", tc.name, d1, d2)
			}

			tk.mu.Lock()
			rendered1 := tk.getDuration()
			tk.mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			tk.mu.Lock()
			rendered2 := tk.getDuration()
			tk.mu.Unlock()
			if rendered1 != rendered2 {
				t.Fatalf("%s: getDuration() kept ticking after terminal status: %q -> %q", tc.name, rendered1, rendered2)
			}
		})
	}
}
