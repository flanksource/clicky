package task

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/clicky/metrics"
	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pool metrics", func() {
	const workers = 3

	var original *Manager

	BeforeEach(func() {
		original = global
		global = newTestManager(workers)
	})

	AfterEach(func() {
		global.stopRender()
		global = original
	})

	// sample records one pool sample into a fresh store and returns each
	// recorded id with its value.
	sample := func() map[string]float64 {
		store := metrics.NewMemory(metrics.MemoryConfig{})
		at := time.Now()
		Expect(global.recordPoolMetrics(store, at)).To(Succeed())
		ids := []string{
			MetricWorkersBusy, MetricWorkersBackground, MetricWorkersLimit,
			MetricSlotsUsed, MetricSlotsCapacity, MetricQueued, MetricSupervisors,
		}
		for i, name := range ids {
			ids[i] = MetricID(PoolMetricRunID, name)
		}
		global.mu.RLock()
		for _, g := range global.groups {
			for _, name := range []string{MetricConcurrencyRunning, MetricConcurrencyLimit, MetricConcurrencyWaiting} {
				ids = append(ids, MetricID(g.ID(), name))
			}
		}
		global.mu.RUnlock()
		out := map[string]float64{}
		for _, id := range ids {
			points, err := store.Query(metrics.QueryRequest{ID: id})
			Expect(err).NotTo(HaveOccurred())
			if len(points) > 0 {
				out[id] = points[len(points)-1].Value
			}
		}
		return out
	}

	It("records busy workers, slots and a throttled group's admission line", func() {
		gate := make(chan struct{})
		block := func(flanksourceContext.Context, *Task) (int, error) { <-gate; return 0, nil }

		throttled := StartGroup[int]("throttled", WithConcurrency(1))
		for i := range 3 {
			throttled.Add(fmt.Sprintf("member-%d", i), block)
		}
		free := StartGroup[int]("free")
		free.Add("ungrouped", block)
		defer func() {
			close(gate)
			throttled.WaitFor()
			free.WaitFor()
		}()

		pool := func(name string) string { return MetricID(PoolMetricRunID, name) }
		run := func(name string) string { return MetricID(throttled.ID(), name) }
		Eventually(sample).Should(Equal(map[string]float64{
			pool(MetricWorkersBusy):       2,
			pool(MetricWorkersBackground): 0,
			pool(MetricWorkersLimit):      workers,
			pool(MetricSlotsUsed):         2,
			pool(MetricSlotsCapacity):     workers,
			pool(MetricQueued):            0,
			pool(MetricSupervisors):       0,
			run(MetricConcurrencyRunning): 1,
			run(MetricConcurrencyLimit):   1,
			run(MetricConcurrencyWaiting): 2,
		}))
	})

	It("stops recording a concurrency-limited group once it finishes", func() {
		group := StartGroup[int]("finished", WithConcurrency(2))
		group.Add("only", func(flanksourceContext.Context, *Task) (int, error) { return 0, nil })
		group.WaitFor()

		Eventually(sample).Should(HaveKeyWithValue(MetricID(PoolMetricRunID, MetricWorkersBusy), 0.0))
		for id := range sample() {
			Expect(id).To(HavePrefix(PoolMetricRunID+":"), "a finished group must record no gauges")
			Expect(strings.Contains(id, group.ID())).To(BeFalse())
		}
	})
})
