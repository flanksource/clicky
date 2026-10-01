package task_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Group steps", func() {
	stepSnapshot := func(group task.TypedGroup[bool], name string) task.TaskSnapshot {
		GinkgoHelper()
		for _, snapshot := range task.SnapshotByID(group.ID()) {
			if snapshot.Type == "task" && snapshot.Name == name {
				return snapshot
			}
		}
		Fail(fmt.Sprintf("group %s has no task %q", group.ID(), name))
		return task.TaskSnapshot{}
	}

	It("shows work a running task does inline as its own running task of the group", func() {
		group := task.StartGroup[bool]("index with dependencies", task.WithConcurrency(1))
		release, stepped := make(chan struct{}), make(chan *task.Step, 1)
		owner := group.Add("example.org/app", func(commonscontext.Context, *task.Task) (bool, error) {
			step := group.StartStep("local dependency example.org/lib")
			stepped <- step
			<-release
			step.Finish(errors.New("parse failed"))
			return true, nil
		})
		var step *task.Step
		Eventually(stepped).Should(Receive(&step))

		Expect(stepSnapshot(group, "local dependency example.org/lib").Status).To(Equal(string(task.StatusRunning)),
			"the step runs on its owner's goroutine, so group concurrency 1 does not hold it back")
		close(release)
		_, err := owner.GetResult()
		Expect(err).ToNot(HaveOccurred())

		failed := stepSnapshot(group, "local dependency example.org/lib")
		Expect([]string{failed.Status, failed.Error}).To(Equal([]string{string(task.StatusFailed), "parse failed"}))
		Expect(group.WaitFor().Status).To(Equal(task.StatusFailed), "a failed step fails its group")
		Expect(step.Task().ID()).To(Equal(failed.ID))
	})

	DescribeTable("records the outcome of the first Finish",
		func(first error, status task.Status) {
			group := task.StartGroup[bool]("one step")
			step := group.StartStep("step")

			step.Finish(first)
			step.Finish(errors.New("finished again"))

			Expect(stepSnapshot(group, "step").Status).To(Equal(string(status)))
			Expect(group.WaitFor().Status).To(Equal(status))
		},
		Entry("success", nil, task.StatusSuccess),
		Entry("a cancelled context", fmt.Errorf("load: %w", context.Canceled), task.StatusCancelled),
		Entry("a failure", errors.New("boom"), task.StatusFailed),
	)
})
