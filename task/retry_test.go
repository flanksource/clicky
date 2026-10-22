package task_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	errInputsChanged = errors.New("inputs changed while indexing")
	errTimeoutText   = errors.New("connection timeout")
	errStopped       = errors.New("caller stopped")
)

var _ = Describe("Retry", func() {
	const (
		maxRetries = 2
		tinyDelay  = time.Millisecond
	)
	isInputsChanged := func(err error) bool { return errors.Is(err, errInputsChanged) }
	fastConfig := func() task.RetryConfig {
		return task.RetryConfig{Retryable: isInputsChanged, MaxRetries: maxRetries, BaseDelay: tinyDelay, BackoffFactor: 1}
	}

	It("lets the Retryable predicate decide over matching RetryableErrors patterns", func() {
		config := fastConfig()
		config.RetryableErrors = []string{"timeout"}
		calls := 0
		err := task.Retry(context.Background(), config, func(context.Context, int) error {
			calls++
			return errTimeoutText
		})
		Expect(err).To(Equal(errTimeoutText), "a non-retryable error is returned as-is")
		Expect(calls).To(Equal(1), "a pattern match does not retry when the predicate says no")
	})

	It("retries a wrapped sentinel until MaxRetries is exhausted, preserving errors.Is", func() {
		var attempts []int
		err := task.Retry(context.Background(), fastConfig(), func(_ context.Context, attempt int) error {
			attempts = append(attempts, attempt)
			return fmt.Errorf("index module: %w", errInputsChanged)
		})
		Expect(err).To(MatchError(errInputsChanged))
		Expect(err.Error()).To(HavePrefix(fmt.Sprintf("failed after %d attempts", maxRetries+1)))
		Expect(attempts).To(Equal([]int{1, 2, 3}), "attempts are 1-based and total MaxRetries+1")
	})

	It("returns nil once a retried attempt succeeds", func() {
		calls := 0
		err := task.Retry(context.Background(), fastConfig(), func(context.Context, int) error {
			calls++
			if calls == 1 {
				return errInputsChanged
			}
			return nil
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(calls).To(Equal(2))
	})

	It("returns the context cause when cancelled while waiting to retry", func() {
		config := fastConfig()
		config.BaseDelay = time.Hour
		ctx, cancel := context.WithCancelCause(context.Background())
		calls := 0
		err := task.Retry(ctx, config, func(context.Context, int) error {
			calls++
			cancel(errStopped)
			return errInputsChanged
		})
		Expect(err).To(MatchError(errStopped))
		Expect(calls).To(Equal(1))
	})

	It("retries a task whose func fails once with a wrapped sentinel, then succeeds", func() {
		var runs atomic.Int32
		handle := task.StartTask("flaky index", func(commonscontext.Context, *task.Task) (bool, error) {
			if runs.Add(1) == 1 {
				return false, fmt.Errorf("index: %w", errInputsChanged)
			}
			return true, nil
		}, task.WithRetryConfig(fastConfig()))
		ok, err := handle.GetResult()
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue())
		Expect(handle.Status()).To(Equal(task.StatusSuccess))
		Expect(runs.Load()).To(Equal(int32(2)))
	})
})

var _ = Describe("RetryConfig.Delay", func() {
	const (
		base    = 100 * time.Millisecond
		maxWait = 350 * time.Millisecond
		factor  = 2.0
		jitter  = 0.1
		samples = 200
	)
	config := task.RetryConfig{BaseDelay: base, MaxDelay: maxWait, BackoffFactor: factor, JitterFactor: jitter}

	DescribeTable("stays within ±JitterFactor of the capped exponential delay",
		func(attempt int, nominal time.Duration) {
			low, high := time.Duration(float64(nominal)*(1-jitter)), time.Duration(float64(nominal)*(1+jitter))
			seen := map[time.Duration]bool{}
			for range samples {
				delay := config.Delay(attempt)
				Expect(delay).To(BeNumerically(">=", low))
				Expect(delay).To(BeNumerically("<=", high))
				seen[delay] = true
			}
			Expect(len(seen)).To(BeNumerically(">", 1), "jitter spreads the delay")
		},
		Entry("first attempt uses BaseDelay", 1, base),
		Entry("second attempt doubles", 2, 2*base),
		Entry("third attempt is capped at MaxDelay", 3, maxWait),
	)
})
