package task_test

import (
	"context"
	"time"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task timeout", func() {
	const budget = 300 * time.Millisecond

	It("measures the budget from when the task starts, not while it waits on dependencies", func() {
		dependency := task.StartTask("slow dependency", func(commonscontext.Context, *task.Task) (bool, error) {
			time.Sleep(2 * budget)
			return true, nil
		})
		var remaining time.Duration
		dependent := task.StartTask("dependent", func(ctx commonscontext.Context, _ *task.Task) (bool, error) {
			deadline, ok := ctx.Deadline()
			Expect(ok).To(BeTrue(), "a task with a timeout runs under a deadline")
			remaining = time.Until(deadline)
			time.Sleep(budget / 3)
			return true, ctx.Err()
		}, task.WithTimeout(budget), task.WithDependencies(dependency.Task))

		_, err := dependent.GetResult()
		Expect(err).ToNot(HaveOccurred())
		Expect(dependent.Status()).To(Equal(task.StatusSuccess))
		Expect(remaining).To(BeNumerically(">", budget/2), "the deadline is armed when the task starts")
	})

	It("cancels a task that runs past its budget once started", func() {
		// WaitFor can return before the run function does: its ticker cancels
		// a task whose context is done.
		ctxErr := make(chan error, 1)
		started := time.Now()
		overrun := task.StartTask("overrun", func(ctx commonscontext.Context, _ *task.Task) (bool, error) {
			select {
			case <-ctx.Done():
			case <-time.After(10 * budget):
			}
			ctxErr <- ctx.Err()
			return false, ctx.Err()
		}, task.WithTimeout(budget))

		wait := overrun.WaitFor()
		_, resultErr := overrun.GetResult()
		Eventually(ctxErr).Should(Receive(MatchError(context.DeadlineExceeded)))
		Expect(overrun.Status()).To(Equal(task.StatusCancelled))
		Expect(time.Since(started)).To(BeNumerically("<", 5*budget))
		for _, err := range []error{wait.Error, resultErr} {
			Expect(err).To(MatchError(context.DeadlineExceeded), "a self-timeout must not read as success")
			Expect(err).To(MatchError(ContainSubstring(`task "overrun" timed out after 300ms`)))
		}
	})

	It("cuts off a group member that ignores its context once its budget elapses", func() {
		group := task.StartGroup[string]("timeout group", task.WithConcurrency(2))
		release := make(chan struct{})
		defer close(release)
		quick := group.Add("quick", func(commonscontext.Context, *task.Task) (string, error) {
			return "done", nil
		})
		stuck := group.Add("stuck", func(commonscontext.Context, *task.Task) (string, error) {
			<-release
			return "late", nil
		}, task.WithTimeout(budget))

		waited := make(chan *task.WaitResult, 1)
		go func() { waited <- group.WaitFor() }()
		var wait *task.WaitResult
		Eventually(waited).WithTimeout(10 * budget).Should(Receive(&wait))

		Expect(wait.Error).To(MatchError(ContainSubstring(`task "stuck" timed out after 300ms`)))
		Expect(stuck.Status()).To(Equal(task.StatusCancelled))
		_, stuckErr := stuck.GetResult()
		Expect(stuckErr).To(MatchError(context.DeadlineExceeded))
		Expect(quick.GetResult()).To(Equal("done"))
	})

	It("keeps an explicitly cancelled task without drain error-free", func() {
		running := make(chan struct{})
		cancelled := task.StartTask("cancelled", func(ctx commonscontext.Context, _ *task.Task) (bool, error) {
			close(running)
			<-ctx.Done()
			// Returning ctx.Err() here races WaitFor: Cancel signals done
			// before the run function's error is stored.
			return false, nil
		}, task.WithTimeout(10*budget))
		Eventually(running).Should(BeClosed())
		cancelled.Cancel()

		wait := cancelled.WaitFor()
		_, resultErr := cancelled.GetResult()
		Expect([]any{cancelled.Status(), wait.Error, resultErr}).To(Equal([]any{task.StatusCancelled, nil, nil}))
	})
})
