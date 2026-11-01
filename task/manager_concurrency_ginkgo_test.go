package task

import (
	"fmt"
	"sync/atomic"

	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SetMaxConcurrent", func() {
	const started = 2

	var original *Manager

	BeforeEach(func() {
		original = global
		global = newTestManager(started)
	})

	AfterEach(func() {
		global.stopRender()
		global = original
	})

	// runBlocked starts members tasks that block until released and reports
	// the most that ran at once once every task is released.
	runBlocked := func(members int) (peak func() int32, running func() int32, release func()) {
		var now, most atomic.Int32
		gate := make(chan struct{})
		group := StartGroup[int]("max-concurrent")
		for i := range members {
			group.Add(fmt.Sprintf("member-%d", i), func(flanksourceContext.Context, *Task) (int, error) {
				n := now.Add(1)
				for {
					m := most.Load()
					if n <= m || most.CompareAndSwap(m, n) {
						break
					}
				}
				<-gate
				now.Add(-1)
				return i, nil
			})
		}
		return most.Load, now.Load, func() {
			close(gate)
			group.WaitFor()
		}
	}

	It("runs as many tasks at once as a raised maximum allows", func() {
		SetMaxConcurrent(6)
		_, running, release := runBlocked(6)
		defer release()

		Eventually(running).Should(Equal(int32(6)))
	})

	It("runs no more tasks at once than a lowered maximum allows", func() {
		SetMaxConcurrent(6)
		SetMaxConcurrent(1)
		peak, running, release := runBlocked(4)

		Eventually(running).Should(Equal(int32(1)))
		Consistently(running, "200ms").Should(Equal(int32(1)))
		release()
		Expect(peak()).To(Equal(int32(1)))
	})
})
