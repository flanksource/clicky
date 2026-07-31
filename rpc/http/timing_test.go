package rpchttp

import (
	"context"
	"testing"
	"time"
)

func TestAddTimingSumsByName(t *testing.T) {
	ctx, timings := WithTimings(context.Background())
	AddTiming(ctx, TimingMetric{Name: "parse", Duration: 2 * time.Millisecond})
	AddTiming(ctx, TimingMetric{Name: "find", Duration: time.Millisecond})
	AddTiming(ctx, TimingMetric{Name: "parse", Duration: 3 * time.Millisecond})

	if got := timings.Header(); got != "parse;dur=5.0, find;dur=1.0" {
		t.Fatalf("Header() = %q", got)
	}
}

func TestHeaderEmptyWhenNoPhases(t *testing.T) {
	_, timings := WithTimings(context.Background())
	if got := timings.Header(); got != "" {
		t.Fatalf("Header() = %q, want empty", got)
	}
}

func TestTrackRecordsElapsed(t *testing.T) {
	ctx, timings := WithTimings(context.Background())
	stop := Track(ctx, "enrich")
	time.Sleep(5 * time.Millisecond)
	stop()

	total, ok := timings.metrics["enrich"]
	if !ok {
		t.Fatal("enrich not recorded")
	}
	if total.duration < 5*time.Millisecond {
		t.Fatalf("elapsed = %v, want >= 5ms", total.duration)
	}
}

func TestAddTimingNoAccumulatorIsNoop(t *testing.T) {
	// Must not panic on the CLI path where no accumulator was installed.
	AddTiming(context.Background(), TimingMetric{Name: "parse", Duration: time.Millisecond})
	Track(context.Background(), "parse")()
}
