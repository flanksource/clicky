package task_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Nested group lifecycle", func() {
	It("records parent completion when its last nested worker finishes", func(ctx SpecContext) {
		parent := task.StartGroup[bool]("terminal parent", task.WithGroupContext(ctx))
		child := task.StartGroup[bool]("terminal child", task.WithParentGroup(parent.Group))
		worker := child.Add("read", func(ctx commonscontext.Context, _ *task.Task) (bool, error) { return true, ctx.Err() })
		worker.WaitFor()
		Eventually(parent.FinishedAt).ShouldNot(BeZero())
	})

	It("waits for a child added while the parent's results are being collected", func(ctx SpecContext) {
		parent := task.StartGroup[bool]("dynamic parent", task.WithGroupContext(ctx))
		create, started, finish := make(chan struct{}), make(chan struct{}), make(chan struct{})
		release := sync.OnceFunc(func() { close(finish) })
		defer release()
		parent.Add("create child", func(ctx commonscontext.Context, _ *task.Task) (bool, error) {
			<-create
			child := task.StartGroup[bool]("dynamic child", task.WithParentGroup(parent.Group), task.WithGroupContext(parent.Context()))
			child.Add("read", func(commonscontext.Context, *task.Task) (bool, error) { close(started); <-finish; return true, nil })
			return true, nil
		}, task.WithSlots(0))
		done := make(chan error, 1)
		go func() { _, err := parent.GetResults(); done <- err }()
		close(create)
		Eventually(started).Should(BeClosed())
		Consistently(done, 100*time.Millisecond).ShouldNot(Receive())
		release()
		Eventually(done).Should(Receive(BeNil()))
	})

	It("runs a supervising parent and nested worker with concurrency one", func(ctx SpecContext) {
		parent := task.StartGroup[bool]("single slot parent", task.WithGroupContext(ctx), task.WithConcurrency(1))
		done := make(chan error, 1)
		parent.Add("supervise", func(ctx commonscontext.Context, _ *task.Task) (bool, error) {
			child := task.StartGroup[bool]("single slot child", task.WithParentGroup(parent.Group), task.WithGroupContext(ctx), task.WithConcurrency(1))
			child.Add("read source", func(ctx commonscontext.Context, _ *task.Task) (bool, error) { return true, ctx.Err() })
			_, err := child.GetResults()
			done <- err
			return true, err
		}, task.WithSlots(0))
		Eventually(done, 5*time.Second).Should(Receive(BeNil()))
		Expect(parent.WaitFor().Status).To(Equal(task.StatusSuccess))
		Expect(task.SnapshotByID(parent.ID())).To(HaveLen(4))
	})

	It("retains request values, aggregates descendants and snapshots each member once", func() {
		type key struct{}
		ctx := context.WithValue(context.Background(), key{}, "request")
		parent := task.StartGroup[bool]("nested parent", task.WithGroupContext(ctx), task.WithConcurrency(1))
		child := task.StartGroup[bool]("nested child", task.WithParentGroup(parent.Group), task.WithGroupContext(ctx))
		Expect(child.Context().Value(key{})).To(Equal("request"))
		step := child.StartStep("nested fetch")
		Expect(parent.Status()).To(Equal(task.StatusRunning))
		snaps := task.SnapshotByID(parent.ID())
		Expect(snaps).To(HaveLen(3))
		Expect(snaps[1].ParentGroupID).To(Equal(parent.ID()))
		step.Finish(errors.New("fetch failed"))
		Expect(parent.WaitFor().Status).To(Equal(task.StatusFailed))
		Expect(task.SnapshotGroup(parent.Group).Failed).To(Equal(1))
	})

	It("cancels descendant steps when the request is canceled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		parent := task.StartGroup[bool]("cancel nested parent", task.WithGroupContext(ctx))
		child := task.StartGroup[bool]("cancel nested child", task.WithParentGroup(parent.Group))
		step := child.StartStep("cancel fetch")
		cancel()
		Eventually(child.Context().Err).Should(MatchError(context.Canceled))
		step.Finish(child.Context().Err())
		Expect(parent.WaitFor().Status).To(Equal(task.StatusCancelled))
	})
})
