//go:build unix

package exec

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/flanksource/clicky/task"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSupervisedProcessFixture(t *testing.T) {
	mode := os.Getenv("CLICKY_CANCEL_FIXTURE")
	if mode == "" {
		return
	}
	signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
	if mode == "parent" {
		child := osexec.Command(os.Args[0], "-test.run=^TestSupervisedProcessFixture$")
		child.Env = append(os.Environ(), "CLICKY_CANCEL_FIXTURE=child")
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		fmt.Println(child.Process.Pid)
	}
	for {
		time.Sleep(time.Minute)
	}
}

var _ = Describe("Supervised cancellation", func() {
	It("force stops descendants in separate process groups and preserves the cancellation cause", func() {
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(context.Canceled)
		group := task.StartGroup[ExecResult]("isolated preview")
		handle := NewExec(os.Args[0], "-test.run=^TestSupervisedProcessFixture$").
			WithoutShell().WithEnv(map[string]string{"CLICKY_CANCEL_FIXTURE": "parent"}).WithProcessGroup().RunSupervisedAsTask(RunSupervisedTaskOptions{
			Name:      "stubborn preview",
			Supervise: SuperviseOptions{ForceStop: true},
			Task: []task.Option{
				task.WithGroup(group.Group),
				task.WithContext(ctx),
				task.WithCancellationDrain(),
			},
		})
		var parent, child int
		Eventually(func() int {
			for _, snap := range task.SnapshotByID(group.ID()) {
				if snap.ID == handle.ID() {
					child, _ = strconv.Atoi(strings.TrimSpace(snap.Stdout))
					if details, ok := snap.Details.(ProcessDetails); ok {
						parent = details.PID
					}
				}
			}
			return child
		}, 5*time.Second).Should(BeNumerically(">", 0))
		DeferCleanup(func() { _ = syscall.Kill(child, syscall.SIGKILL) })
		descendants, err := collectDescendants(int32(parent))
		Expect(err).ToNot(HaveOccurred())
		Expect(descendants).To(ContainElement(treeNode{pid: int32(child), depth: 1}))
		Expect(task.SnapshotByID(group.ID())[1].Controls).To(ContainElements(task.ControlStop, task.ControlRestart))
		started := time.Now()
		cancel(context.DeadlineExceeded)
		result, err := handle.GetResult()
		Expect(err).To(MatchError(context.DeadlineExceeded))
		Expect(result.Status).To(Equal("timeout"))
		Expect(time.Since(started)).To(BeNumerically("<", 3*time.Second))
		Eventually(func() bool { return pidAlive(child) }, 3*time.Second).Should(BeFalse())
		Expect(task.SnapshotByID(group.ID())[1].Controls).To(BeEmpty())
	})
})
