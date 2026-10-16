package task

import (
	"fmt"
	"slices"
	"sync"

	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// runningSet records which members of a group are inside their task body, in
// ascending insertion index.
type runningSet struct {
	mu      sync.Mutex
	members []int
}

func (r *runningSet) enter(i int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members = append(r.members, i)
	slices.Sort(r.members)
}

func (r *runningSet) leave(i int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members = slices.DeleteFunc(r.members, func(m int) bool { return m == i })
}

func (r *runningSet) snapshot() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.members)
}

var _ = Describe("Group concurrency admission order", func() {
	const members = 8

	var original *Manager

	BeforeEach(func() {
		original = global
		global = newTestManager(members + 1)
	})

	AfterEach(func() {
		global.stopRender()
		global = original
	})

	DescribeTable("admits a group's tasks to its permits in the order they were added",
		func(concurrency int) {
			group := StartGroup[int](fmt.Sprintf("fifo-%d", concurrency), WithConcurrency(concurrency))
			running := &runningSet{}
			releases := make([]chan struct{}, members)
			for i := range members {
				releases[i] = make(chan struct{})
				group.Add(fmt.Sprintf("member-%d", i), func(flanksourceContext.Context, *Task) (int, error) {
					running.enter(i)
					defer running.leave(i)
					<-releases[i]
					return i, nil
				})
			}

			// Releasing the oldest running member frees exactly one permit, which
			// must go to the oldest member still waiting — never a later one.
			for oldest := range members {
				expected := make([]int, 0, concurrency)
				for i := oldest; i < min(oldest+concurrency, members); i++ {
					expected = append(expected, i)
				}
				Eventually(running.snapshot).Should(Equal(expected), "after releasing members before %d", oldest)
				close(releases[oldest])
			}
			Eventually(group.Status).Should(Equal(StatusSuccess))
		},
		Entry("one permit runs members in strict insertion order", 1),
		Entry("two permits start members in insertion order", 2),
	)

	It("passes over a member waiting on a later member of the same group instead of deadlocking", func() {
		group := StartGroup[int]("fifo-dependency", WithConcurrency(1))
		var order []string
		var mu sync.Mutex
		record := func(name string) func(flanksourceContext.Context, *Task) (int, error) {
			return func(flanksourceContext.Context, *Task) (int, error) {
				mu.Lock()
				defer mu.Unlock()
				order = append(order, name)
				return 0, nil
			}
		}

		// The dependency is built first so the dependent can name it, but it
		// joins the group's line only after the dependent, behind it.
		later := global.newTask("later", WithGroup(group.Group), WithFunc(func(ctx flanksourceContext.Context, t *Task) error {
			_, err := record("later")(ctx, t)
			return err
		}))
		group.Add("dependent", record("dependent"), WithDependencies(later))
		attachTaskableToGroup(later, later)
		global.enqueueClaimed(later)

		Eventually(group.Status).Should(Equal(StatusSuccess))
		Expect(order).To(Equal([]string{"later", "dependent"}))
	})
})
