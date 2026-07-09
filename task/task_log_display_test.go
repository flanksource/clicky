package task

import (
	"strings"
	"testing"

	"github.com/flanksource/commons/logger"

	"github.com/flanksource/clicky/text"
)

const logDisplayMessage = "log-display-probe"

// newTaskAtLogLevel creates an unqueued task whose context logger (which
// Pretty() consults for the display filter) and buffered logger (which gates
// Debugf/Tracef at append time) are both set to the given level, without
// touching global logger state.
func newTaskAtLogLevel(tm *Manager, name string, level logger.LogLevel) *Task {
	task := tm.newTask(name)
	ctxLogger := logger.NewBufferedLogger(1)
	ctxLogger.SetLogLevel(level)
	task.ctx.Logger = ctxLogger
	return task
}

func TestTaskLogDisplay(t *testing.T) {
	tests := []struct {
		name        string
		level       logger.LogLevel
		status      Status
		logFn       func(*Task)
		wantVisible bool
	}{
		{
			name:        "failed task renders Infof entry at Info level",
			level:       logger.Info,
			status:      StatusFailed,
			logFn:       func(task *Task) { task.Infof("%s", logDisplayMessage) },
			wantVisible: true,
		},
		{
			name:        "successful task hides Infof entry at Info level",
			level:       logger.Info,
			status:      StatusSuccess,
			logFn:       func(task *Task) { task.Infof("%s", logDisplayMessage) },
			wantVisible: false,
		},
		{
			name:        "successful task renders Debugf entry at Debug level",
			level:       logger.Debug,
			status:      StatusSuccess,
			logFn:       func(task *Task) { task.Debugf("%s", logDisplayMessage) },
			wantVisible: true,
		},
		{
			name:        "successful task renders Infof entry at Debug level",
			level:       logger.Debug,
			status:      StatusSuccess,
			logFn:       func(task *Task) { task.Infof("%s", logDisplayMessage) },
			wantVisible: true,
		},
		{
			name:        "failed task renders Warnf entry at Info level",
			level:       logger.Info,
			status:      StatusFailed,
			logFn:       func(task *Task) { task.Warnf("%s", logDisplayMessage) },
			wantVisible: true,
		},
		{
			name:        "failed task renders Errorf entry at Info level",
			level:       logger.Info,
			status:      StatusFailed,
			logFn:       func(task *Task) { task.Errorf("%s", logDisplayMessage) },
			wantVisible: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := newTestManager(1)
			t.Cleanup(func() { close(tm.shutdown) })

			task := newTaskAtLogLevel(tm, "log-display", tt.level)
			tt.logFn(task)
			task.status = tt.status

			rendered := text.StripANSI(task.Pretty().String())
			if got := strings.Contains(rendered, logDisplayMessage); got != tt.wantVisible {
				t.Errorf("log message visible=%v, want %v at level %d with status %s; rendered:\n%s",
					got, tt.wantVisible, tt.level, tt.status, rendered)
			}
		})
	}
}
