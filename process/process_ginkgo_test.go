package process

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gops "github.com/shirou/gopsutil/v4/process"
)

const processFixture = `
9000 1 1.5 2.0 1024 S 0:01.50 Mon Sep  8 10:00:00 2026 501 acme agent --session-id root
9001 9000 2.5 3.0 2048 R 00:00:02 Mon Sep  8 10:01:00 2026 501 acme browser child
9002 9001 4.0 5.0 4096 Z 1-00:00:00 Mon Sep  8 10:02:00 2026 501 acme browser grandchild
9100 1 0.5 1.0 512 T 0:00.00 Mon Sep  8 10:03:00 2026 260 _acmed unrelated
`

var _ = Describe("process snapshots", func() {
	It("indexes, traverses, and aggregates one stable host snapshot", func() {
		snapshot := mustParseSnapshot(processFixture)

		Expect(snapshot.All()).To(Equal([]Process{
			{PID: 9000, PPID: 1, Status: "sleeping", Active: true, CPUPercent: 1.5, CPUTime: 1500 * time.Millisecond, MemoryPercent: 2, RSSBytes: 1024 * 1024, StartedAt: utcTime("2026-09-08T10:00:00Z"), UID: 501, User: "acme", Command: "agent --session-id root"},
			{PID: 9001, PPID: 9000, Status: "active", Active: true, CPUPercent: 2.5, CPUTime: 2 * time.Second, MemoryPercent: 3, RSSBytes: 2048 * 1024, StartedAt: utcTime("2026-09-08T10:01:00Z"), UID: 501, User: "acme", Command: "browser child"},
			{PID: 9002, PPID: 9001, Status: "zombie", CPUPercent: 4, CPUTime: 24 * time.Hour, MemoryPercent: 5, RSSBytes: 4096 * 1024, StartedAt: utcTime("2026-09-08T10:02:00Z"), UID: 501, User: "acme", Command: "browser grandchild"},
			{PID: 9100, PPID: 1, Status: "stopped", CPUPercent: .5, MemoryPercent: 1, RSSBytes: 512 * 1024, StartedAt: utcTime("2026-09-08T10:03:00Z"), UID: 260, User: "_acmed", Command: "unrelated"},
		}))
		Expect(snapshot.Subtree(9000)).To(Equal(snapshot.All()[:3]))
		Expect(snapshot.Ancestors(9002)).To(Equal([]Process{snapshot.All()[1], snapshot.All()[0]}))
		Expect(snapshot.AggregateSubtree(9000)).To(Equal(ResourceUsage{
			CPUPercent: 8, MemoryPercent: 10, RSSBytes: 7 * 1024 * 1024, Processes: snapshot.All()[:3],
		}))
	})

	It("returns empty results for an unknown pid", func() {
		snapshot := mustParseSnapshot(processFixture)
		_, found := snapshot.Get(42)
		Expect(found).To(BeFalse())
		Expect(snapshot.Subtree(42)).To(BeEmpty())
		Expect(snapshot.Ancestors(42)).To(BeEmpty())
		Expect(snapshot.AggregateSubtree(42)).To(Equal(ResourceUsage{}))
	})

	It("stops tree walks when the process graph contains a cycle", func() {
		snapshot := mustParseSnapshot(`8000 8001 1 1 1 R 0:00.00 Mon Sep 8 10:00:00 2026 501 acme first
8001 8000 1 1 1 R 0:00.00 Mon Sep 8 10:00:00 2026 501 acme second`)
		Expect(snapshot.Subtree(8000)).To(HaveLen(2))
		Expect(snapshot.Ancestors(8000)).To(Equal([]Process{snapshot.All()[1]}))
	})

	It("leaves the ps it runs out of the caller's own subtree", func() {
		snapshot, err := Discover(context.Background(), SnapshotOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshot.Subtree(os.Getpid())).NotTo(ContainElement(HaveField("Command", HavePrefix("ps -eo "+psFormat))))
		self, found := snapshot.Get(os.Getpid())
		Expect(found).To(BeTrue(), "the caller itself is still listed")
		Expect(self.PID).To(Equal(os.Getpid()))
		owner, err := user.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect([]any{self.UID, self.User}).To(Equal([]any{os.Getuid(), owner.Username}), "the caller is listed with its owner")
	})

	It("omits environment data unless capture is requested", func() {
		encoded, err := json.Marshal(Process{PID: 9000})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(encoded)).NotTo(ContainSubstring("environment"))
	})

	It("populates disk counters for a selected process without losing a measured zero", func() {
		snapshot := NewSnapshot([]Process{{PID: os.Getpid(), Active: true}})
		Expect(snapshot.PopulateIO(context.Background(), []int{os.Getpid()})).To(Succeed())
		observed, found := snapshot.Get(os.Getpid())
		Expect(found).To(BeTrue())
		Expect(observed.IO).NotTo(BeNil())
	})

	It("leaves disk counters unknown for a process that has exited but not been reaped", func() {
		child := exec.Command("sh", "-c", "exit 0")
		Expect(child.Start()).To(Succeed())
		DeferCleanup(func() { _ = child.Wait() })
		var zombie Process
		Eventually(func() string {
			snapshot, err := Discover(context.Background(), SnapshotOptions{})
			Expect(err).NotTo(HaveOccurred())
			zombie, _ = snapshot.Get(child.Process.Pid)
			return zombie.Status
		}).WithTimeout(5 * time.Second).Should(Equal("zombie"))

		snapshot := NewSnapshot([]Process{zombie})
		Expect(snapshot.PopulateIO(context.Background(), []int{zombie.PID})).To(Succeed())
		observed, _ := snapshot.Get(zombie.PID)
		Expect(observed.IO).To(BeNil())
	})

	DescribeTable("records the errors the kernel gives about one process as its reading, replacing stale readings on every read",
		func(permission error) {
			const unreadable, readable = 9000, 9001
			snapshot := NewSnapshot([]Process{{PID: unreadable, IO: &ProcessIO{DiskReadBytes: 10}}, {PID: readable}})
			readErr := fmt.Errorf("disk counters: %w", permission)
			readIO := func(_ context.Context, pid int) (*gops.IOCountersStat, error) {
				if pid == unreadable && readErr != nil {
					return nil, readErr
				}
				return &gops.IOCountersStat{DiskReadBytes: 20}, nil
			}
			Expect(snapshot.populateIO(context.Background(), []int{unreadable, readable}, readIO)).To(Succeed())
			observed, _ := snapshot.Get(unreadable)
			Expect(observed.IO).To(BeNil())
			Expect(observed.IOError).To(Equal("read disk I/O: " + readErr.Error()))
			other, _ := snapshot.Get(readable)
			Expect(other.IO).To(Equal(&ProcessIO{DiskReadBytes: 20}))

			readErr = nil
			Expect(snapshot.populateIO(context.Background(), []int{unreadable}, readIO)).To(Succeed())
			observed, _ = snapshot.Get(unreadable)
			Expect(observed.IO).To(Equal(&ProcessIO{DiskReadBytes: 20}))
			Expect(observed.IOError).To(BeEmpty())
		},
		Entry("OS permission denied", os.ErrPermission),
		Entry("gopsutil operation not permitted", gops.ErrorNotPermitted),
		Entry("another errno proc_pid_rusage gives for the process", fmt.Errorf("proc_pid_rusage failed for pid 9000: %w", syscall.ETIMEDOUT)),
	)

	It("fails the snapshot for an error that is no answer about one process", func() {
		snapshot := NewSnapshot([]Process{{PID: 9000}})
		broken := errors.New("load libproc: symbol not found")
		readIO := func(context.Context, int) (*gops.IOCountersStat, error) { return nil, broken }

		Expect(snapshot.populateIO(context.Background(), []int{9000}, readIO)).To(MatchError(broken))
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

func mustParseSnapshot(output string) *Snapshot {
	GinkgoHelper()
	snapshot, err := parseSnapshot([]byte(output), time.UTC)
	Expect(err).NotTo(HaveOccurred())
	return snapshot
}

func utcTime(rfc3339 string) *time.Time {
	value, err := time.Parse(time.RFC3339, rfc3339)
	Expect(err).NotTo(HaveOccurred())
	return &value
}
