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
})
