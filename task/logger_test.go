package task

import (
	"testing"

	"github.com/flanksource/commons/logger"
)

func TestTaskImplementsLoggerInterface(t *testing.T) {
	task := &Task{}

	// Verify that *Task implements logger.Logger interface
	var _ logger.Logger = task

	// Test that we can call logger methods
	task.Debugf("debug message: %s", "test")
	task.Infof("info message: %d", 42)
	task.Warnf("warn message")
	task.Errorf("error message")
	task.Tracef("trace message")

	// Test logger interface methods
	if !task.IsLevelEnabled(logger.Info) {
		t.Error("Expected Info level to be enabled by default")
	}

	task.SetLogLevel(logger.Debug)
	if !task.IsDebugEnabled() {
		t.Error("Expected debug to be enabled after setting log level")
	}

	// Test WithValues (should return a Logger)
	loggerWithValues := task.WithValues("key", "value")
	if loggerWithValues == nil {
		t.Error("WithValues should return a Logger")
	}

	// Test V() returns Verbose
	verbose := task.V(logger.Debug)
	if verbose == nil {
		t.Error("V() should return a Verbose logger")
	}

	// Test verbose with filter
	verboseWithFilter := verbose.WithFilter("filtered")
	if verboseWithFilter == nil {
		t.Error("WithFilter should return a Verbose logger")
	}

	// Test that logs are captured in buffered logger
	logs := task.getBufferedLogger().GetLogs()
	if len(logs) == 0 {
		t.Error("Expected buffered logger to capture logs")
	}

	t.Logf("Captured %d buffered logs", len(logs))
}

func TestBufferedLoggerWithFilter(t *testing.T) {
	task := &Task{}

	// Get verbose logger with filter
	verbose := task.V(logger.Debug).WithFilter("skip_me")

	// This should be filtered out
	verbose.Infof("This message contains skip_me and should be filtered")

	// This should not be filtered
	verbose.Infof("This message should appear")

	logs := task.getBufferedLogger().GetLogs()
	t.Logf("Captured %d logs after filtering test", len(logs))

	// The filtered message should not appear in logs
	for _, log := range logs {
		if log.Message == "This message contains skip_me and should be filtered" {
			t.Error("Filtered message should not appear in logs")
		}
	}
}
