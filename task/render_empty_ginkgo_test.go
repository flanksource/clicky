package task

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/flanksource/clicky/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type emptyPrettyShortResult struct{}

func (emptyPrettyShortResult) PrettyShort() api.Textable {
	return api.Text{}
}

var _ = Describe("Empty task rendering", func() {
	newEmptyResultTask := func(tm *Manager, name string) *Task {
		task := tm.newTask(name)
		task.result = emptyPrettyShortResult{}
		task.SetStatus(StatusSuccess)
		task.completed.Store(true)
		task.dirty.Store(true)
		return task
	}

	It("does not allocate rows to hidden task results", func() {
		tm := newTestManager(1)
		DeferCleanup(func() { close(tm.shutdown) })

		hiddenBefore := newEmptyResultTask(tm, "hidden-before")
		visible := tm.newTask("visible-failure")
		visible.SetStatus(StatusFailed)
		visible.completed.Store(true)
		hiddenAfter := newEmptyResultTask(tm, "hidden-after")

		rendered := tm.prettyFromTasks([]*Task{hiddenBefore, visible, hiddenAfter}).String()
		lines := strings.Split(strings.TrimRight(rendered, " \t\r\n"), "\n")

		Expect(lines).To(HaveLen(1))
		Expect(lines[0]).To(ContainSubstring("visible-failure"))
		Expect(rendered).NotTo(ContainSubstring("hidden-before"))
		Expect(rendered).NotTo(ContainSubstring("hidden-after"))
	})

	It("returns an empty task tree when every task result is hidden", func() {
		tm := newTestManager(1)
		DeferCleanup(func() { close(tm.shutdown) })

		rendered := tm.prettyFromTasks([]*Task{
			newEmptyResultTask(tm, "hidden-one"),
			newEmptyResultTask(tm, "hidden-two"),
		})

		Expect(rendered.IsEmpty()).To(BeTrue())
		Expect(rendered.String()).To(BeEmpty())
	})

	It("does not print an empty plain task delta", func() {
		tm := newTestManager(1)
		DeferCleanup(func() { close(tm.shutdown) })
		output := &syncBuffer{}
		tm.renderer = lipgloss.NewRenderer(output)
		tm.tasks = append(tm.tasks, newEmptyResultTask(tm, "hidden"))

		tm.PlainRender()

		Expect(output.String()).To(BeEmpty())
	})

	It("does not print empty plain live or final blocks", func() {
		tm := newTestManager(1)
		DeferCleanup(func() { close(tm.shutdown) })
		output := &syncBuffer{}
		tm.renderer = lipgloss.NewRenderer(output)
		tm.setLiveRenderer(&fakeLiveRenderer{})
		tm.tasks = append(tm.tasks, newEmptyResultTask(tm, "hidden"))

		tm.PlainRender()
		tm.renderFinal(false)

		Expect(output.String()).To(BeEmpty())
	})

	It("emits no terminal bytes for an empty frame without a previous frame", func() {
		tm := newTestManager(1)
		DeferCleanup(func() { close(tm.shutdown) })
		tm.isInteractive.Store(true)
		output := &syncBuffer{}
		tm.renderer = lipgloss.NewRenderer(output)
		tm.setLiveRenderer(&fakeLiveRenderer{})

		lastLines := tm.interactiveRender(-1, false)

		Expect(lastLines).To(Equal(-1))
		Expect(output.String()).To(BeEmpty())
	})

	It("clears a populated frame once when the next frame is empty", func() {
		tm := newTestManager(1)
		DeferCleanup(func() { close(tm.shutdown) })
		tm.isInteractive.Store(true)
		output := &syncBuffer{}
		tm.renderer = lipgloss.NewRenderer(output)
		renderer := &fakeLiveRenderer{live: "visible"}
		tm.setLiveRenderer(renderer)

		lastLines := tm.interactiveRender(-1, false)
		firstTickBytes := len(output.String())
		renderer.live = ""
		lastLines = tm.interactiveRender(lastLines, false)
		clearingTick := output.String()[firstTickBytes:]

		Expect(lastLines).To(Equal(-1))
		Expect(clearingTick).To(Equal("\x1b[2K\r"))

		bytesAfterClear := len(output.String())
		lastLines = tm.interactiveRender(lastLines, false)
		Expect(lastLines).To(Equal(-1))
		Expect(output.String()).To(HaveLen(bytesAfterClear))
	})
})
