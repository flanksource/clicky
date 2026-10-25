package task_test

import (
	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("task snapshot dependencies", func() {
	It("starts planned steps only after their prerequisite finishes and records timestamps", func() {
		group := clicky.StartGroup[struct{}]("planned steps", task.WithNoRender())
		started := group.StartedAt()
		first := group.Group.PlanStep("Connect database")
		next := group.Group.PlanStep("Resolve schemas", task.WithDependencies(first.Task()))
		DeferCleanup(func() { first.Finish(nil); next.Finish(nil) })
		pending := task.SnapshotTask(next.Task(), group.Group)
		Expect(pending.StartedAt).To(BeEmpty())
		Expect(pending.MaxValue).To(BeZero())
		Expect(group.StartedAt()).To(Equal(started))
		Expect(task.SnapshotGroup(group.Group).StartedAt).NotTo(BeEmpty())
		Expect(next.Start()).To(MatchError(ContainSubstring("waiting for Connect database")))
		Expect(first.Start()).To(Succeed())
		Expect(group.StartedAt()).To(Equal(started))
		Expect(first.Start()).To(MatchError(ContainSubstring("cannot start from")))
		first.Finish(nil)
		Expect(next.Start()).To(Succeed())
		next.Finish(nil)
		snapshot := task.SnapshotTask(next.Task(), group.Group)
		Expect(snapshot.Status).To(Equal("success"))
		Expect(snapshot.StartedAt).NotTo(BeEmpty())
		Expect(snapshot.FinishedAt).NotTo(BeEmpty())
	})
	It("exposes stable prerequisite identities while the dependent is pending", func() {
		release := make(chan struct{})
		first := clicky.StartTask("Connect database", func(commonscontext.Context, *task.Task) (struct{}, error) {
			<-release
			return struct{}{}, nil
		})
		DeferCleanup(func() { close(release); first.WaitFor() })
		next := clicky.StartTask("Resolve schemas", func(commonscontext.Context, *task.Task) (struct{}, error) {
			return struct{}{}, nil
		}, clicky.WithDependencies(first.GetTask()))
		DeferCleanup(func() { next.Cancel(); next.WaitFor() })
		Expect(task.SnapshotTask(next.GetTask(), nil).DependencyIDs).To(Equal([]string{first.GetTask().ID()}))
		Expect(task.SnapshotTask(next.GetTask(), nil).Status).To(Equal("pending"))
	})
})
