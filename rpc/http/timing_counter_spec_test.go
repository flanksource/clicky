package rpchttp

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP server timing counters", func() {
	It("aggregates duration and counters in first-seen order", func() {
		ctx, timings := WithTimings(context.Background())

		AddTiming(ctx, TimingMetric{
			Name: "sql", Duration: 2 * time.Millisecond,
			Counters: []TimingCounter{{Name: "queries", Value: 1}, {Name: "rows_returned", Value: 500}},
		})
		AddTiming(ctx, TimingMetric{
			Name: "redis", Duration: time.Millisecond,
			Counters: []TimingCounter{{Name: "ops", Value: 1}, {Name: "hits", Value: 1}},
		})
		AddTiming(ctx, TimingMetric{
			Name: "sql", Duration: 3 * time.Millisecond,
			Counters: []TimingCounter{{Name: "queries", Value: 1}, {Name: "rows_returned", Value: 1}},
		})

		Expect(timings.Header()).To(Equal(
			`sql;dur=5.0;desc="queries=2 rows_returned=501", redis;dur=1.0;desc="ops=1 hits=1"`,
		))
	})

	It("retains zero-valued seeded metrics", func() {
		ctx, timings := WithTimings(context.Background())

		AddTiming(ctx, TimingMetric{
			Name: "redis",
			Counters: []TimingCounter{
				{Name: "ops"}, {Name: "hits"}, {Name: "misses"}, {Name: "errors"},
			},
		})

		Expect(timings.Header()).To(Equal(
			`redis;dur=0.0;desc="ops=0 hits=0 misses=0 errors=0"`,
		))
	})

	It("reads accumulated metrics back in insertion order", func() {
		ctx, timings := WithTimings(context.Background())

		AddTiming(ctx, TimingMetric{
			Name: "sql", Duration: 2 * time.Millisecond,
			Counters: []TimingCounter{{Name: "queries", Value: 1}, {Name: "rows_returned", Value: 500}},
		})
		AddTiming(ctx, TimingMetric{Name: "formulas", Counters: []TimingCounter{{Name: "evaluated", Value: 3}}})
		AddTiming(ctx, TimingMetric{
			Name: "sql", Duration: 3 * time.Millisecond,
			Counters: []TimingCounter{{Name: "queries", Value: 1}},
		})

		Expect(timings.Metrics()).To(Equal([]TimingMetric{
			{
				Name:     "sql",
				Duration: 5 * time.Millisecond,
				Counters: []TimingCounter{{Name: "queries", Value: 2}, {Name: "rows_returned", Value: 500}},
			},
			{
				Name:     "formulas",
				Counters: []TimingCounter{{Name: "evaluated", Value: 3}},
			},
		}))
	})

	It("reports a single counter and duration by name", func() {
		ctx, timings := WithTimings(context.Background())

		AddTiming(ctx, TimingMetric{
			Name: "sql", Duration: 4 * time.Millisecond,
			Counters: []TimingCounter{{Name: "queries", Value: 7}},
		})

		queries, ok := timings.Counter("sql", "queries")
		Expect(ok).To(BeTrue())
		Expect(queries).To(Equal(int64(7)))

		elapsed, ok := timings.Duration("sql")
		Expect(ok).To(BeTrue())
		Expect(elapsed).To(Equal(4 * time.Millisecond))

		_, ok = timings.Counter("sql", "rows_returned")
		Expect(ok).To(BeFalse(), "a counter that was never recorded is absent, not zero")
		_, ok = timings.Counter("redis", "ops")
		Expect(ok).To(BeFalse())
		_, ok = timings.Duration("redis")
		Expect(ok).To(BeFalse())
	})
})
