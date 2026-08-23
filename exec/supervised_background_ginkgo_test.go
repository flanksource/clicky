//go:build unix

package exec

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/task"
)

// backgroundFlag reads the process task's background flag once the supervisor
// has begun a generation.
func backgroundFlag(s *SupervisedProcess) func() bool {
	return func() bool {
		s.mu.RLock()
		run := s.taskRun
		s.mu.RUnlock()
		if run == nil {
			return false
		}
		return run.Task().IsBackground()
	}
}

var _ = Describe("Supervised background processes", func() {
	// A supervised JSON-RPC server (captain's claude-agent / codex-appserver
	// providers) must outlive the waits its own client makes: the client commits
	// mid-session and the commit pipeline drains global tasks. Without Background
	// the process task stays Running for the whole session and that drain
	// deadlocks — the reported `gavel pr status --ai-fix` hang.
	It("marks the process task so global waits skip it", func() {
		supervisor := NewExec("while :; do sleep 1; done").WithProcessGroup().Supervise(SuperviseOptions{
			Task: SupervisedTaskOptions{Background: true},
		})
		supervisor.Start()
		DeferCleanup(supervisor.Stop)

		Eventually(backgroundFlag(supervisor), 5*time.Second).Should(BeTrue())

		s := supervisor
		s.mu.RLock()
		run := s.taskRun
		s.mu.RUnlock()
		Expect(run.Task().Status()).To(Equal(task.StatusRunning))
	})

	// Background is opt-in: a supervised process that IS the work (`gavel proc
	// run`) must keep blocking the wait.
	It("leaves an ordinary supervised process blocking the wait", func() {
		supervisor := NewExec("while :; do sleep 1; done").WithProcessGroup().Supervise(SuperviseOptions{})
		supervisor.Start()
		DeferCleanup(supervisor.Stop)

		Eventually(func() bool {
			supervisor.mu.RLock()
			defer supervisor.mu.RUnlock()
			return supervisor.taskRun != nil
		}, 5*time.Second).Should(BeTrue())

		Expect(backgroundFlag(supervisor)()).To(BeFalse())
	})
})
