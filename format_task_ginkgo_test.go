package clicky

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = ginkgo.Describe("task-aware command rendering", func() {
	ginkgo.DescribeTable("waits for outstanding tasks outside a task context", func(withContext bool) {
		var ctx context.Context
		if withContext {
			ctx = context.Background()
		}
		release := make(chan struct{})
		started := make(chan struct{})
		worker := task.StartTask("outstanding work", func(commonscontext.Context, *task.Task) (int, error) {
			close(started)
			<-release
			return 1, nil
		})
		ginkgo.DeferCleanup(func() { worker.Cancel() })
		Eventually(started).Should(BeClosed())
		path := filepath.Join(ginkgo.GinkgoT().TempDir(), "result.json")
		finished := make(chan error, 1)
		go func() {
			finished <- PrintAndWriteSinks("complete", FormatOptions{Context: ctx, Sinks: []formatters.FormatSink{{Format: "json", File: path}}})
		}()
		Consistently(finished, 100*time.Millisecond).ShouldNot(Receive())
		close(release)
		Eventually(finished).Should(Receive(BeNil()))
		contents, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(ContainSubstring("complete"))
	}, ginkgo.Entry("without a context", false), ginkgo.Entry("with an ordinary context", true))

	ginkgo.DescribeTable("finishes a generated command and starts its queued successor", func(result any) {
		previous := Flags.FormatOptions
		ginkgo.DeferCleanup(func() { Flags.FormatOptions = previous })
		path := filepath.Join(ginkgo.GinkgoT().TempDir(), "result.json")
		Flags.FormatOptions = FormatOptions{Sinks: []formatters.FormatSink{{Format: "json", File: path}}}
		group := task.StartGroup[int]("nested rendering", task.WithConcurrency(1))
		first := group.Add("render command", func(ctx commonscontext.Context, _ *task.Task) (int, error) {
			parent := &cobra.Command{Use: "example"}
			AddNamedCommandWithContext("report", parent, struct{}{}, func(context.Context, struct{}) (any, error) {
				return result, nil
			})
			parent.SetArgs([]string{"report"})
			return 1, parent.ExecuteContext(ctx)
		})
		second := group.Add("queued successor", func(commonscontext.Context, *task.Task) (int, error) {
			return 2, nil
		}, task.WithDependencies(first.Task))
		ginkgo.DeferCleanup(func() { first.Cancel(); second.Cancel() })
		Eventually(second.Status, 3*time.Second).Should(Equal(task.StatusSuccess))
		Expect(first.Status()).To(Equal(task.StatusSuccess))
		if result != nil {
			contents, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("report result"))
		}
	}, ginkgo.Entry("with no result", nil), ginkgo.Entry("with a result", "report result"))
})
