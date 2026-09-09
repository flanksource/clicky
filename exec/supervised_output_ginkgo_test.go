//go:build unix

package exec

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/task"
)

var _ = Describe("Supervised process output", func() {
	// A bound task outlives every child it supervises, so the output it shows has
	// to outlive them too. Reading the child's own capture instead empties the
	// pane on each restart — and a viewer watching a stdout tab sees it vanish.
	It("carries output across a restart and marks the boundary", func() {
		// Resolved here rather than inside the cleanup: GinkgoT().Context()
		// registers a DeferCleanup of its own, which Ginkgo forbids from within one.
		ctx := GinkgoT().Context()
		group := task.StartGroup[ExecResult]("restarting agent")
		handle := NewExec("sh", "-c", "echo generation; sleep 5").WithProcessGroup().RunSupervisedAsTask(
			RunSupervisedTaskOptions{
				Name:      "restarting",
				Supervise: SuperviseOptions{Limits: ResourceLimits{Interval: 20 * time.Millisecond}},
				Task:      []task.Option{task.WithGroup(group.Group)},
			},
		)
		// The process task appears a moment after the group, so poll defensively
		// rather than indexing into a list that is briefly one entry long.
		snapshot := func() task.TaskSnapshot {
			snapshots := task.SnapshotByID(group.ID())
			if len(snapshots) < 2 {
				return task.TaskSnapshot{}
			}
			return snapshots[1]
		}
		Eventually(func() string { return snapshot().Stdout }, 5*time.Second).Should(Equal("generation\n"))

		taskID := snapshot().ID
		stop := func() {
			_ = task.ControlTask(ctx, group.ID(), taskID, task.ControlStop)
			_, _ = handle.GetResult()
		}
		DeferCleanup(stop)

		Expect(task.ControlTask(ctx, group.ID(), taskID, task.ControlRestart)).To(Succeed())

		Eventually(func() int {
			return strings.Count(snapshot().Stdout, "generation\n")
		}, 10*time.Second).Should(Equal(2), "the restarted generation appends to what the first one wrote")
		Expect(snapshot().Stdout).To(ContainSubstring("── restarted"))
		Expect(snapshot().StdoutOffset).To(BeZero(), "nothing was discarded, so the tail is the whole stream")
		Expect(snapshot().StdoutTruncated).To(BeFalse())
	})

	It("reports the discarded prefix once the retained output rolls over", func() {
		const (
			lines     = 40
			lineWidth = 255 // plus the newline echo adds
			marker    = "TAIL-MARKER"
		)
		label := fmt.Sprintf("rollover-%d", time.Now().UnixNano())
		supervisor := NewExec("sh", "-c", fmt.Sprintf(
			"for i in $(seq 1 %d); do echo %s; done; echo %s", lines, strings.Repeat("y", lineWidth), marker,
		)).WithProcessGroup().Supervise(SuperviseOptions{
			CaptureLimit: 1024,
			Limits:       ResourceLimits{Interval: 20 * time.Millisecond},
			Task:         SupervisedTaskOptions{Labels: map[string]string{"spec": label}},
		})
		supervisor.Start()
		supervisor.Wait()

		runs := task.Runs(task.RunFilter{Labels: map[string]string{"spec": label}})
		Expect(runs).To(HaveLen(1))
		snapshot := task.SnapshotByID(runs[0].ID)[1]

		Expect(snapshot.Stdout).To(HaveSuffix(marker + "\n"))
		Expect(len(snapshot.Stdout)).To(BeNumerically("<=", 1024))
		Expect(snapshot.StdoutTruncated).To(BeTrue())
		Expect(snapshot.StdoutOffset+int64(len(snapshot.Stdout))).To(
			BeEquivalentTo(lines*(lineWidth+1)+len(marker)+1),
			"the offset accounts for every byte the process wrote, discarded ones included",
		)
	})
})
