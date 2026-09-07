package task_test

import (
	"context"
	"errors"
	"time"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cancellation drain", func() {
	It("never invokes work added after its group was canceled", func() {
		group := task.StartGroup[bool]("canceled owner")
		group.Cancel()
		handle := group.Add("must not start", func(commonscontext.Context, *task.Task) (bool, error) { return true, nil }, task.WithCancellationDrain())
		result, err := handle.GetResult()
		Expect(result).To(BeFalse())
		Expect(err).To(MatchError(context.Canceled))
	})

	It("preserves the original work total when a managed run stops early", func() {
		run := task.StartManagedRun("batch outcomes")
		initial := task.WorkProgress{Total: 20}
		run.SetWorkProgress(initial)
		final := task.WorkProgress{Total: 20, Completed: 1, Unstarted: 19}
		run.SetWorkProgress(final)
		run.Finish(task.StatusCancelled, context.Canceled)
		snapshot := task.SnapshotByID(run.ID())[0]
		Expect(snapshot.Work).To(Equal(&final))
		Expect(task.RunMetaFromSnapshot(snapshot).Work).To(Equal(&final))
		Expect(func() { run.SetWorkProgress(task.WorkProgress{Total: 1}) }).To(Panic())
	})

	It("preserves prompt legacy cancellation by default", func() {
		started, release := make(chan struct{}), make(chan struct{})
		handle := task.StartTask("notify canceled work", func(ctx commonscontext.Context, _ *task.Task) (string, error) {
			close(started)
			<-ctx.Done()
			<-release
			return "reaped", context.Canceled
		})
		Eventually(started).Should(BeClosed())
		handle.Cancel()

		result, err := handle.GetResult()

		Expect(result).To(BeEmpty())
		Expect(err).ToNot(HaveOccurred())
		close(release)
		Eventually(func() any {
			result, _ := handle.GetTask().GetResult()
			return result
		}).Should(Equal("reaped"))
	})

	It("waits for a canceled callback to reap its work when opted in", func() {
		started, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
		handle := task.StartTask("reap canceled work", func(ctx commonscontext.Context, _ *task.Task) (string, error) {
			close(started)
			<-ctx.Done()
			<-release
			return "reaped", context.Canceled
		}, task.WithCancellationDrain())
		Eventually(started).Should(BeClosed())
		go func() {
			defer GinkgoRecover()
			result, err := handle.GetResult()
			Expect(result).To(Equal("reaped"))
			Expect(err).To(MatchError(context.Canceled))
			close(returned)
		}()
		handle.Cancel()
		Consistently(returned, 100*time.Millisecond).ShouldNot(BeClosed())
		close(release)
		Eventually(returned).Should(BeClosed())
	})

	It("returns group metadata together with a child error", func() {
		group := task.StartGroup[bool]("failed aggregate")
		group.Add("fails", func(commonscontext.Context, *task.Task) (bool, error) {
			return false, errors.New("fatal aggregate failure")
		})

		result := group.WaitFor()

		Expect(result.Error).To(MatchError("fatal aggregate failure"))
		Expect(result.Status).To(Equal(task.StatusFailed))
		Expect(result.Duration).To(BeNumerically(">", 0))
		Expect(result.TaskCount).To(Equal(1))
		Expect(result.FailureCount).To(Equal(1))
	})

	It("marks the transition to running as dirty", func() {
		release := make(chan struct{})
		handle := task.StartTask("visible running state", func(commonscontext.Context, *task.Task) (bool, error) {
			<-release
			return true, nil
		})
		Eventually(handle.Status).Should(Equal(task.StatusRunning))

		Expect(handle.PopDirty()).To(BeTrue())
		close(release)
		_, err := handle.GetResult()
		Expect(err).ToNot(HaveOccurred())
	})
})
