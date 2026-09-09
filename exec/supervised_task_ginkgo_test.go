//go:build unix

package exec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	It("never starts the process when the task context is already cancelled", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "started")
		handle := NewExec("touch", marker).WithoutShell().RunSupervisedAsTask(
			RunSupervisedTaskOptions{
				Name: "cancelled-before-start",
				Task: []task.Option{task.WithTaskTimeout(time.Nanosecond)},
			},
		)

		result, err := handle.GetResult()

		Expect(err).To(HaveOccurred())
		Expect(result.Status).To(Equal("cancelled"))
		Expect(marker).ToNot(BeAnExistingFile())
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

	It("preserves a caller-supplied retry policy", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "attempts")
		handle := NewExec("sh", "-c", fmt.Sprintf("echo attempt >> %q; exit 1", marker)).WithProcessGroup().RunSupervisedAsTask(
			RunSupervisedTaskOptions{
				Name: "retry supervised process",
				Task: []task.Option{task.WithRetryConfig(task.RetryConfig{
					RetryableErrors: []string{""},
					MaxRetries:      1,
				})},
			},
		)

		_, err := handle.GetResult()
		Expect(err).To(HaveOccurred())
		contents, err := os.ReadFile(marker)
		Expect(err).ToNot(HaveOccurred())
		Expect(strings.Count(string(contents), "attempt")).To(Equal(2))
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

	It("re-evaluates caller annotations on every snapshot", func() {
		phase := "generate"
		group := task.StartGroup[ExecResult]("annotated agent")
		handle := NewExec("sh", "-c", "sleep 0.3").WithProcessGroup().RunSupervisedAsTask(
			RunSupervisedTaskOptions{
				Name: "annotated",
				Supervise: SuperviseOptions{
					Limits: ResourceLimits{Interval: 20 * time.Millisecond},
					Task: SupervisedTaskOptions{
						Annotations: func() map[string]string {
							return map[string]string{"phase": phase, "model": "test-model"}
						},
					},
				},
				Task: []task.Option{task.WithGroup(group.Group)},
			},
		)
		// The process task appears a moment after the group, so poll defensively
		// rather than indexing into a list that is briefly one entry long.
		annotations := func() map[string]string {
			snapshots := task.SnapshotByID(group.ID())
			if len(snapshots) < 2 {
				return nil
			}
			details, ok := snapshots[1].Details.(ProcessDetails)
			if !ok {
				return nil
			}
			return details.Annotations
		}

		Eventually(annotations, 5*time.Second).Should(HaveKeyWithValue("phase", "generate"))
		Expect(annotations()).To(HaveKeyWithValue("model", "test-model"))
		phase = "verify"
		Expect(annotations()).To(HaveKeyWithValue("phase", "verify"))

		_, err := handle.GetResult()
		Expect(err).ToNot(HaveOccurred())
	})

	It("omits annotations when the caller supplies none", func() {
		group := task.StartGroup[ExecResult]("unannotated agent")
		handle := NewExec("sh", "-c", "echo plain").WithProcessGroup().RunSupervisedAsTask(
			RunSupervisedTaskOptions{
				Name: "unannotated",
				Task: []task.Option{task.WithGroup(group.Group)},
			},
		)

		_, err := handle.GetResult()

		Expect(err).ToNot(HaveOccurred())
		Expect(task.SnapshotByID(group.ID())[1].Details.(ProcessDetails).Annotations).To(BeNil())
	})

	It("bounds the retained capture so a long-lived process cannot grow it without limit", func() {
		// 4 KiB per line keeps the write count low while overflowing the cap.
		line := strings.Repeat("y", 4095)
		supervisor := NewExec("sh", "-c", fmt.Sprintf("for i in $(seq 1 40); do echo %s; done; echo TAIL-MARKER", line)).
			WithProcessGroup().
			Supervise(SuperviseOptions{
				CaptureLimit: 8192,
				Limits:       ResourceLimits{Interval: 20 * time.Millisecond},
			})
		supervisor.Start()

		Eventually(func() string { return supervisor.Result().Stdout }, 5*time.Second).Should(HaveSuffix("TAIL-MARKER\n"))
		Expect(len(supervisor.Result().Stdout)).To(BeNumerically("<=", 8192))
	})
})

var _ = Describe("Supervised process resource limits", func() {
	It("serializes limits in camelCase with a human-readable interval", func() {
		payload, err := json.Marshal(ResourceLimits{
			MaxRSSBytes:    1024,
			MaxCPUPercent:  85,
			CPUSampleCount: 4,
			Interval:       2 * time.Second,
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(string(payload)).To(Equal(`{"maxRssBytes":1024,"maxCpuPercent":85,"cpuSampleCount":4,"interval":"2s"}`))
	})

	It("omits every unset limit", func() {
		payload, err := json.Marshal(ResourceLimits{})

		Expect(err).ToNot(HaveOccurred())
		Expect(string(payload)).To(Equal("{}"))
	})
})
