package process

import (
	"context"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cumulative CPU time", func() {
	DescribeTable("parses ps time columns",
		func(value string, expected time.Duration) {
			Expect(parseCPUTime(value)).To(Equal(expected))
		},
		Entry("darwin minutes:seconds.hundredths", "0:00.00", time.Duration(0)),
		Entry("darwin minutes beyond an hour", "158:22.97", 158*time.Minute+22*time.Second+970*time.Millisecond),
		Entry("linux hours:minutes:seconds", "01:02:03", time.Hour+2*time.Minute+3*time.Second),
		Entry("linux days", "2-03:04:05", 51*time.Hour+4*time.Minute+5*time.Second),
		Entry("days with fractional seconds", "1-00:00:00.5", 24*time.Hour+500*time.Millisecond),
	)

	DescribeTable("rejects malformed ps time columns",
		func(value string) {
			_, err := parseCPUTime(value)
			Expect(err).To(MatchError(ContainSubstring(value)))
		},
		Entry("empty", ""),
		Entry("bare seconds", "12"),
		Entry("too many fields", "1:2:3:4"),
		Entry("non-numeric", "ab:cd"),
		Entry("negative days", "-1-00:00:00"),
	)

	DescribeTable("maps ps state codes to a status",
		func(stat, status string, active bool) {
			gotStatus, gotActive := processStatus(stat)
			Expect([]any{gotStatus, gotActive}).To(Equal([]any{status, active}))
		},
		Entry("running", "R+", "active", true),
		Entry("sleeping", "Ss", "sleeping", true),
		Entry("stopped", "T", "stopped", false),
		Entry("zombie", "Z", "zombie", false),
		Entry("darwin trying to exit", "E", "exiting", false),
		Entry("linux dead", "X", "exiting", false),
	)

	It("fails loudly on a ps row with an unparsable numeric column", func() {
		_, err := parseSnapshot([]byte("9000 1 x.y 2.0 1024 S 0:01.00 Mon Sep  8 10:00:00 2026 agent"), time.UTC)
		Expect(err).To(MatchError(And(ContainSubstring("pcpu"), ContainSubstring("x.y"))))
	})

	It("fails loudly on a truncated ps row", func() {
		_, err := parseSnapshot([]byte("9000 1 1.5 2.0 1024 S"), time.UTC)
		Expect(err).To(MatchError(ContainSubstring("9000 1 1.5")))
	})
})

var _ = Describe("IntervalCPUPercent", func() {
	started := time.Date(2026, time.September, 8, 10, 0, 0, 0, time.UTC)
	at := started.Add(time.Hour)
	sample := func(cpu time.Duration, start time.Time, offset time.Duration) CPUSample {
		return CPUSample{Process: Process{PID: 9000, StartedAt: &start, CPUTime: cpu}, At: at.Add(offset)}
	}

	It("is unknown on the first observation", func() {
		Expect(IntervalCPUPercent(nil, sample(time.Second, started, 0))).To(BeNil())
	})

	It("divides CPU time used by wall time elapsed, per core", func() {
		previous := sample(10*time.Second, started, 0)
		Expect(IntervalCPUPercent(&previous, sample(13*time.Second, started, 2*time.Second))).To(HaveValue(BeNumerically("~", 150, 1e-9)))
	})

	It("is unknown when the pid was reused by a process with a different start time", func() {
		previous := sample(10*time.Second, started, 0)
		Expect(IntervalCPUPercent(&previous, sample(time.Second, started.Add(time.Minute), time.Second))).To(BeNil())
	})

	It("rejects a sample whose clock did not advance", func() {
		previous := sample(10*time.Second, started, 0)
		_, err := IntervalCPUPercent(&previous, sample(11*time.Second, started, 0))
		Expect(err).To(MatchError(ContainSubstring("did not advance")))
	})

	It("rejects CPU time going backwards for the same process", func() {
		previous := sample(10*time.Second, started, 0)
		_, err := IntervalCPUPercent(&previous, sample(9*time.Second, started, time.Second))
		Expect(err).To(MatchError(ContainSubstring("went backwards")))
	})

	It("rejects comparing two different pids", func() {
		previous := sample(10*time.Second, started, 0)
		current := sample(11*time.Second, started, time.Second)
		current.Process.PID = 9001
		_, err := IntervalCPUPercent(&previous, current)
		Expect(err).To(MatchError(ContainSubstring("9001")))
	})

	It("rejects a process without a start time", func() {
		previous := sample(10*time.Second, started, 0)
		current := sample(11*time.Second, started, time.Second)
		current.Process.StartedAt = nil
		_, err := IntervalCPUPercent(&previous, current)
		Expect(err).To(MatchError(ContainSubstring("start time")))
	})

	It("reads a busy process as busy over the sampled interval", func() {
		var stop atomic.Bool
		defer stop.Store(true)
		go func() {
			runtime.LockOSThread()
			for !stop.Load() {
			}
		}()
		first := discoverSelf()
		time.Sleep(1500 * time.Millisecond)
		percent, err := IntervalCPUPercent(&first, discoverSelf())
		Expect(err).NotTo(HaveOccurred())
		Expect(percent).To(HaveValue(BeNumerically(">", 40)))
	})
})

func discoverSelf() CPUSample {
	GinkgoHelper()
	snapshot, err := Discover(context.Background(), SnapshotOptions{})
	Expect(err).NotTo(HaveOccurred())
	at := time.Now()
	self, found := snapshot.Get(os.Getpid())
	Expect(found).To(BeTrue())
	return CPUSample{Process: self, At: at}
}
