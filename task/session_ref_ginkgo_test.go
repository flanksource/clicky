package task_test

import (
	"encoding/json"

	"github.com/flanksource/clicky/task"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Session references", func() {
	ref := task.SessionRef{Type: "captain", ID: "session-1", Href: "/api/sessions/session-1"}

	It("snapshots a task's session, marks it dirty, and serializes it under session", func() {
		group := task.StartGroup[bool]("session-task")
		step := group.StartStep("execute")
		step.Task().PopDirty()
		step.Task().SetSession(ref)
		Expect(step.Task().PopDirty()).To(BeTrue())

		snapshot := task.SnapshotTask(step.Task(), group.Group)
		Expect(snapshot.Session).To(Equal(&ref))
		data, err := json.Marshal(snapshot)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring(`"session":{"type":"captain","id":"session-1","href":"/api/sessions/session-1"}`))
		step.Finish(nil)
	})

	It("lifts a group's session into its snapshot and RunMeta", func() {
		group := task.StartGroup[bool]("session-group")
		group.Group.SetSession(ref)
		Expect(task.RunMetaFromSnapshot(task.SnapshotGroup(group.Group)).Session).To(Equal(&ref))
	})

	It("omits the session of a task that has none", func() {
		group := task.StartGroup[bool]("session-none")
		step := group.StartStep("execute")
		Expect(task.SnapshotTask(step.Task(), group.Group).Session).To(BeNil())
		step.Finish(nil)
	})

	DescribeTable("rejects a session without a type or id",
		func(invalid task.SessionRef) {
			group := task.StartGroup[bool]("session-invalid")
			step := group.StartStep("execute")
			Expect(func() { step.Task().SetSession(invalid) }).To(Panic())
			Expect(func() { group.Group.SetSession(invalid) }).To(Panic())
			step.Finish(nil)
		},
		Entry("missing type", task.SessionRef{ID: "session-1"}),
		Entry("missing id", task.SessionRef{Type: "captain"}),
	)
})
