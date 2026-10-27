package exec

import (
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/commons/logger"
)

var _ = Describe("Process control", func() {
	// stopAndAwaitExit reaps a started process and waits for its terminal
	// state so no child outlives the spec. KillTree runs first because a
	// shell-wrapped command does not forward interrupts to its children.
	stopAndAwaitExit := func(process *Process) {
		_ = process.KillTree()
		_ = process.Stop()
		Eventually(process.IsRunning, 5*time.Second, 10*time.Millisecond).Should(BeFalse())
	}

	It("starts a process in the background", func() {
		process := NewExec("sleep 60; echo done").WithProcessGroup()
		Expect(process.Start()).To(Succeed())
		DeferCleanup(func() { stopAndAwaitExit(process) })
		Eventually(process.IsRunning, 5*time.Second, 10*time.Millisecond).Should(BeTrue())
	})

	It("terminates a long-running process", func() {
		process := NewExec("sleep", "60").WithoutShell().Debug()
		go process.Run()
		DeferCleanup(func() { stopAndAwaitExit(process) })
		Eventually(process.IsRunning, 5*time.Second, 10*time.Millisecond).Should(BeTrue())
		logger.Infof("%s", process.Pretty().ANSI())

		Expect(process.Stop()).To(Succeed())
		logger.Infof("after: %s", process.Pretty().ANSI())
		Expect(process.IsOK()).To(BeFalse())
		Expect(process.IsRunning()).To(BeFalse())
		Expect(process.Result().Status).To(Equal("failed"))
	})

	It("stops a completed process without error", func() {
		process := NewExec("sleep", "0.1").WithoutShell().Debug()
		go process.Run()
		Eventually(func() string { return process.Result().Status }, 5*time.Second, 10*time.Millisecond).Should(Equal("success"))

		Expect(process.MustStop(5 * time.Second)).To(Succeed())
		Expect(process.IsOK()).To(BeTrue())
	})

	It("returns from a timed run when a backgrounded descendant keeps stdout open", func() {
		// The direct child exits at once but leaves a grandchild holding the
		// stdout pipe, the same shape as `pg_ctl start` daemonising postgres.
		process := NewExec("sh", "-c", "sleep 60 & echo $!").WithTimeout(5 * time.Second)
		DeferCleanup(func() {
			if pid, err := strconv.Atoi(strings.TrimSpace(process.GetStdout())); err == nil {
				if orphan, err := os.FindProcess(pid); err == nil {
					_ = orphan.Kill()
				}
			}
		})

		done := make(chan struct{})
		go func() {
			process.Run()
			close(done)
		}()
		Eventually(done, 5*time.Second).Should(BeClosed())
	})

	Describe("Start failures", func() {
		It("reports a terminal failed status when the working directory is invalid", func() {
			result := NewExec("echo", "started").WithoutShell().WithCwd("/nonexistent-dir-for-exec-tests").Run().Result()
			Expect(result.Error).To(HaveOccurred())
			Expect(result.IsPending()).To(BeFalse())
			Expect(result.Status).To(Equal("failed"))
		})

		It("reports a terminal failed status when the shell cannot be resolved", func() {
			result := NewExec("echo started").WithShell("/nonexistent-shell-for-exec-tests").Run().Result()
			Expect(result.Error).To(MatchError(ContainSubstring("shell not found")))
			Expect(result.IsPending()).To(BeFalse())
			Expect(result.Status).To(Equal("failed"))
		})
	})
})
