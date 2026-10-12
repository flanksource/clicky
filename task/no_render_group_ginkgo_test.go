package task

import (
	"context"
	"time"

	"github.com/charmbracelet/lipgloss"
	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WithNoRender group", func() {
	const hiddenName, visibleName = "hidden chat turn", "visible build step"

	newHiddenGroup := func(tm *Manager) *Group {
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		group := &Group{name: "background chat", manager: tm, ctx: ctx, cancel: cancel}
		WithNoRender()(group)
		return group
	}

	finishedTask := func(tm *Manager, name string, opts ...Option) *Task {
		t := tm.newTask(name, opts...)
		t.SetStatus(StatusSuccess)
		t.completed.Store(true)
		t.dirty.Store(true)
		return t
	}

	renderStateOf := func(tm *Manager) renderLifecycleState {
		tm.mu.RLock()
		defer tm.mu.RUnlock()
		return tm.renderState
	}

	It("leaves its tasks out of the live tree, plain deltas and final tree while other tasks still render", func() {
		tm := newTestManager(1)
		DeferCleanup(func() { close(tm.shutdown) })
		output := &syncBuffer{}
		tm.renderer = lipgloss.NewRenderer(output)
		tm.tasks = append(tm.tasks,
			finishedTask(tm, hiddenName, WithGroup(newHiddenGroup(tm))),
			finishedTask(tm, visibleName))

		live := tm.Pretty().String()
		tm.PlainRender()
		tm.renderFinal(false)

		Expect(live).To(ContainSubstring(visibleName))
		Expect(live).NotTo(ContainSubstring(hiddenName))
		Expect(output.String()).To(ContainSubstring(visibleName))
		Expect(output.String()).NotTo(ContainSubstring(hiddenName))
	})

	It("prints no final summary when every task belongs to a hidden group", func() {
		tm := newTestManager(1)
		DeferCleanup(func() { close(tm.shutdown) })
		output := &syncBuffer{}
		tm.renderer = lipgloss.NewRenderer(output)
		tm.tasks = append(tm.tasks, finishedTask(tm, hiddenName, WithGroup(newHiddenGroup(tm))))

		tm.renderFinal(true)

		Expect(output.String()).To(BeEmpty())
	})

	It("does not start the render loop for its tasks, while a visible task does", func() {
		tm := newTestManager(1)
		tm.noProgress.Store(false)
		tm.renderer = lipgloss.NewRenderer(&syncBuffer{})
		DeferCleanup(func() {
			tm.stopRender()
			close(tm.shutdown)
		})
		run := func(t *Task) *Task {
			t.runFunc = func(flanksourceContext.Context, *Task) error { return nil }
			return t
		}

		hidden := run(tm.newTask(hiddenName, WithGroup(newHiddenGroup(tm))))
		tm.enqueueClaimed(hidden)
		Eventually(hidden.doneChan).WithTimeout(2 * time.Second).Should(BeClosed())
		Expect(renderStateOf(tm)).To(Equal(renderIdle))
		Expect(hidden.Status()).To(Equal(StatusSuccess))

		tm.enqueueClaimed(run(tm.newTask(visibleName)))
		Expect(renderStateOf(tm)).To(Equal(renderRunning))
	})

	It("still runs, returns results and lists its tasks in run snapshots", func() {
		original := global
		global = newTestManager(2)
		DeferCleanup(func() {
			global.stopRender()
			global = original
		})

		group := StartGroup[string]("background chat", WithGroupID("hidden-run"), WithNoRender())
		result, err := group.Add(hiddenName, func(flanksourceContext.Context, *Task) (string, error) {
			return "answered", nil
		}).GetResult()

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("answered"))
		Expect(group.Status()).To(Equal(StatusSuccess))
		var names []string
		for _, snapshot := range SnapshotByID("hidden-run") {
			names = append(names, snapshot.Name)
		}
		Expect(names).To(ContainElements("background chat", hiddenName))
	})
})
