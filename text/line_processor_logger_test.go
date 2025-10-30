package text_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/flanksource/clicky/text"
	"github.com/flanksource/commons/logger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Simple test logger that writes to a buffer
type testLogger struct {
	buf *bytes.Buffer
	mu  sync.Mutex
}

func newTestLogger(buf *bytes.Buffer) logger.Logger {
	return &testLogger{buf: buf}
}

func (t *testLogger) write(format string, args ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(t.buf, format+"\n", args...)
}

func (t *testLogger) Infof(format string, args ...interface{}) {
	t.write("INFO: "+format, args...)
}

func (t *testLogger) Debugf(format string, args ...interface{}) {
	t.write("DEBUG: "+format, args...)
}

func (t *testLogger) Errorf(format string, args ...interface{}) {
	t.write("ERROR: "+format, args...)
}

func (t *testLogger) Warnf(format string, args ...interface{}) {
	t.write("WARN: "+format, args...)
}

func (t *testLogger) Tracef(format string, args ...interface{}) {
	t.write("TRACE: "+format, args...)
}

func (t *testLogger) Fatalf(format string, args ...interface{}) {
	t.write("FATAL: "+format, args...)
}

func (t *testLogger) WithValues(keysAndValues ...interface{}) logger.Logger {
	return &testLogger{buf: t.buf}
}

func (t *testLogger) Named(name string) logger.Logger {
	return &testLogger{buf: t.buf}
}

func (t *testLogger) WithoutName() logger.Logger {
	return &testLogger{buf: t.buf}
}

func (t *testLogger) WithSkipReportLevel(i int) logger.Logger {
	return &testLogger{buf: t.buf}
}

func (t *testLogger) IsTraceEnabled() bool {
	return true
}

func (t *testLogger) IsDebugEnabled() bool {
	return true
}

func (t *testLogger) IsLevelEnabled(level logger.LogLevel) bool {
	return true
}

func (t *testLogger) GetLevel() logger.LogLevel {
	return logger.Info
}

func (t *testLogger) SetLogLevel(level any) {}

func (t *testLogger) SetMinLogLevel(level any) {}

func (t *testLogger) V(level any) logger.Verbose {
	return nil
}

func (t *testLogger) WithV(level any) logger.Logger {
	return &testLogger{buf: t.buf}
}

func (t *testLogger) GetSlogLogger() *slog.Logger {
	return nil
}

var _ = Describe("LoggingFilter", func() {
	var (
		buf       *bytes.Buffer
		baseLog   logger.Logger
		uppercase func(string) (string, bool)
		skipAll   func(string) (string, bool)
	)

	BeforeEach(func() {
		buf = &bytes.Buffer{}
		baseLog = newTestLogger(buf)

		uppercase = func(line string) (string, bool) {
			return strings.ToUpper(line), false
		}

		skipAll = func(line string) (string, bool) {
			return line, true
		}
	})

	Context("with single processor", func() {
		It("should process Infof messages", func() {
			log := text.LoggingFilter(baseLog, uppercase)
			log.Infof("hello world")

			Eventually(buf.String).Should(ContainSubstring("HELLO WORLD"))
		})

		It("should process Debugf messages", func() {
			log := text.LoggingFilter(baseLog, uppercase)
			log.Debugf("debug message")

			Eventually(buf.String).Should(ContainSubstring("DEBUG MESSAGE"))
		})

		It("should process Errorf messages", func() {
			log := text.LoggingFilter(baseLog, uppercase)
			log.Errorf("error message")

			Eventually(buf.String).Should(ContainSubstring("ERROR MESSAGE"))
		})

		It("should process Warnf messages", func() {
			log := text.LoggingFilter(baseLog, uppercase)
			log.Warnf("warn message")

			Eventually(buf.String).Should(ContainSubstring("WARN MESSAGE"))
		})

		It("should process Tracef messages", func() {
			log := text.LoggingFilter(baseLog, uppercase)
			log.Tracef("trace message")

			Eventually(buf.String).Should(ContainSubstring("TRACE MESSAGE"))
		})

		It("should skip messages when processor returns skip=true", func() {
			log := text.LoggingFilter(baseLog, skipAll)
			log.Infof("should be skipped")

			Consistently(buf.String).Should(BeEmpty())
		})
	})

	Context("with multiple processors", func() {
		It("should execute processors left-to-right", func() {
			addPrefix := func(line string) (string, bool) {
				return "[PREFIX] " + line, false
			}

			addSuffix := func(line string) (string, bool) {
				return line + " [SUFFIX]", false
			}

			log := text.LoggingFilter(baseLog, addPrefix, addSuffix)
			log.Infof("test")

			Eventually(buf.String).Should(ContainSubstring("[PREFIX] test [SUFFIX]"))
		})

		It("should short-circuit on skip", func() {
			neverCalled := func(line string) (string, bool) {
				Fail("Second processor should not be called")
				return line, false
			}

			log := text.LoggingFilter(baseLog, skipAll, neverCalled)
			log.Infof("test")

			Consistently(buf.String).Should(BeEmpty())
		})
	})

	Context("with concurrent logging", func() {
		It("should be thread-safe", func() {
			log := text.LoggingFilter(baseLog, uppercase)

			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					log.Infof("message")
				}()
			}

			wg.Wait()

			// Should have 10 messages
			count := strings.Count(buf.String(), "MESSAGE")
			Expect(count).To(Equal(10))
		})
	})

	Context("with panic recovery", func() {
		It("should recover from panic and skip message", func() {
			panicProcessor := func(line string) (string, bool) {
				panic("test panic")
			}

			log := text.LoggingFilter(baseLog, panicProcessor)
			log.Infof("test")

			Consistently(buf.String).Should(BeEmpty())
		})

		It("should continue logging after panic", func() {
			panicOnFirst := func(line string) (string, bool) {
				if strings.Contains(line, "panic") {
					panic("test panic")
				}
				return line, false
			}

			log := text.LoggingFilter(baseLog, panicOnFirst)
			log.Infof("panic this")
			log.Infof("normal message")

			Eventually(buf.String).Should(ContainSubstring("normal message"))
			Expect(buf.String()).ToNot(ContainSubstring("panic this"))
		})
	})

	Context("WithValues", func() {
		It("should return a new logger", func() {
			log := text.LoggingFilter(baseLog, uppercase)
			contextLog := log.WithValues("key", "value")
			Expect(contextLog).ToNot(BeNil())

			contextLog.Infof("test")
			Eventually(buf.String).Should(ContainSubstring("TEST"))
		})
	})

	Context("Named", func() {
		It("should return a new logger", func() {
			log := text.LoggingFilter(baseLog, uppercase)
			namedLog := log.Named("mycomponent")
			Expect(namedLog).ToNot(BeNil())

			namedLog.Infof("test")
			Eventually(buf.String).Should(ContainSubstring("TEST"))
		})
	})
})
