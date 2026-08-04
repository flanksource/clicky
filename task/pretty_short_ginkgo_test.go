package task_test

import (
	"errors"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/task"
	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type shortAndFullResult struct{}

func (shortAndFullResult) Pretty() api.Text          { return api.Text{}.Append("full\nstdout") }
func (shortAndFullResult) PrettyShort() api.Textable { return api.Text{}.Append("short") }

var _ = Describe("Task result rendering", func() {
	BeforeEach(func() {
		task.SetNoRender(true)
	})

	AfterEach(func() {
		task.SetNoRender(false)
	})

	It("prefers PrettyShort over Pretty", func() {
		running := task.StartTask("fixture", func(flanksourceContext.Context, *task.Task) (shortAndFullResult, error) {
			return shortAndFullResult{}, nil
		})
		_, err := running.GetResult()
		Expect(err).NotTo(HaveOccurred())

		Expect(running.Pretty().String()).To(Equal("short"))
	})

	It("still returns task failures", func() {
		running := task.StartTask("fixture", func(flanksourceContext.Context, *task.Task) (shortAndFullResult, error) {
			return shortAndFullResult{}, errors.New("failed")
		})
		_, err := running.GetResult()
		Expect(err).To(MatchError("failed"))
	})
})
