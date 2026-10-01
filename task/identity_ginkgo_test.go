package task_test

import (
	"sync/atomic"
	"time"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task identity", func() {
	It("hands a second caller of a running identity the running task and its result", func() {
		identity := "identity-" + uuid.NewString()
		release := make(chan struct{})
		var invocations atomic.Int32
		work := func(commonscontext.Context, *task.Task) (string, error) {
			invocations.Add(1)
			<-release
			return "indexed once", nil
		}
		first := task.StartTask("first caller", work, task.WithIdentity(identity))
		Eventually(first.Status).Should(Equal(task.StatusRunning))

		second := task.StartTask("second caller", work, task.WithIdentity(identity))
		close(release)

		results := make(chan string, 2)
		for _, handle := range []task.TypedTask[string]{first, second} {
			go func(handle task.TypedTask[string]) {
				defer GinkgoRecover()
				result, err := handle.GetResult()
				Expect(err).ToNot(HaveOccurred())
				results <- result
			}(handle)
		}
		Eventually(results, 5*time.Second).Should(Receive(Equal("indexed once")))
		Eventually(results, 5*time.Second).Should(Receive(Equal("indexed once")))
		Expect(second.ID()).To(Equal(first.ID()))
		Expect(invocations.Load()).To(Equal(int32(1)))
	})

	It("does not attach a duplicate of a running identity to the caller's group", func() {
		identity := "identity-" + uuid.NewString()
		release := make(chan struct{})
		running := task.StartTask("owner", func(commonscontext.Context, *task.Task) (bool, error) {
			<-release
			return true, nil
		}, task.WithIdentity(identity))
		Eventually(running.Status).Should(Equal(task.StatusRunning))
		group := task.StartGroup[bool]("joining group")

		joined := group.Add("duplicate", func(commonscontext.Context, *task.Task) (bool, error) {
			return false, nil
		}, task.WithIdentity(identity))
		close(release)

		Expect(joined.ID()).To(Equal(running.ID()))
		Expect(group.GetTasks()).To(BeEmpty())
		result, err := joined.GetResult()
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(BeTrue())
	})
})

var _ = Describe("Group identity", func() {
	It("returns the running group to a second StartGroup with the same identity", func() {
		identity := "group-" + uuid.NewString()
		release := make(chan struct{})
		first := task.StartGroup[bool]("first run", task.WithGroupIdentity(identity))
		first.Add("work", func(commonscontext.Context, *task.Task) (bool, error) {
			<-release
			return true, nil
		})

		second := task.StartGroup[bool]("second run", task.WithGroupIdentity(identity))

		Expect([]any{second.ID(), first.Joined(), second.Joined()}).To(Equal([]any{first.ID(), false, true}))
		close(release)
		Expect(second.WaitFor().Status).To(Equal(task.StatusSuccess))
	})

	It("starts a new group once the group holding the identity has finished", func() {
		identity := "group-" + uuid.NewString()
		first := task.StartGroup[bool]("finished run", task.WithGroupIdentity(identity))
		first.Add("work", func(commonscontext.Context, *task.Task) (bool, error) { return true, nil })
		Expect(first.WaitFor().Status).To(Equal(task.StatusSuccess))

		next := task.StartGroup[bool]("next run", task.WithGroupIdentity(identity))

		Expect(next.ID()).ToNot(Equal(first.ID()))
		Expect(next.Joined()).To(BeFalse())
	})

	It("starts a new group once the group holding the identity was cancelled before any work", func() {
		identity := "group-" + uuid.NewString()
		first := task.StartGroup[bool]("cancelled run", task.WithGroupIdentity(identity))
		first.Cancel()

		next := task.StartGroup[bool]("next run", task.WithGroupIdentity(identity))

		Expect(next.ID()).ToNot(Equal(first.ID()))
	})
})
