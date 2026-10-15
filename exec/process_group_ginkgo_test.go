//go:build unix

package exec

import (
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
)

var _ = Describe("WithProcessGroup", func() {
	start := func(process *Process) int {
		Expect(process.Start()).To(Succeed())
		DeferCleanup(func() {
			Expect(process.KillTree()).To(Succeed())
			Eventually(process.IsRunning, 5*time.Second, 10*time.Millisecond).Should(BeFalse())
		})
		Eventually(process.IsRunning, 5*time.Second, 10*time.Millisecond).Should(BeTrue())
		return process.Pid()
	}

	It("starts the child as leader of its own session, detached from the caller's terminal", func() {
		pid := start(NewExec("sleep", "30").WithoutShell().WithProcessGroup())

		Expect(unix.Getsid(pid)).To(Equal(pid))
		Expect(syscall.Getpgid(pid)).To(Equal(pid))
	})

	It("keeps the caller's session when the child opts into the controlling terminal", func() {
		pid := start(NewExec("sleep", "30").WithoutShell().WithProcessGroup().WithControllingTerminal())

		ownSession, err := unix.Getsid(0)
		Expect(err).NotTo(HaveOccurred())
		Expect(unix.Getsid(pid)).To(Equal(ownSession))
		Expect(syscall.Getpgid(pid)).To(Equal(pid))
	})
})
