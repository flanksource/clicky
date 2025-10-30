package text

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/flanksource/commons/logger"
)

type loggingFilterWrapper struct {
	logger   logger.Logger
	pipeline []LineProcessor
	mu       sync.Mutex
}

// LoggingFilter wraps a logger.Logger to apply a pipeline of line processors
// to all log messages. Each log message is processed through the pipeline
// in left-to-right order. If any processor returns skip=true, the message
// is not logged.
//
// The returned logger is thread-safe for concurrent use.
//
// Example:
//
//	redactor := clicky.RedactSecrets()
//	log := clicky.LoggingFilter(baseLogger, redactor)
//	log.Infof("password=secret") // Logs redacted output
func LoggingFilter(log logger.Logger, pipeline ...LineProcessor) logger.Logger {
	return &loggingFilterWrapper{
		logger:   log,
		pipeline: pipeline,
	}
}

func (l *loggingFilterWrapper) processMessage(format string, args ...interface{}) (string, bool) {
	msg := fmt.Sprintf(format, args...)
	current := msg

	for _, processor := range l.pipeline {
		var skip bool

		// Recover from panics
		func() {
			defer func() {
				if r := recover(); r != nil {
					skip = true
				}
			}()
			current, skip = processor(current)
		}()

		if skip {
			return current, true
		}
	}
	return current, false
}

func (l *loggingFilterWrapper) Infof(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	processed, skip := l.processMessage(format, args...)
	if !skip {
		l.logger.Infof("%s", processed)
	}
}

func (l *loggingFilterWrapper) Debugf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	processed, skip := l.processMessage(format, args...)
	if !skip {
		l.logger.Debugf("%s", processed)
	}
}

func (l *loggingFilterWrapper) Errorf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	processed, skip := l.processMessage(format, args...)
	if !skip {
		l.logger.Errorf("%s", processed)
	}
}

func (l *loggingFilterWrapper) Warnf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	processed, skip := l.processMessage(format, args...)
	if !skip {
		l.logger.Warnf("%s", processed)
	}
}

func (l *loggingFilterWrapper) Tracef(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	processed, skip := l.processMessage(format, args...)
	if !skip {
		l.logger.Tracef("%s", processed)
	}
}

func (l *loggingFilterWrapper) Fatalf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	processed, skip := l.processMessage(format, args...)
	if !skip {
		l.logger.Fatalf("%s", processed)
	}
}

func (l *loggingFilterWrapper) WithValues(keysAndValues ...interface{}) logger.Logger {
	return &loggingFilterWrapper{
		logger:   l.logger.WithValues(keysAndValues...),
		pipeline: l.pipeline,
	}
}

func (l *loggingFilterWrapper) Named(name string) logger.Logger {
	return &loggingFilterWrapper{
		logger:   l.logger.Named(name),
		pipeline: l.pipeline,
	}
}

func (l *loggingFilterWrapper) WithoutName() logger.Logger {
	return &loggingFilterWrapper{
		logger:   l.logger.WithoutName(),
		pipeline: l.pipeline,
	}
}

func (l *loggingFilterWrapper) WithSkipReportLevel(i int) logger.Logger {
	return &loggingFilterWrapper{
		logger:   l.logger.WithSkipReportLevel(i),
		pipeline: l.pipeline,
	}
}

func (l *loggingFilterWrapper) IsTraceEnabled() bool {
	return l.logger.IsTraceEnabled()
}

func (l *loggingFilterWrapper) IsDebugEnabled() bool {
	return l.logger.IsDebugEnabled()
}

func (l *loggingFilterWrapper) IsLevelEnabled(level logger.LogLevel) bool {
	return l.logger.IsLevelEnabled(level)
}

func (l *loggingFilterWrapper) GetLevel() logger.LogLevel {
	return l.logger.GetLevel()
}

func (l *loggingFilterWrapper) SetLogLevel(level any) {
	l.logger.SetLogLevel(level)
}

func (l *loggingFilterWrapper) SetMinLogLevel(level any) {
	l.logger.SetMinLogLevel(level)
}

func (l *loggingFilterWrapper) V(level any) logger.Verbose {
	return l.logger.V(level)
}

func (l *loggingFilterWrapper) WithV(level any) logger.Logger {
	return &loggingFilterWrapper{
		logger:   l.logger.WithV(level),
		pipeline: l.pipeline,
	}
}

func (l *loggingFilterWrapper) GetSlogLogger() *slog.Logger {
	return l.logger.GetSlogLogger()
}
