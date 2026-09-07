package task

import (
	"fmt"
	"reflect"
	"time"

	"github.com/flanksource/commons/logger"
	"golang.org/x/sync/semaphore"
)

const waitForWarnAfter = 30 * time.Second

// WaitFor waits for completion. Cancellation notifies legacy waiters immediately;
// WithCancellationDrain opts into waiting until the callback has returned.
func (t *Task) WaitFor() *WaitResult {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	start, warnAfter := time.Now(), waitForWarnAfter
	wait := t.doneChan
	if t.drainCancellation {
		wait = t.drainedChan
	}
	for {
		select {
		case <-wait:
			return t.waitResult()
		case <-ticker.C:
			t.mu.Lock()
			cancelled := t.ctx.Err() != nil
			t.mu.Unlock()
			if cancelled {
				t.Cancel()
			}
			if waited := time.Since(start); waited >= warnAfter {
				logger.Warnf("Still waiting for task %q (%s) after %s", t.Name(), t.Status(), waited.Round(time.Second))
				warnAfter *= 2
			}
		}
	}
}

func (t *Task) waitResult() *WaitResult {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := &WaitResult{Status: t.status, Error: t.resultError(), TaskCount: 1}
	if !t.enqueuedAt.IsZero() {
		result.Duration = t.endTime.Sub(t.startTime)
	}
	switch t.status {
	case StatusSuccess, StatusPASS, StatusSKIP:
		result.SuccessCount = 1
	case StatusFailed, StatusFAIL, StatusERR:
		result.FailureCount = 1
	case StatusWarning:
		result.WarningCount = 1
	}
	return result
}

func (t *Task) resultError() error {
	if t.err != nil {
		return t.err
	}
	if t.drainCancellation && t.status == StatusCancelled {
		return t.cancelErr
	}
	return nil
}

// GetResult returns the stored result and error
func (t *Task) GetResult() (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.result, t.resultError()
}

// SetResult stores a result in the task
func (t *Task) SetResult(result interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.result = result
	if result != nil {
		t.resultType = reflect.TypeOf(result)
	}
}

// GetTypedResult retrieves the result with type assertion
func (t *Task) GetTypedResult(target interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.result == nil {
		return t.resultError()
	}

	// Use reflection to set the target value
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	resultValue := reflect.ValueOf(t.result)
	targetElement := targetValue.Elem()

	if !resultValue.Type().AssignableTo(targetElement.Type()) {
		return fmt.Errorf("result type %T cannot be assigned to target type %T", t.result, target)
	}

	targetElement.Set(resultValue)
	return t.resultError()
}

// Duration returns the task duration
func (t *Task) Duration() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == StatusPending || t.enqueuedAt.IsZero() {
		return 0
	}

	endTime := t.endTime
	if t.status == StatusRunning {
		endTime = time.Now()
	}

	return endTime.Sub(t.startTime)
}

// EndTime returns when the task reached a terminal state, or the zero time if
// it is still pending/running.
func (t *Task) EndTime() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.endTime
}

// IsGroup returns false for Task
func (t *Task) IsGroup() bool {
	return false
}

// groupSem returns the concurrency semaphore of the task's parent group, or nil
// if the task is ungrouped or its group has no concurrency limit. Group.sem is
// assigned once in StartGroup before any task is added and never mutated, so no
// lock is needed.
func (t *Task) groupSem() *semaphore.Weighted {
	if t.parent == nil {
		return nil
	}
	return t.parent.sem
}
