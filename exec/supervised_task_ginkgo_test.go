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

var _ = Describe("Supervised process task runs", func() {
	It("runs a supervised process inside a caller-owned task group", func() {
		group := task.StartGroup[ExecResult]("slice model")
		handle := NewExec("sh", "-c", "echo sliced; sleep 0.2").WithProcessGroup().RunSupervisedAsTask(
			RunSupervisedTaskOptions{
				Name:      "run OrcaSlicer",
				Supervise: SuperviseOptions{Limits: ResourceLimits{Interval: 20 * time.Millisecond}},
				Task:      []task.Option{task.WithGroup(group.Group)},
			},
		)

		result, err := handle.GetResult()

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Stdout).To(ContainSubstring("sliced"))
		snapshots := task.SnapshotByID(group.ID())
		Expect(snapshots).To(HaveLen(2))
		Expect(snapshots[1].Details).To(BeAssignableToTypeOf(ProcessDetails{}))
		details := snapshots[1].Details.(ProcessDetails)
		Expect(details.Command).To(Equal("sh"))
		Expect(details.Args).To(Equal([]string{"-c", "echo sliced; sleep 0.2"}))
		Expect(details.Metrics).To(HaveKeyWithValue("rss", task.MetricID(handle.GetTask().ID(), "rss")))
		Expect(details.Peak.VMSBytes).To(BeNumerically(">", 0))
	})

	It("returns a terminal failed status for a process that cannot start", func() {
		handle := NewExec("echo", "started").WithoutShell().WithCwd("/nonexistent-dir-for-exec-tests").RunSupervisedAsTask(
			RunSupervisedTaskOptions{Name: "invalid-executable"},
		)

		result, err := handle.GetResult()

		Expect(err).To(HaveOccurred())
		Expect(result.IsPending()).To(BeFalse())
		Expect(result.Status).To(Equal("failed"))
	})

	It("creates a separate, output-isolated task run for every automatic generation", func() {
		label := fmt.Sprintf("generation-%d", time.Now().UnixNano())
		supervisor := NewExec("echo generation=$$; exit 7").WithProcessGroup().Supervise(SuperviseOptions{
			RestartPolicy: RestartOnFailure,
			MaxRestarts:   1,
			Task: SupervisedTaskOptions{
				Labels: map[string]string{"spec": label},
			},
		})
		supervisor.Start()
		supervisor.Wait()

		runs := task.Runs(task.RunFilter{Kind: "supervised-process", Labels: map[string]string{"spec": label}})
		Expect(runs).To(HaveLen(2))
		Expect(runs[0].ID).ToNot(Equal(runs[1].ID))
		for _, run := range runs {
			snapshots := task.SnapshotByID(run.ID)
			Expect(snapshots).To(HaveLen(2))
			Expect(strings.Count(snapshots[1].Stdout, "generation=")).To(Equal(1))
			Expect(snapshots[0].Details).ToNot(BeNil())
			Expect(snapshots[0].Status).To(Equal(string(task.StatusFailed)))
		}
	})

	It("resets metrics per manual generation and exposes stop and restart controls", func() {
		label := fmt.Sprintf("controls-%d", time.Now().UnixNano())
		supervisor := NewExec("while :; do sleep 1; done").WithProcessGroup().Supervise(SuperviseOptions{
			Limits: ResourceLimits{Interval: 50 * time.Millisecond},
			Task: SupervisedTaskOptions{
				Labels: map[string]string{"spec": label},
			},
		})
		supervisor.Start()
		DeferCleanup(supervisor.Stop)

		var first task.RunMeta
		Eventually(func() int {
			runs := task.Runs(task.RunFilter{Labels: map[string]string{"spec": label}})
			if len(runs) == 1 {
				first = runs[0]
			}
			return len(runs)
		}, 5*time.Second).Should(Equal(1))
		Expect(first.Controls).To(ContainElements(task.ControlStop, task.ControlRestart))
		Eventually(supervisor.Resources, 5*time.Second).ShouldNot(Equal(ResourceSnapshot{}))
		EventallyDetails := func() ProcessDetails {
			return task.SnapshotByID(first.ID)[0].Details.(ProcessDetails)
		}
		Eventually(func() uint64 { return EventallyDetails().Latest.VMSBytes }, 5*time.Second).Should(BeNumerically(">", 0))
		Expect(EventallyDetails().Metrics).To(HaveKeyWithValue("vms", task.MetricID(first.ID, "vms")))

		Expect(task.ControlRun(GinkgoT().Context(), first.ID, task.ControlRestart)).To(Succeed())
		Eventually(func() int {
			return len(task.Runs(task.RunFilter{Labels: map[string]string{"spec": label}}))
		}, 5*time.Second).Should(Equal(2))
		firstSnapshot := task.SnapshotByID(first.ID)
		Expect(firstSnapshot[0].Status).To(Equal(string(task.StatusCancelled)))

		latest := task.Runs(task.RunFilter{Labels: map[string]string{"spec": label}})[0]
		Expect(task.ControlRun(GinkgoT().Context(), latest.ID, task.ControlStop)).To(Succeed())
		Eventually(func() string {
			return task.SnapshotByID(latest.ID)[0].Status
		}, 5*time.Second).Should(Equal(string(task.StatusCancelled)))
	})
})
