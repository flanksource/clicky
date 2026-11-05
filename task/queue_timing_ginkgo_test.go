package task_test

import (
	"context"
	"time"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task progress audit: queue timing", func() {
	It("keeps execution timestamps empty when cancelled behind a dependency", func() {
		group := task.StartGroup[bool]("queued timing")
		dependency := group.PlanStep("dependency")
		defer dependency.Finish(nil)
		queued := group.Add("never executed", func(commonscontext.Context, *task.Task) (bool, error) {
			defer GinkgoRecover()
			Fail("cancelled queued work must not execute")
			return false, nil
		}, task.WithDependencies(dependency.Task()))
		defer queued.Cancel()
		Expect(queued.StartTime()).To(BeZero())
		Eventually(queued.WaitTime).Should(BeNumerically(">", 0))
		queued.Cancel()
		result := queued.WaitFor()
		Expect(result.Duration).To(BeZero())
		Expect(queued.Duration()).To(BeZero())
		Expect(queued.StartTime()).To(BeZero())
		Expect(queued.WaitTime()).To(BeNumerically(">", 0))
		for _, snapshot := range task.SnapshotByID(group.ID()) {
			if snapshot.ID == queued.ID() {
				Expect(snapshot.StartedAt).To(BeEmpty())
			}
		}
	})

	It("freezes queue time once execution starts", func() {
		started, release := make(chan struct{}), make(chan struct{})
		defer close(release)
		current := task.StartTask("running timing", func(ctx commonscontext.Context, _ *task.Task) (bool, error) {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
			}
			return true, nil
		}, task.WithContext(context.Background()))
		defer current.Cancel()
		Eventually(started).Should(BeClosed())
		waited := current.WaitTime()
		Consistently(current.WaitTime, 30*time.Millisecond).Should(Equal(waited))
		Expect(current.StartTime()).NotTo(BeZero())
		Expect(current.Duration()).To(BeNumerically(">", 0))
	})
})
