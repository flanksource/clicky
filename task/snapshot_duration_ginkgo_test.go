package task_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/task"
)

// Snapshots carry timestamps, never an elapsed value: a stored duration of a
// running task changes on every read, so a stream comparing frames would see a
// change every tick. Duration() derives the value at the moment it is asked.
var _ = Describe("snapshot durations", func() {
	const (
		startedAt  = "2026-11-05T10:00:00Z"
		finishedAt = "2026-11-05T10:00:01.5Z"
	)
	finishedElapsed := 1500 * time.Millisecond

	type timestamps func(startedAt, finishedAt string) time.Duration
	snapshotDuration := func(startedAt, finishedAt string) time.Duration {
		return task.TaskSnapshot{StartedAt: startedAt, FinishedAt: finishedAt}.Duration()
	}
	runDuration := func(startedAt, finishedAt string) time.Duration {
		return task.RunMeta{StartedAt: startedAt, FinishedAt: finishedAt}.Duration()
	}

	DescribeTable("derives the elapsed time from startedAt and finishedAt",
		func(duration timestamps) {
			Expect(duration("", "")).To(BeZero(), "not started")
			Expect(duration(startedAt, finishedAt)).To(Equal(finishedElapsed), "finished")

			const runningFor = time.Minute
			running := time.Now().Add(-runningFor).UTC().Format(time.RFC3339Nano)
			Expect(duration(running, "")).To(BeNumerically(">=", runningFor), "running")
			Expect(duration(running, "")).To(BeNumerically("<", runningFor+5*time.Second), "running")
		},
		Entry("TaskSnapshot", timestamps(snapshotDuration)),
		Entry("RunMeta", timestamps(runDuration)),
	)

	DescribeTable("panics on a timestamp that is not RFC3339",
		func(duration timestamps) {
			Expect(func() { duration("yesterday", "") }).To(PanicWith(ContainSubstring(`startedAt "yesterday"`)))
			Expect(func() { duration(startedAt, "later") }).To(PanicWith(ContainSubstring(`finishedAt "later"`)))
		},
		Entry("TaskSnapshot", timestamps(snapshotDuration)),
		Entry("RunMeta", timestamps(runDuration)),
	)

	It("serialises a finished task with finishedAt and no duration", func() {
		run := task.StartManagedRun("timestamps only")
		run.Finish(task.StatusSuccess, nil)

		var snapshot task.TaskSnapshot
		for _, candidate := range task.SnapshotByID(run.ID()) {
			if candidate.Type == "task" {
				snapshot = candidate
			}
		}
		payload, err := json.Marshal(snapshot)
		Expect(err).NotTo(HaveOccurred())
		var fields map[string]any
		Expect(json.Unmarshal(payload, &fields)).To(Succeed())
		Expect(fields).To(HaveKey("startedAt"))
		Expect(fields).To(HaveKey("finishedAt"))
		Expect(fields).NotTo(HaveKey("duration"))
		Expect(snapshot.Duration()).To(BeNumerically(">", 0))
	})
})
