package task

import (
	"context"
	"errors"
	"time"

	fc "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("task slots", func() {
	var original *Manager
	BeforeEach(func() { original = global; global = newTestManager(1) })
	AfterEach(func() { global.awaitAll(); global.stopRender(); global = original })

	It("allows nested supervisors to await children with one global and group slot", func() {
		group := StartGroup[int]("supervisors", WithConcurrency(1))
		outer := group.Add("outer", func(ctx fc.Context, _ *Task) (int, error) {
			inner := group.Add("inner", func(ctx fc.Context, _ *Task) (int, error) {
				return group.Add("child", func(fc.Context, *Task) (int, error) { return 7, nil }, WithContext(ctx)).GetResult()
			}, WithSlots(0), WithContext(ctx))
			return inner.GetResult()
		}, WithSlots(0), WithTimeout(time.Second), WithCancellationDrain())
		value, err := outer.GetResult()
		Expect(err).NotTo(HaveOccurred())
		Expect(value).To(Equal(7))
	})

	It("starts a supervisor while ordinary capacity is occupied and includes it in waits", func() {
		gate := make(chan struct{})
		defer close(gate)
		started := make(chan struct{})
		StartTask("occupied", func(fc.Context, *Task) (int, error) { close(started); <-gate; return 0, nil })
		Eventually(started).Should(BeClosed())
		supervisor := StartTask("supervisor", func(fc.Context, *Task) (int, error) { return 9, nil }, WithSlots(0))
		Eventually(supervisor.Task.IsOk).Should(BeTrue())
		Expect(global.allTasksCompleted()).To(BeFalse())
	})

	It("rejects invalid reservations", func() {
		Expect(func() { WithSlots(-1) }).To(Panic())
		_, err := StartTask("impossible", func(fc.Context, *Task) (int, error) { return 0, nil }, WithSlots(2)).GetResult()
		Expect(err).To(MatchError(ContainSubstring("slots")))
	})

	It("keeps global waits blocked on a foreground supervisor", func() {
		gate := make(chan struct{})
		started := make(chan struct{})
		run := StartTask("waiting supervisor", func(fc.Context, *Task) (int, error) { close(started); <-gate; return 0, nil }, WithSlots(0))
		Eventually(started).Should(BeClosed())
		done := make(chan struct{})
		go func() { global.awaitAll(); close(done) }()
		Consistently(done, 50*time.Millisecond).ShouldNot(BeClosed())
		close(gate)
		_, err := run.GetResult()
		Expect(err).NotTo(HaveOccurred())
		Eventually(done).Should(BeClosed())
	})

	It("reserves weighted global and group capacity until callbacks return", func() {
		SetMaxConcurrent(3)
		group := StartGroup[int]("weighted", WithConcurrency(2))
		gate, started, next := make(chan struct{}), make(chan struct{}), make(chan struct{})
		first := group.Add("two slots", func(fc.Context, *Task) (int, error) { close(started); <-gate; return 0, nil }, WithSlots(2))
		Eventually(started).Should(BeClosed())
		second := group.Add("one slot", func(fc.Context, *Task) (int, error) { close(next); return 0, nil })
		Consistently(next, 50*time.Millisecond).ShouldNot(BeClosed())
		close(gate)
		_, err := first.GetResult()
		Expect(err).NotTo(HaveOccurred())
		_, err = second.GetResult()
		Expect(err).NotTo(HaveOccurred())
	})

	It("propagates dependency failure to a zero-slot supervisor", func() {
		failed := StartTask("failure", func(fc.Context, *Task) (int, error) { return 0, errors.New("broken") }, WithRetryConfig(RetryConfig{MaxRetries: 0}))
		run := StartTask("dependent", func(fc.Context, *Task) (int, error) { return 1, nil }, WithSlots(0), WithDependencies(failed.Task))
		_, err := run.GetResult()
		Expect(err).To(MatchError("dependency failed"))
	})

	It("drains a cancelled supervisor callback before returning", func() {
		ctx, cancel := context.WithCancel(context.Background())
		started, returned := make(chan struct{}), make(chan struct{})
		run := StartTask("cancel", func(ctx fc.Context, _ *Task) (int, error) {
			close(started)
			<-ctx.Done()
			close(returned)
			return 0, ctx.Err()
		}, WithSlots(0), WithContext(ctx), WithCancellationDrain())
		Eventually(started).Should(BeClosed())
		cancel()
		_, err := run.GetResult()
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		Expect(returned).To(BeClosed())
	})
})
