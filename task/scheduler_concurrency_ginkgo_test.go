package task

import (
	"fmt"
	"sync/atomic"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler run concurrency", func() {
	const runTasks = 3

	var (
		original *Manager
		ctx      flanksourceContext.Context
		sched    *Scheduler
	)

	BeforeEach(func() {
		original = global
		global = newTestManager(runTasks + 1)
		ctx = flanksourceContext.NewContext(GinkgoT().Context())
		sched = NewScheduler(SchedulerOptions{})
	})

	AfterEach(func() {
		global.stopRender()
		global = original
	})

	It("rejects a negative concurrency where a schedule is added or run now", func() {
		kind := uniqueKind("negative-concurrency")
		sched.RegisterRunner(kind, func(flanksourceContext.Context, Schedule, *Group) error { return nil })
		schedule := Schedule{Name: "negative", Kind: kind, Cron: "@every 1h", Concurrency: -1}

		Expect(sched.Add(ctx, schedule)).To(MatchError(ContainSubstring("concurrency must not be negative")))
		_, err := sched.RunNow(ctx, schedule)
		Expect(err).To(MatchError(ContainSubstring("concurrency must not be negative")))
	})

	DescribeTable("runs at most Concurrency of a run's tasks at once",
		func(concurrency, expectedRunning int) {
			release := make(chan struct{})
			var running atomic.Int64
			kind := uniqueKind("concurrency")
			sched.RegisterRunner(kind, func(_ flanksourceContext.Context, _ Schedule, group *Group) error {
				typed := TypedGroup[any]{Group: group}
				for i := range runTasks {
					typed.Add(fmt.Sprintf("target-%d", i), func(flanksourceContext.Context, *Task) (any, error) {
						running.Add(1)
						defer running.Add(-1)
						<-release
						return nil, nil
					})
				}
				return nil
			})

			group, err := sched.RunNow(ctx, Schedule{Name: kind, Kind: kind, Concurrency: concurrency})
			Expect(err).NotTo(HaveOccurred())

			Eventually(running.Load).Should(BeEquivalentTo(expectedRunning))
			Consistently(running.Load, 300*time.Millisecond).Should(BeEquivalentTo(expectedRunning))
			close(release)
			Eventually(group.Status).Should(Equal(StatusSuccess))
		},
		Entry("one at a time", 1, 1),
		Entry("two at a time", 2, 2),
		Entry("all at once when unbounded", 0, runTasks),
	)
})
