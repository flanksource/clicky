package task_test

import (
	"github.com/flanksource/clicky/task"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Structured progress metrics", func() {
	It("retains zero progress, transfer metrics and independent task labels", func() {
		group := task.StartGroup[bool]("metrics")
		step := group.StartStep("[acme/rules] Fetch main")
		labels := map[string]string{"repository": "acme/rules"}
		step.Task().SetLabels(labels)
		labels["repository"] = "changed"
		step.Task().SetProgress(0, 10)
		step.Task().SetProgressMetrics(task.ProgressMetrics{Unit: "objects", Bytes: 1024, BytesPerSecond: 512})
		snapshot := task.SnapshotTask(step.Task(), group.Group)
		Expect(snapshot.Labels).To(Equal(map[string]string{"repository": "acme/rules"}))
		Expect(snapshot.ProgressMetrics).To(Equal(&task.ProgressMetrics{Unit: "objects", Bytes: 1024, BytesPerSecond: 512}))
		Expect(snapshot.MaxValue).To(Equal(10))
		Expect(step.Task().Pretty().String()).To(ContainSubstring("objects"))
		step.Finish(nil)
	})
})
