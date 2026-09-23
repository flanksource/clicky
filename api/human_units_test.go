package api

import (
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("human units", func() {
	ginkgo.DescribeTable("HumanizeBytes uses binary units with one trimmed decimal",
		func(bytes int64, expected string) {
			Expect(HumanizeBytes(bytes)).To(Equal(Text{Content: expected}))
		},
		ginkgo.Entry("zero", int64(0), "0B"),
		ginkgo.Entry("one byte", int64(1), "1B"),
		ginkgo.Entry("just below a kilobyte", int64(1023), "1023B"),
		ginkgo.Entry("exact kilobyte", int64(1024), "1K"),
		ginkgo.Entry("fractional kilobyte", int64(1536), "1.5K"),
		ginkgo.Entry("rounds down to one decimal", int64(1996), "1.9K"),
		ginkgo.Entry("rounds up and trims .0", int64(2047), "2K"),
		ginkgo.Entry("megabyte", int64(1<<20), "1M"),
		ginkgo.Entry("gigabyte", int64(1<<30), "1G"),
		ginkgo.Entry("terabyte", int64(1<<40), "1T"),
		ginkgo.Entry("petabyte", int64(1<<50), "1P"),
		ginkgo.Entry("exabyte", int64(1<<60), "1E"),
		ginkgo.Entry("negative wraps to uint64 max", int64(-1), "16E"),
	)

	ginkgo.DescribeTable("Human formats durations of a day or more as d/w units",
		func(d time.Duration, expected string) {
			Expect(Human(d)).To(Equal(Text{Content: expected, Style: "duration"}))
		},
		ginkgo.Entry("exactly one day", 24*time.Hour, "24h"),
		ginkgo.Entry("one day plus seconds truncates to minutes", 24*time.Hour+30*time.Second, "24h"),
		ginkgo.Entry("one day one hour", 25*time.Hour, "1d1h"),
		ginkgo.Entry("keeps a non-zero minute component", 26*time.Hour+30*time.Minute, "1d2h30m"),
		ginkgo.Entry("two days keeps the zero hour", 48*time.Hour, "2d0h"),
		ginkgo.Entry("exactly one week", 168*time.Hour, "7d0h"),
		ginkgo.Entry("over a week truncates to hours", 169*time.Hour+30*time.Minute, "1w0d1h"),
		ginkgo.Entry("multiple weeks", 400*time.Hour, "2w2d16h"),
	)

	ginkgo.It("humanizeDuration refuses durations shorter than a day", func() {
		Expect(func() { humanizeDuration(24*time.Hour - time.Nanosecond) }).To(PanicWith(ContainSubstring("shorter than a day")))
	})
})
