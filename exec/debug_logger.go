package exec

import (
	"log/slog"

	"github.com/flanksource/commons/logger"
)

func NewDebugLogger(log logger.Logger, level logger.LogLevel) logger.Logger {
	return &debugLogger{
		log: log,
		// level to log to the underlying logger
		level: logger.Info,
		// pseudoLevel to control what gets logged
		pseudoLevel: level,
	}
}

type debugLogger struct {
	log         logger.Logger
	level       logger.LogLevel
	pseudoLevel logger.LogLevel
}

// Debugf implements logger.Logger.
func (d *debugLogger) Debugf(format string, args ...interface{}) {
	if d.IsDebugEnabled() {
		d.V(d.level).Infof("V(1): "+format, args...)
	}
}

// Errorf implements logger.Logger.
func (d *debugLogger) Errorf(format string, args ...interface{}) {
	d.V(d.level).Infof(format, args...)

}

// Fatalf implements logger.Logger.
func (d *debugLogger) Fatalf(format string, args ...interface{}) {
	d.V(d.level).Infof(format, args...)

}

// GetLevel implements logger.Logger.
func (d *debugLogger) GetLevel() logger.LogLevel {
	return d.pseudoLevel
}

// GetSlogLogger implements logger.Logger.
func (d *debugLogger) GetSlogLogger() *slog.Logger {
	return nil
}

// Infof implements logger.Logger.
func (d *debugLogger) Infof(format string, args ...interface{}) {
	d.V(d.level).Infof(format, args...)

}

// IsDebugEnabled implements logger.Logger.
func (d *debugLogger) IsDebugEnabled() bool {
	return d.pseudoLevel >= logger.Debug
}

// IsLevelEnabled implements logger.Logger.
func (d *debugLogger) IsLevelEnabled(level logger.LogLevel) bool {
	return d.pseudoLevel >= level
}

// IsTraceEnabled implements logger.Logger.
func (d *debugLogger) IsTraceEnabled() bool {
	return d.pseudoLevel >= logger.Trace
}

// Named implements logger.Logger.
func (d *debugLogger) Named(name string) logger.Logger {
	return d
}

// SetLogLevel implements logger.Logger.
func (d *debugLogger) SetLogLevel(level any) {
	d.pseudoLevel = logger.ParseLevel(d, level)
}

// SetMinLogLevel implements logger.Logger.
func (d *debugLogger) SetMinLogLevel(level any) {
	d.level = logger.ParseLevel(d, level)
}

// Tracef implements logger.Logger.
func (d *debugLogger) Tracef(format string, args ...interface{}) {
	if d.IsTraceEnabled() {
		d.V(d.level).Infof(format, args...)
	}
}

type debugVerbose struct {
	v           logger.Verbose
	level       logger.LogLevel
	actualLevel logger.LogLevel
}

// Enabled implements logger.Verbose.
func (d debugVerbose) Enabled() bool {
	return d.actualLevel >= d.level
}

// Infof implements logger.Verbose.
func (d debugVerbose) Infof(format string, args ...interface{}) {
	if d.Enabled() {
		d.v.Infof(format, args...)
	}
}

// WithValues implements logger.Verbose.
func (d debugVerbose) WithValues(_ ...interface{}) logger.Verbose {
	return d
}

// Always implements logger.Verbose.
func (d debugVerbose) Always() logger.Verbose {
	return d
}

// WithFilter implements logger.Verbose.
func (d debugVerbose) WithFilter(filters ...string) logger.Verbose {
	d.v = d.v.WithFilter(filters...)
	return d
}

// Write implements logger.Verbose.
func (d debugVerbose) Write(p []byte) (n int, err error) {
	if d.Enabled() {
		return d.v.Write(p)
	}
	return 0, nil
}

// V implements logger.Logger.
func (d *debugLogger) V(level any) logger.Verbose {
	return debugVerbose{
		v:           d.log.V(d.level),
		actualLevel: d.pseudoLevel,
		level:       logger.ParseLevel(d, level),
	}
}

// Warnf implements logger.Logger.
func (d *debugLogger) Warnf(format string, args ...interface{}) {
	d.V(d.level).Infof(format, args...)
}

// WithSkipReportLevel implements logger.Logger.
func (d *debugLogger) WithSkipReportLevel(i int) logger.Logger {
	return d
}

// WithV implements logger.Logger.
func (d *debugLogger) WithV(level any) logger.Logger {
	return d
}

// WithValues implements logger.Logger.
func (d *debugLogger) WithValues(keysAndValues ...interface{}) logger.Logger {
	d.log = d.log.WithValues(keysAndValues...)
	return d
}

// WithName implements logger.Logger.
func (d *debugLogger) WithName(name string) logger.Logger {
	d.log = d.Named(name)
	return d
}

// WithoutName implements logger.Logger.
func (d *debugLogger) WithoutName() logger.Logger {
	return d
}
