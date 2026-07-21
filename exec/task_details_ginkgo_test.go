package exec

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/task"
)

var _ = Describe("Exec task details", func() {
	It("projects ExecResult output into an attached task group", func() {
		group := task.StartGroup[ExecResult]("commit", task.WithKind("gavel-commit"))
		process := NewExec("printf stdout; printf stderr >&2")
		handle := process.RunAsTask("create commit", task.WithGroup(group.Group))

		result, err := handle.GetResult()
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Stdout).To(Equal("stdout"))
		Expect(result.Stderr).To(Equal("stderr"))

		snapshots := task.SnapshotByID(group.ID())
		Expect(snapshots).To(HaveLen(2))
		Expect(snapshots[1].Stdout).To(Equal("stdout"))
		Expect(snapshots[1].Stderr).To(Equal("stderr"))
		Expect(snapshots[1].Details).ToNot(BeNil())
	})

	It("propagates task cancellation to the subprocess tree", func() {
		process := NewExec("sleep 30").WithProcessGroup()
		handle := process.StartAsTask("long command")
		Eventually(process.Pid, 5*time.Second).Should(BeNumerically(">", 0))

		handle.Cancel()
		Eventually(process.IsRunning, 5*time.Second).Should(BeFalse())
		Expect(handle.WaitFor().Status).To(Equal(task.StatusCancelled))
	})
})
