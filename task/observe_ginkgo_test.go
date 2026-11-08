package task_test

import (
	"sync"

	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/task"
)

type observedTask struct {
	Phase  task.TaskPhase
	Status task.Status
}

func recordTasks(name string) (events func() []observedTask, release func()) {
	var mu sync.Mutex
	var seen []observedTask
	release = task.ObserveTasks(func(event task.TaskEvent) {
		if event.Task.Name() != name {
			return
		}
		mu.Lock()
		seen = append(seen, observedTask{Phase: event.Phase, Status: event.Status})
		mu.Unlock()
	})
	return func() []observedTask {
		mu.Lock()
		defer mu.Unlock()
		return append([]observedTask(nil), seen...)
	}, release
}

var _ = Describe("ObserveTasks", func() {
	It("delivers each status transition once and progress in order", func() {
		const name = "observed success"
		events, release := recordTasks(name)
		defer release()
		handle := task.StartTask(name, func(_ commonscontext.Context, t *task.Task) (bool, error) {
			t.SetProgress(1, 2)
			return true, nil
		})
		_, err := handle.GetResult()
		Expect(err).ToNot(HaveOccurred())
		Eventually(events).Should(Equal([]observedTask{
			{Phase: task.TaskStatusChanged, Status: task.StatusRunning},
			{Phase: task.TaskProgressed, Status: task.StatusRunning},
			{Phase: task.TaskStatusChanged, Status: task.StatusSuccess},
		}))
		Consistently(events).Should(HaveLen(3))
	})

	It("reports a step cancelled by its context", func() {
		const name = "observed cancel"
		events, release := recordTasks(name)
		defer release()
		group := task.StartGroup[bool]("observed group")
		step := group.StartStep(name)
		step.Task().Cancel()
		step.Finish(nil)
		Expect(events()).To(Equal([]observedTask{
			{Phase: task.TaskStatusChanged, Status: task.StatusRunning},
			{Phase: task.TaskStatusChanged, Status: task.StatusCancelled},
		}))
	})

	It("stops delivering after release", func() {
		const name = "observed released"
		events, release := recordTasks(name)
		release()
		release()
		group := task.StartGroup[bool]("released group")
		group.StartStep(name).Finish(nil)
		Expect(events()).To(BeEmpty())
	})

	It("builds a snapshot of the observed task on demand", func() {
		const name = "observed snapshot"
		var mu sync.Mutex
		var snapshot task.TaskSnapshot
		release := task.ObserveTasks(func(event task.TaskEvent) {
			if event.Task.Name() == name && event.Status == task.StatusSuccess {
				mu.Lock()
				snapshot = event.Snapshot()
				mu.Unlock()
			}
		})
		defer release()
		group := task.StartGroup[bool]("snapshot group")
		group.StartStep(name).Finish(nil)
		mu.Lock()
		defer mu.Unlock()
		Expect(snapshot.Name).To(Equal(name))
		Expect(snapshot.Status).To(Equal(string(task.StatusSuccess)))
		Expect(snapshot.GroupID).To(Equal(group.ID()))
	})
})
