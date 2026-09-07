package task

import (
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/text"
	"log/slog"
	"time"
)

// getDuration returns formatted duration string
func (t *Task) getDuration() string {
	if t.status == StatusPending || t.startTime.IsZero() {
		return ""
	}
	// Note: This should be called with mutex already locked
	var end time.Time
	if t.endTime.IsZero() {
		end = time.Now()
	} else {
		end = t.endTime
	}

	return text.HumanizeDuration(end.Sub(t.startTime))
}

// Pretty returns a formatted text representation of the task with its full
// buffered log history.
func (t *Task) Pretty() api.Text {
	t.mu.Lock()
	defer t.mu.Unlock()
	text, _ := t.prettyWithLogOffset(0)
	return text
}

// prettyPlainDelta renders the task line plus only the log entries not yet
// emitted by a previous PlainRender tick, then advances the cursor. The buffer
// itself is preserved for Pretty(), snapshots, and the final tree.
func (t *Task) prettyPlainDelta() api.Text {
	t.mu.Lock()
	defer t.mu.Unlock()
	text, total := t.prettyWithLogOffset(t.plainLogsRendered)
	t.plainLogsRendered = total
	return text
}

// Logger interface implementation methods

// getBufferedLogger ensures the buffered logger is initialized
func (t *Task) getBufferedLogger() *logger.BufferedLogger {
	t.loggerOnce.Do(func() {
		t.bufferedLogger = logger.NewBufferedLogger(1000)
		if t.ctx.Logger != nil {
			t.bufferedLogger.SetLogLevel(t.ctx.Logger.GetLevel())
		}
	})
	return t.bufferedLogger
}

// Tracef logs a trace message (implements Logger interface)
func (t *Task) Tracef(format string, args ...interface{}) {
	t.getBufferedLogger().Tracef(format, args...)
}

// Fatalf logs a fatal message (implements Logger interface)
func (t *Task) Fatalf(format string, args ...interface{}) {
	t.getBufferedLogger().Fatalf(format, args...)
	t.markLogForStreaming()
}

// WithValues returns a logger with additional key-value pairs (implements Logger interface)
func (t *Task) WithValues(keysAndValues ...interface{}) logger.Logger {
	return t.getBufferedLogger().WithValues(keysAndValues...)
}

// IsTraceEnabled checks if trace level is enabled (implements Logger interface)
func (t *Task) IsTraceEnabled() bool {
	return t.getBufferedLogger().IsTraceEnabled()
}

// IsDebugEnabled checks if debug level is enabled (implements Logger interface)
func (t *Task) IsDebugEnabled() bool {
	return t.getBufferedLogger().IsDebugEnabled()
}

// IsLevelEnabled checks if a specific level is enabled (implements Logger interface)
func (t *Task) IsLevelEnabled(level logger.LogLevel) bool {
	return t.getBufferedLogger().IsLevelEnabled(level)
}

// GetLevel returns the current log level (implements Logger interface)
func (t *Task) GetLevel() logger.LogLevel {
	return t.getBufferedLogger().GetLevel()
}

// ClearLogs clears all buffered logs for this task
func (t *Task) ClearLogs() {
	t.getBufferedLogger().ClearLogs()
}

// SetLogLevel sets the log level (implements Logger interface)
func (t *Task) SetLogLevel(level any) {
	t.getBufferedLogger().SetLogLevel(level)
}

// SetMinLogLevel sets the minimum log level (implements Logger interface)
func (t *Task) SetMinLogLevel(level any) {
	t.getBufferedLogger().SetMinLogLevel(level)
}

// V returns a verbose logger (implements Logger interface)
func (t *Task) V(level any) logger.Verbose {
	return t.getBufferedLogger().V(level)
}

// WithV returns a logger with verbosity level (implements Logger interface)
func (t *Task) WithV(level any) logger.Logger {
	return t.getBufferedLogger().WithV(level)
}

// Named returns a named logger (implements Logger interface - noop)
func (t *Task) Named(name string) logger.Logger {
	return t.getBufferedLogger().Named(name)
}

// WithoutName returns a logger without name (implements Logger interface - noop)
func (t *Task) WithoutName() logger.Logger {
	return t.getBufferedLogger().WithoutName()
}

// WithSkipReportLevel returns a logger with skip report level (implements Logger interface - noop)
func (t *Task) WithSkipReportLevel(i int) logger.Logger {
	return t.getBufferedLogger().WithSkipReportLevel(i)
}

// GetSlogLogger returns the slog logger (implements Logger interface - unsupported)
func (t *Task) GetSlogLogger() *slog.Logger {
	return t.getBufferedLogger().GetSlogLogger()
}
