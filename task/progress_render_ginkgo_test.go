package task_test

import (
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task progress rendering", func() {
	BeforeEach(func() {
		task.SetNoRender(true)
	})

	AfterEach(func() {
		task.SetNoRender(false)
	})

	renderWithProgress := func(value, maximum int) string {
		running := task.StartTask("fixture", func(_ flanksourceContext.Context, t *task.Task) (int, error) {
			t.SetProgress(value, maximum)
			return value, nil
		})
		_, err := running.GetResult()
		Expect(err).NotTo(HaveOccurred())
		return running.Pretty().String()
	}

	It("renders the value/max count and a bar for bounded progress", func() {
		line := renderWithProgress(3, 7)
		Expect(line).To(ContainSubstring("3/7"))
		Expect(line).To(ContainSubstring("▰▰▰▰▱▱▱▱▱▱"))
	})

	It("renders no count or bar without bounded progress", func() {
		line := renderWithProgress(3, 0)
		Expect(line).NotTo(ContainSubstring("3/"))
		Expect(line).NotTo(ContainSubstring("▱"))
	})

	It("renders no count or bar when the task never reports progress", func() {
		running := task.StartTask("fixture", func(_ flanksourceContext.Context, _ *task.Task) (int, error) {
			return 0, nil
		})
		_, err := running.GetResult()
		Expect(err).NotTo(HaveOccurred())
		line := running.Pretty().String()
		Expect(line).NotTo(ContainSubstring("0/100"))
		Expect(line).NotTo(ContainSubstring("▱"))
	})
})
