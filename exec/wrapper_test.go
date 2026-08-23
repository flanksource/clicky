package exec

import (
	"context"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Wrapper", func() {
	Describe("Basic Execution", func() {
		It("should execute simple commands successfully", func() {
			echo := NewExec("echo").AsWrapper()

			result, err := echo("hello", "world")
			Expect(err).ToNot(HaveOccurred())
			Expect(result.ExitCode).To(Equal(0))

			output := strings.TrimSpace(result.Stdout)
			Expect(output).To(Equal("hello world"))
			Expect(result.Command).ToNot(BeEmpty())
		})

		It("should handle multiple arguments with base command", func() {
			wrapper := NewExec("echo", "base").AsWrapper()

			result, err := wrapper("additional", "args")
			Expect(err).ToNot(HaveOccurred())

			output := strings.TrimSpace(result.Stdout)
			Expect(output).To(Equal("base additional args"))
		})

		It("should convert non-string argument types", func() {
			wrapper := NewExec("echo").AsWrapper()

			result, err := wrapper(123)
			Expect(err).ToNot(HaveOccurred())

			output := strings.TrimSpace(result.Stdout)
			Expect(output).To(Equal("123"))
		})
	})

	Describe("Functional Options", func() {
		Context("when using timeout option", func() {
			It("should timeout long-running commands", func() {
				sleep := NewExec("sleep").AsWrapper()

				start := time.Now()
				result, err := sleep("10", WithTimeout(1*time.Second))
				duration := time.Since(start)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("timed out"))
				Expect(duration).To(BeNumerically("<", 10*time.Second))
				Expect(result).ToNot(BeNil())
			})
		})

		Context("when using context option", func() {
			It("should cancel on context timeout", func() {
				sleep := NewExec("sleep").AsWrapper()

				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				start := time.Now()
				_, err := sleep("10", WithContext(ctx))
				duration := time.Since(start)

				Expect(err).To(MatchError(context.DeadlineExceeded))
				Expect(duration).To(BeNumerically("<", 10*time.Second))
			})

			It("should not start the command when the context is already canceled", func() {
				sleep := NewExec("sleep").AsWrapper()

				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
				cancel()

				start := time.Now()
				result, err := sleep("10", WithContext(ctx))

				Expect(err).To(MatchError(context.Canceled))
				Expect(time.Since(start)).To(BeNumerically("<", time.Second))
				Expect(result.PID).To(BeZero(), "the command must never have started")
				Expect(result.IsPending()).To(BeFalse())
			})

			It("should fail fast on an already-expired deadline instead of waiting without a timeout", func() {
				sleep := NewExec("sleep").AsWrapper()

				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()

				start := time.Now()
				result, err := sleep("10", WithContext(ctx))

				Expect(err).To(MatchError(context.DeadlineExceeded))
				Expect(time.Since(start)).To(BeNumerically("<", time.Second))
				Expect(result.PID).To(BeZero(), "the command must never have started")
				Expect(result.IsPending()).To(BeFalse())
			})
		})

		Context("when using directory option", func() {
			It("should execute command in specified directory", func() {
				wrapper := NewExec("pwd").AsWrapper()

				result, err := wrapper(WithDir("/tmp"))
				Expect(err).ToNot(HaveOccurred())

				output := strings.TrimSpace(result.Stdout)
				Expect(output).To(SatisfyAny(Equal("/tmp"), Equal("/private/tmp")))
			})
		})

		Context("when using environment variables", func() {
			It("should preserve template configuration across calls", func() {
				template := Process{
					Cmd:     "printenv",
					Env:     map[string]string{"TEST_VAR": "original"},
					Timeout: 5 * time.Second,
				}

				printenv := template.AsWrapper()

				By("using original environment variable")
				result1, err := printenv("TEST_VAR")
				Expect(err).ToNot(HaveOccurred())
				Expect(result1.Stdout).To(ContainSubstring("original"))

				By("overriding environment variable for single call")
				result2, err := printenv("TEST_VAR", WithEnv("TEST_VAR", "modified"))
				Expect(err).ToNot(HaveOccurred())
				Expect(result2.Stdout).To(ContainSubstring("modified"))

				By("verifying template remains unchanged")
				result3, err := printenv("TEST_VAR")
				Expect(err).ToNot(HaveOccurred())
				Expect(result3.Stdout).To(ContainSubstring("original"))
			})
		})
	})

	Describe("Error Handling", func() {
		Context("when command exits with non-zero code", func() {
			var sh func(args ...any) (*ExecResult, error)

			BeforeEach(func() {
				sh = NewExec("sh", "-c").AsWrapper()
			})

			It("should return error by default", func() {
				result, err := sh("exit 42")
				Expect(err).To(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.ExitCode).To(Equal(42))
			})

			It("should not return error with WithoutErrorOnNonZero option", func() {
				result, err := sh("exit 42", WithoutErrorOnNonZero())
				Expect(err).ToNot(HaveOccurred())
				Expect(result.ExitCode).To(Equal(42))
			})
		})
	})

	Describe("Concurrency", func() {
		It("should handle concurrent wrapper calls safely", func() {
			date := NewExec("date", "+%s.%N").AsWrapper()

			const numGoroutines = 10
			var wg sync.WaitGroup
			results := make([]*ExecResult, numGoroutines)
			errors := make([]error, numGoroutines)

			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					result, err := date()
					results[idx] = result
					errors[idx] = err
				}(i)
			}

			wg.Wait()

			outputs := make(map[string]bool)
			for i := 0; i < numGoroutines; i++ {
				Expect(errors[i]).ToNot(HaveOccurred(), "Goroutine %d should not error", i)
				Expect(results[i]).ToNot(BeNil(), "Goroutine %d should have result", i)
				Expect(results[i].ExitCode).To(Equal(0), "Goroutine %d should have exit code 0", i)

				output := strings.TrimSpace(results[i].Stdout)
				outputs[output] = true
			}

			Expect(len(outputs)).To(BeNumerically(">=", 2), "Should have multiple different timestamps")
		})
	})

	Describe("Result Metadata", func() {
		It("should populate result metadata fields", func() {
			wrapper := NewExec("echo").AsWrapper()

			result, err := wrapper("test")
			Expect(err).ToNot(HaveOccurred())
			Expect(result.Duration).To(BeNumerically(">", 0))
			Expect(result.PID).To(BeNumerically(">", 0))
			Expect(result.Command).ToNot(BeEmpty())
			Expect(result.Command).To(ContainSubstring("echo"))
		})
	})
})
