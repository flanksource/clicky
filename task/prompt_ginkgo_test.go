package task_test

import (
	"context"
	"time"

	"github.com/flanksource/clicky/prompt"
	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task prompts", func() {
	// installManager installs a fresh process-wide prompt manager for one spec.
	// Specs keep the returned value rather than a shared variable, so a task still
	// blocked after a failure never races the next spec's install.
	installManager := func() *prompt.Manager {
		previous := prompt.GlobalManager()
		manager := prompt.NewManager(prompt.NewMemory(prompt.MemoryConfig{}))
		prompt.SetDefault(manager)
		DeferCleanup(func() { prompt.SetDefault(previous) })
		return manager
	}

	taskPrompts := func(groupID, taskID string) []prompt.PromptSnapshot {
		for _, snap := range task.SnapshotByID(groupID) {
			if snap.Type == "task" && snap.ID == taskID {
				return snap.Prompts
			}
		}
		return nil
	}

	choose := func(manager *prompt.Manager, title string) func(commonscontext.Context, *task.Task) (int, error) {
		return func(ctx commonscontext.Context, _ *task.Task) (int, error) {
			indexes, ok := manager.Select(ctx, title, []string{"Remove", "Cancel"}, prompt.SelectOptions{})
			if !ok {
				return -1, nil
			}
			return indexes[0], nil
		}
	}

	It("shows a prompt raised inside a running task on that task's snapshot until it is answered", func() {
		manager := installManager()
		owner := "todo-" + uuid.NewString()
		outer := prompt.WithScope(context.Background(), prompt.Scope{Owner: owner, Labels: map[string]string{"session": "s1"}})
		group := task.StartGroup[int]("commit " + uuid.NewString())
		asking := group.Add("remove stale lock", choose(manager, "Remove stale lock?"), task.WithContext(outer))

		Eventually(func() []prompt.PromptSnapshot { return taskPrompts(group.ID(), asking.ID()) }, 5*time.Second).Should(HaveLen(1))
		pending := taskPrompts(group.ID(), asking.ID())[0]
		Expect(pending.Title).To(Equal("Remove stale lock?"))
		Expect(pending.State).To(Equal(string(prompt.StatePending)))
		Expect(pending.Owner).To(Equal(owner), "the caller's scope is kept")
		Expect(pending.Labels).To(Equal(map[string]string{"session": "s1", prompt.TaskLabel: asking.ID()}))

		Expect(manager.Resolve(pending.ID, prompt.Answer{Values: map[string]any{"choice": "0"}})).To(Succeed())

		choice, err := asking.GetResult()
		Expect(err).ToNot(HaveOccurred())
		Expect(choice).To(Equal(0))
		Expect(taskPrompts(group.ID(), asking.ID())).To(BeEmpty(), "an answered prompt leaves the task")
	})

	It("attributes a prompt to the innermost running task", func() {
		manager := installManager()
		group := task.StartGroup[int]("nested " + uuid.NewString())
		innerIDs := make(chan string, 1)
		outerTask := group.Add("outer", func(ctx commonscontext.Context, _ *task.Task) (int, error) {
			inner := group.Add("inner", choose(manager, "Inner question?"), task.WithContext(ctx))
			innerIDs <- inner.ID()
			return inner.GetResult()
		})

		var innerID string
		Eventually(innerIDs, 5*time.Second).Should(Receive(&innerID))
		Eventually(func() []prompt.PromptSnapshot { return taskPrompts(group.ID(), innerID) }, 5*time.Second).Should(HaveLen(1))
		Expect(taskPrompts(group.ID(), outerTask.ID())).To(BeEmpty())

		pending := taskPrompts(group.ID(), innerID)[0]
		Expect(pending.Labels).To(HaveKeyWithValue(prompt.TaskLabel, innerID))
		Expect(manager.Resolve(pending.ID, prompt.Answer{Values: map[string]any{"choice": "0"}})).To(Succeed())
		answered, err := outerTask.GetResult()
		Expect(err).ToNot(HaveOccurred())
		Expect(answered).To(Equal(0))
	})
})
