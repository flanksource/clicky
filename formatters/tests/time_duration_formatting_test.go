package formatters

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type formatFixture struct {
	name     string
	input    any
	style    string
	str      string
	ansi     string
	html     string
	markdown string
}

func expectStringsEqual(expected, actual, message string) {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)

	if expected == actual {
		return
	}

	if strings.EqualFold(expected, actual) {
		message += " (case mismatch)"
	}

	ginkgo.Fail(fmt.Sprintf("%s '%s'(%d) != '%s'(%d)", message, expected, len(expected), actual, len(actual)), 1)
}

func runTests(tests []formatFixture) {

	for _, tt := range tests {
		tt := tt
		ginkgo.It(tt.name, func() {
			text := api.Human(tt.input, tt.style)

			expectStringsEqual(tt.str, text.String(), "String() output should match")
			expectStringsEqual(tt.html, text.HTML(), "HTML() output should match")
			expectStringsEqual(tt.markdown, text.Markdown(), "Markdown() output should match")

			// Verify ANSI() output contains the content
			if tt.ansi != "" {
				Expect(text.ANSI()).To(Equal(tt.ansi), "ANSI() output should match")
			} else {
				Expect(text.ANSI()).To(ContainSubstring(tt.str), "ANSI() should contain the string content")
			}
		})
	}
}

