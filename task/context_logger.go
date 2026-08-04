package task

import "github.com/flanksource/commons/logger"

// taskContextLogger is installed as the Logger on a task's flanksource
// context, so lines logged through ctx.Logger land in the task's buffered
// logger and render under the task's line in the tree instead of
// interleaving with global logger output. Emits go through the Task's log
// methods, which also mark the task dirty at streaming levels so plain
// mode prints them promptly. Everything else — level queries, verbosity
// checks, derived loggers — comes from the embedded buffered logger;
// loggers derived via Named/WithValues still write to the buffer but skip
// the dirty marking.
type taskContextLogger struct {
	logger.Logger
	task *Task
}

func (l taskContextLogger) Tracef(format string, args ...interface{}) {
	l.task.Tracef(format, args...)
}

func (l taskContextLogger) Debugf(format string, args ...interface{}) {
	l.task.Debugf(format, args...)
}

func (l taskContextLogger) Infof(format string, args ...interface{}) {
	l.task.Infof(format, args...)
}

func (l taskContextLogger) Warnf(format string, args ...interface{}) {
	l.task.Warnf(format, args...)
}

func (l taskContextLogger) Errorf(format string, args ...interface{}) {
	l.task.Errorf(format, args...)
}

func (l taskContextLogger) Fatalf(format string, args ...interface{}) {
	l.task.Fatalf(format, args...)
}

// contextLogger returns the Logger to install on the task's context. It
// initializes the buffered logger first (while ctx.Logger still points at
// the previous logger, whose level seeds the buffer's level).
func (t *Task) contextLogger() logger.Logger {
	return taskContextLogger{Logger: t.getBufferedLogger(), task: t}
}
