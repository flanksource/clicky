package task_test

import (
	"github.com/flanksource/clicky/task"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task progress audit: native notifications", func() {
	DescribeTable("retains negative counters without panicking", func(value, maximum int) {
		group := task.StartGroup[bool]("negative progress")
		step := group.StartStep("read documents")
		defer step.Finish(nil)
		Expect(func() { step.Task().SetProgress(value, maximum) }).NotTo(Panic())
		gotValue, gotMaximum := step.Task().Progress()
		Expect([]int{gotValue, gotMaximum}).To(Equal([]int{value, maximum}))
	}, Entry("negative value", -1, 10), Entry("negative maximum", 1, -1))

	It("publishes descriptions, counters, and metrics after unlocking the task", func() {
		group := task.StartGroup[bool]("progress notifications")
		var snapshots []task.TaskSnapshot
		step := group.StartStep("read documents", task.WithOnProgress(func() {
			snapshots = append(snapshots, task.SnapshotByID(group.ID())[1])
		}))
		defer step.Finish(nil)
		step.Task().SetDescription("Loading documents")
		step.Task().SetProgress(1, 3)
		step.Task().SetProgressMetrics(task.ProgressMetrics{Unit: "documents"})
		Expect(snapshots).To(HaveLen(3))
		Expect(snapshots[0].Description).To(Equal("Loading documents"))
		Expect([]int{snapshots[1].Progress, snapshots[1].MaxValue}).To(Equal([]int{1, 3}))
		Expect(snapshots[2].ProgressMetrics.Unit).To(Equal("documents"))
	})
})