var _ = ginkgo.Describe("Time and Duration Formatting", func() {

	ginkgo.Context("Time Formatting", func() {
		tests := []formatFixture{
			{
				name:     "RFC3339 time (UTC)",
				input:    time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
				style:    "date",
				str:      "2024-01-15T14:30:00Z",
				ansi:     "2024-01-15T14:30:00Z",
				html:     `<span class="date">2024-01-15T14:30:00Z</span>`,
				markdown: `2024-01-15T14:30:00Z`,
			},
			{
				name:     "RFC3339 time with milliseconds",
				input:    time.Date(2024, 1, 15, 14, 30, 45, 123456789, time.UTC),
				style:    "date",
				str:      "2024-01-15T14:30:45Z",
				ansi:     "2024-01-15T14:30:45Z",
				html:     `<span class="date">2024-01-15T14:30:45Z</span>`,
				markdown: `2024-01-15T14:30:45Z`,
			},
			{
				name:     "Zero time",
				input:    time.Time{},
				style:    "date",
				str:      "",
				ansi:     "",
				html:     `<span class="date"></span>`,
				markdown: ``,
			},
		}

		for _, tt := range tests {
			tt := tt
			ginkgo.It(tt.name, func() {
				text := api.Human(tt.input, tt.style)

				// Verify String() output
				Expect(text.String()).To(Equal(tt.str), "String() output should match")

				// Verify HTML() output
				Expect(text.HTML()).To(Equal(tt.html), "HTML() output should match")

				// Verify Markdown() output
				Expect(text.Markdown()).To(Equal(tt.markdown), "Markdown() output should match")

				// Verify ANSI() output (if expected is provided)
				if tt.ansi != "" {
					Expect(text.ANSI()).To(Equal(tt.ansi), "ANSI() output should match")
				} else {
					// At minimum, verify ANSI output contains the content
					Expect(text.ANSI()).To(ContainSubstring(tt.str), "ANSI() should contain the string content")
				}
			})
		}
	})

	ginkgo.Context("Duration Formatting", func() {
		tests := []formatFixture{
			{
				name:     "< 5s (100ms)",
				input:    100 * time.Millisecond,
				str:      "100ms",
				ansi:     "100ms",
				html:     `<span class="duration">100ms</span>`,
				markdown: `100ms`,
			},
			{
				name:     "< 5s (1.5s as 1500ms)",
				input:    1500 * time.Millisecond,
				str:      "1500ms",
				ansi:     "1500ms",
				html:     `<span class="duration">1500ms</span>`,
				markdown: `1500ms`,
			},
			{
				name:     "5s-1m (12.34s)",
				input:    12340 * time.Millisecond,
				style:    "duration",
				str:      "12.34s",
				ansi:     "12.34s",
				html:     `<span class="duration">12.34s</span>`,
				markdown: `12.34s`,
			},
			{
				name:     "5s-1m (exactly 5s)",
				input:    5 * time.Second,
				style:    "duration",
				str:      "5.00s",
				ansi:     "5.00s",
				html:     `<span class="duration">5.00s</span>`,
				markdown: `5.00s`,
			},
			{
				name:     "1m-1h (5.5m)",
				input:    5*time.Minute + 30*time.Second,
				style:    "duration",
				str:      "5.5m",
				ansi:     "5.5m",
				html:     `<span class="duration">5.5m</span>`,
				markdown: `5.5m`,
			},
			{
				name:     "1m-1h (exactly 1m)",
				input:    1 * time.Minute,
				style:    "duration",
				str:      "1.0m",
				ansi:     "1.0m",
				html:     `<span class="duration">1.0m</span>`,
				markdown: `1.0m`,
			},
			{
				name:     "1m-1h (30.5m)",
				input:    30*time.Minute + 30*time.Second,
				style:    "duration",
				str:      "30.5m",
				ansi:     "30.5m",
				html:     `<span class="duration">30.5m</span>`,
				markdown: `30.5m`,
			},
			{
				name:     "1h-24h (3.2h)",
				input:    3*time.Hour + 12*time.Minute,
				style:    "duration",
				str:      "3.2h",
				ansi:     "3.2h",
				html:     `<span class="duration">3.2h</span>`,
				markdown: `3.2h`,
			},
			{
				name:     "1h-24h (exactly 1h)",
				input:    1 * time.Hour,
				style:    "duration",
				str:      "1.0h",
				ansi:     "1.0h",
				html:     `<span class="duration">1.0h</span>`,
				markdown: `1.0h`,
			},
			{
				name:     "1h-24h (12.5h)",
				input:    12*time.Hour + 30*time.Minute,
				style:    "duration",
				str:      "12.5h",
				ansi:     "12.5h",
				html:     `<span class="duration">12.5h</span>`,
				markdown: `12.5h`,
			},
			{
				name:     ">= 24h (2 days)",
				input:    48 * time.Hour,
				style:    "duration",
				str:      "2d",
				ansi:     "2d",
				html:     `<span class="duration">2dh</span>`,
				markdown: `2d`,
			},
			{
				name:     ">= 24h (2 days)",
				input:    28 * time.Hour,
				style:    "duration",
				str:      "1d4h",
				ansi:     "1d4h",
				html:     `<span class="duration">1d4h</span>`,
				markdown: `1d4h`,
			},
			{
				name:     ">= 24h (exactly 24h)",
				input:    24 * time.Hour,
				style:    "duration",
				str:      "24h",
				ansi:     "24h",
				html:     `<span class="duration">24h</span>`,
				markdown: `24h`,
			},
			{
				name:     ">= 24h (3 days 6 hours)",
				input:    78 * time.Hour,
				style:    "duration",
				str:      "3d6h",
				ansi:     "3d6h",
				html:     `<span class="duration">3d6h</span>`,
				markdown: `3d6h`,
			},
			{
				name:     "Zero duration",
				input:    0 * time.Second,
				style:    "duration",
				str:      "0ms",
				ansi:     "0ms",
				html:     `<span class="duration">0ms</span>`,
				markdown: `0ms`,
			},
		}
		runTests(tests)

	})

	ginkgo.Context("Pointer Handling", func() {
		tests := []formatFixture{
			{
				name:     "Non-nil time pointer",
				input:    func() *time.Time { t := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC); return &t }(),
				style:    "date",
				str:      "2024-01-15T14:30:00Z",
				ansi:     "2024-01-15T14:30:00Z",
				html:     `<span class="date">2024-01-15T14:30:00Z</span>`,
				markdown: `2024-01-15T14:30:00Z`,
			},
			{
				name:     "Non-nil duration pointer (5m)",
				input:    func() *time.Duration { d := 5 * time.Minute; return &d }(),
				style:    "duration",
				str:      "5.0m",
				ansi:     "5.0m",
				html:     `<span class="duration">5.0m</span>`,
				markdown: `5.0m`,
			},
			{
				name:     "Non-nil duration pointer (30s)",
				input:    func() *time.Duration { d := 30 * time.Second; return &d }(),
				style:    "duration",
				str:      "30.00s",
				ansi:     "30.00s",
				html:     `<span class="duration">30.00s</span>`,
				markdown: `30.00s`,
			},
		}
		runTests(tests)

	})

	ginkgo.Context("Edge Cases and Boundaries", func() {
		ginkgo.It("should handle boundary: exactly 5 seconds", func() {
			text := api.Human(5*time.Second, "duration")
			Expect(text.String()).To(Equal("5.00s"))
		})

		ginkgo.It("should handle boundary: just under 5 seconds", func() {
			text := api.Human(4999*time.Millisecond, "duration")
			Expect(text.String()).To(Equal("4999ms"))
		})

		ginkgo.It("should handle boundary: exactly 1 minute", func() {
			text := api.Human(1*time.Minute, "duration")
			Expect(text.String()).To(Equal("1.0m"))
		})

		ginkgo.It("should handle boundary: just under 1 minute", func() {
			text := api.Human(59*time.Second+999*time.Millisecond, "duration")
			Expect(text.String()).To(Equal("60.00s")) // Rounding causes 59.999s -> 60.00s
		})

		ginkgo.It("should handle boundary: exactly 1 hour", func() {
			text := api.Human(1*time.Hour, "duration")
			Expect(text.String()).To(Equal("1.0h"))
		})

		ginkgo.It("should handle boundary: just under 1 hour", func() {
			text := api.Human(59*time.Minute+59*time.Second, "duration")
			Expect(text.String()).To(Equal("60.0m")) // Rounding causes 59.98m -> 60.0m
		})

		ginkgo.It("should handle boundary: exactly 24 hours", func() {
			text := api.Human(24*time.Hour, "duration")
			Expect(text.String()).To(Equal("24h")) // Go's HumanizeDuration formats as "24h" not "1d"
		})

		ginkgo.It("should handle boundary: just under 24 hours", func() {
			text := api.Human(23*time.Hour+59*time.Minute, "duration")
			Expect(text.String()).To(Equal("24.0h")) // Rounding causes 23.98h -> 24.0h
		})
	})

	ginkgo.Context("High Precision Timestamps", func() {
		ginkgo.It("should handle nanosecond precision", func() {
			t := time.Date(2024, 1, 15, 14, 30, 45, 123456789, time.UTC)
			text := api.Human(t, "date")
			// RFC3339 format drops sub-second precision by default
			Expect(text.String()).To(Equal("2024-01-15T14:30:45Z"))
		})

		ginkgo.It("should handle microsecond precision duration", func() {
			d := 1234 * time.Microsecond
			text := api.Human(d, "duration")
			// Should be < 5s, so displayed as milliseconds
			Expect(text.String()).To(Equal("1ms"))
		})

		ginkgo.It("should handle very small duration (nanoseconds)", func() {
			d := 500 * time.Nanosecond
			text := api.Human(d, "duration")
			// Should be < 5s, so displayed as milliseconds (0ms)
			Expect(text.String()).To(Equal("0ms"))
		})
	})
})
