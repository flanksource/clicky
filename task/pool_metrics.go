package task

import (
	"errors"
	"sync"
	"time"

	"github.com/flanksource/clicky/metrics"
	"github.com/flanksource/commons/logger"
)

// PoolMetricRunID is the run id the worker-pool gauges are recorded under, so
// MetricID(PoolMetricRunID, MetricWorkersBusy) is "pool:workers-busy".
const PoolMetricRunID = "pool"

// Worker-pool gauge names, recorded under PoolMetricRunID.
const (
	MetricWorkersBusy       = "workers-busy"
	MetricWorkersBackground = "workers-background"
	MetricWorkersLimit      = "workers-limit"
	MetricSlotsUsed         = "slots-used"
	MetricSlotsCapacity     = "slots-capacity"
	MetricQueued            = "queued"
	MetricSupervisors       = "supervisors"
)

// Concurrency-limited group gauge names, recorded under the group's run id.
const (
	MetricConcurrencyRunning = "concurrency-running"
	MetricConcurrencyLimit   = "concurrency-limit"
	MetricConcurrencyWaiting = "concurrency-waiting"
)

const poolSampleInterval = time.Second

var poolSampler sync.Once

// startPoolSampler records the global manager's pool gauges into the task
// metric store every poolSampleInterval, from the first call on. It is started
// by the HTTP registration, the only consumer of the series.
func startPoolSampler() {
	poolSampler.Do(func() {
		go global.samplePool(runMetrics, poolSampleInterval)
	})
}

func (tm *Manager) samplePool(store metrics.Timeseries, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	failing := false
	for {
		select {
		case <-tm.shutdown:
			return
		case at := <-ticker.C:
			err := tm.recordPoolMetrics(store, at)
			if err != nil && !failing {
				logger.GetLogger("task").Errorf("record task pool metrics: %v", err)
			}
			failing = err != nil
		}
	}
}

// recordPoolMetrics writes one point per pool gauge, and per gauge of every
// live concurrency-limited group, at at.
func (tm *Manager) recordPoolMetrics(store metrics.Timeseries, at time.Time) error {
	tm.mu.RLock()
	workers := append([]*worker(nil), tm.workers...)
	groups := append([]*Group(nil), tm.groups...)
	slotsUsed, slotsCapacity := tm.slotsUsed, tm.maxConcurrent
	tm.mu.RUnlock()

	busy, background := 0, 0
	for _, w := range workers {
		if t := w.inflight.Load(); t != nil {
			busy++
			if t.IsBackground() {
				background++
			}
		}
	}
	limit := len(workers)
	if l := int(tm.workerLimit.Load()); l > 0 {
		limit = l
	}
	if slotsCapacity <= 0 {
		slotsCapacity = len(workers)
	}
	supervisors := 0
	tm.supervisors.Range(func(_, _ any) bool { supervisors++; return true })

	values := map[string]int{
		MetricID(PoolMetricRunID, MetricWorkersBusy):       busy,
		MetricID(PoolMetricRunID, MetricWorkersBackground): background,
		MetricID(PoolMetricRunID, MetricWorkersLimit):      limit,
		MetricID(PoolMetricRunID, MetricSlotsUsed):         slotsUsed,
		MetricID(PoolMetricRunID, MetricSlotsCapacity):     slotsCapacity,
		MetricID(PoolMetricRunID, MetricQueued):            tm.taskQueue.Size(),
		MetricID(PoolMetricRunID, MetricSupervisors):       supervisors,
	}
	for _, g := range groups {
		if g.concurrency <= 0 {
			continue
		}
		if status := g.Status(); status != StatusRunning && status != StatusPending {
			continue
		}
		running, waiting := g.admissionCounts()
		values[MetricID(g.ID(), MetricConcurrencyRunning)] = running
		values[MetricID(g.ID(), MetricConcurrencyLimit)] = g.concurrency
		values[MetricID(g.ID(), MetricConcurrencyWaiting)] = waiting
	}

	var errs []error
	for id, value := range values {
		if err := store.Record(metrics.RecordRequest{ID: id, At: at, Value: float64(value)}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// admissionCounts reports how many of the group's own tasks are running and
// how many are still in its admission line waiting to start.
func (g *Group) admissionCounts() (running, waiting int) {
	for _, item := range g.GetTasks() {
		if item.GetTask().Status() == StatusRunning {
			running++
		}
	}
	g.admission.mu.Lock()
	defer g.admission.mu.Unlock()
	for _, t := range g.admission.waiting {
		if !retiredBeforeStart(t) {
			waiting++
		}
	}
	return running, waiting
}
