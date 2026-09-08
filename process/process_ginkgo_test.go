package process

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const processFixture = `
9000 1 1.5 2.0 1024 S Mon Sep  8 10:00:00 2026 agent --session-id root
9001 9000 2.5 3.0 2048 R Mon Sep  8 10:01:00 2026 browser child
9002 9001 4.0 5.0 4096 Z Mon Sep  8 10:02:00 2026 browser grandchild
9100 1 0.5 1.0 512 T Mon Sep  8 10:03:00 2026 unrelated
`

var _ = Describe("process snapshots", func() {
	It("indexes, traverses, and aggregates one stable host snapshot", func() {
		snapshot := parseSnapshot([]byte(processFixture), time.UTC)

		Expect(snapshot.All()).To(Equal([]Process{
			{PID: 9000, PPID: 1, Status: "sleeping", Active: true, CPUPercent: 1.5, MemoryPercent: 2, RSSBytes: 1024 * 1024, StartedAt: utcTime(2026, time.September, 8, 10, 0), Command: "agent --session-id root"},
			{PID: 9001, PPID: 9000, Status: "active", Active: true, CPUPercent: 2.5, MemoryPercent: 3, RSSBytes: 2048 * 1024, StartedAt: utcTime(2026, time.September, 8, 10, 1), Command: "browser child"},
			{PID: 9002, PPID: 9001, Status: "zombie", CPUPercent: 4, MemoryPercent: 5, RSSBytes: 4096 * 1024, StartedAt: utcTime(2026, time.September, 8, 10, 2), Command: "browser grandchild"},
			{PID: 9100, PPID: 1, Status: "stopped", CPUPercent: .5, MemoryPercent: 1, RSSBytes: 512 * 1024, StartedAt: utcTime(2026, time.September, 8, 10, 3), Command: "unrelated"},
		}))
		Expect(snapshot.Subtree(9000)).To(Equal(snapshot.All()[:3]))
		Expect(snapshot.Ancestors(9002)).To(Equal([]Process{snapshot.All()[1], snapshot.All()[0]}))
		Expect(snapshot.AggregateSubtree(9000)).To(Equal(ResourceUsage{
			CPUPercent: 8, MemoryPercent: 10, RSSBytes: 7 * 1024 * 1024, Processes: snapshot.All()[:3],
		}))
	})

	It("returns empty results for an unknown pid", func() {
		snapshot := parseSnapshot([]byte(processFixture), time.UTC)
		_, found := snapshot.Get(42)
		Expect(found).To(BeFalse())
		Expect(snapshot.Subtree(42)).To(BeEmpty())
		Expect(snapshot.Ancestors(42)).To(BeEmpty())
		Expect(snapshot.AggregateSubtree(42)).To(Equal(ResourceUsage{}))
	})

	It("stops tree walks when the process graph contains a cycle", func() {
		snapshot := parseSnapshot([]byte(`8000 8001 1 1 1 R Mon Sep 8 10:00:00 2026 first
8001 8000 1 1 1 R Mon Sep 8 10:00:00 2026 second`), time.UTC)
		Expect(snapshot.Subtree(8000)).To(HaveLen(2))
		Expect(snapshot.Ancestors(8000)).To(Equal([]Process{snapshot.All()[1]}))
	})

	It("omits environment data unless capture is requested", func() {
		encoded, err := json.Marshal(Process{PID: 9000})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(encoded)).NotTo(ContainSubstring("environment"))
	})

	It("filters environment variables by exact key and prefix", func() {
		environment := filterEnvironment([]string{
			"CMUX_SURFACE_ID=surface-1",
			"CLAUDE_CODE_SESSION_ID=",
			"CODEX_VALUE=left=right",
			"SECRET_TOKEN=hidden",
		}, EnvironmentOptions{Keys: []string{"CLAUDE_CODE_SESSION_ID"}, Prefixes: []string{"CMUX_", "CODEX_"}})

		Expect(environment).To(Equal(map[string]string{
			"CMUX_SURFACE_ID":        "surface-1",
			"CLAUDE_CODE_SESSION_ID": "",
			"CODEX_VALUE":            "left=right",
		}))
	})

	It("captures every variable when no environment selector is supplied", func() {
		Expect(filterEnvironment([]string{"A=1", "B=two=parts"}, EnvironmentOptions{})).To(Equal(map[string]string{
			"A": "1", "B": "two=parts",
		}))
	})
})

func utcTime(year int, month time.Month, day, hour, minute int) *time.Time {
	value := time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
	return &value
}
