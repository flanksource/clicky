package task

import (
	"fmt"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Group throughput", func() {
	const (
		members     = 2000
		concurrency = 4
		budget      = 5 * time.Second
	)

	var original *Manager

	BeforeEach(func() {
		original = global
		global = newTestManager(8)
	})

	AfterEach(func() {
		global.stopRender()
		global = original
	})

	DescribeTable("drains a bounded group of many members within a budget",
		func(body time.Duration) {
			group := StartGroup[int](fmt.Sprintf("throughput-%s", body), WithConcurrency(concurrency))
			start := time.Now()
			for i := range members {
				group.Add(fmt.Sprintf("member-%d", i), func(flanksourceContext.Context, *Task) (int, error) {
					time.Sleep(body)
					return i, nil
				})
			}
			group.WaitFor()
			Expect(group.Status()).To(Equal(StatusSuccess))
			Expect(time.Since(start)).To(BeNumerically("<", budget))
		},
		Entry("members that return at once", time.Duration(0)),
		Entry("members that work 1ms each", time.Millisecond),
	)

	It("finishes a group whose members are cancelled while they wait for a permit", func() {
		group := StartGroup[int]("cancel-waiting", WithConcurrency(1))
		release := make(chan struct{})
		holder := group.Add("holder", func(flanksourceContext.Context, *Task) (int, error) {
			<-release
			return 0, nil
		})
		waiting := make([]TypedTask[int], 3)
		for i := range waiting {
			waiting[i] = group.Add(fmt.Sprintf("waiting-%d", i), func(flanksourceContext.Context, *Task) (int, error) { return i, nil })
		}
		// Give the workers time to dequeue the waiting members and find no permit.
		time.Sleep(100 * time.Millisecond)
		for _, t := range waiting {
			t.Cancel()
		}
		close(release)
		Eventually(func() bool { return isRunning(group.Status()) }, budget).Should(BeFalse())
		Expect(holder.Status()).To(Equal(StatusSuccess))
		for _, t := range waiting {
			Expect(t.Status()).To(Equal(StatusCancelled))
		}
	})
})
