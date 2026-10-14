package exec

import (
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gops "github.com/shirou/gopsutil/v4/process"
)

func skipOnWindows() {
	if runtime.GOOS == "windows" {
		Skip("requires POSIX signals and niceness")
	}
}

func failingRenice() {
	dir := GinkgoT().TempDir()
	Expect(os.WriteFile(filepath.Join(dir, "renice"), []byte("#!/bin/sh\nexit 1\n"), 0o755)).To(Succeed())
	GinkgoT().Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

var _ = Describe("Process runtime limits", func() {
	It("does not execute the target when changing child priority fails", func() {
		skipOnWindows()
		failingRenice()
		result := NewExec("echo", "must not execute").WithNiceness(10).Run().Result()
		Expect(result.Error).To(MatchError(ContainSubstring("launcher failed to set child priority")))
		Expect(result.Stdout).To(BeEmpty())
	})
	It("reports a launcher failure even when nonzero exits are accepted", func() {
		skipOnWindows()
		failingRenice()
		p := NewExec("echo", "must not execute").WithNiceness(10)
		p.SucceedOnNonZero = true
		result := p.Run().Result()
		Expect(result.Error).To(MatchError(ContainSubstring("launcher failed to set child priority")))
		Expect(result.Stdout).To(BeEmpty())
	})
	It("accepts the target's own launcher-like exit status when nonzero exits are accepted", func() {
		skipOnWindows()
		p := NewExec("sh", "-c", "exit 125").WithNiceness(10)
		p.SucceedOnNonZero = true
		Expect(p.Run().Result().Error).NotTo(HaveOccurred())
	})
	It("does not leak the launcher status descriptor into the target", func() {
		skipOnWindows()
		result := NewExec("sh", "-c", "if { true >&3; } 2>/dev/null; then echo open; else echo closed; fi").WithNiceness(10).Run().Result()
		Expect(result.Error).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(result.Stdout)).To(Equal("closed"))
	})
	It("terminates sustained CPU usage with a typed limit error", func() {
		result := NewExec("sh", "-c", "while :\ndo :\ndone").WithProcessGroup().WithTimeout(5 * time.Second).WithResourceLimits(ResourceLimits{MaxCPUPercent: 1, Interval: 50 * time.Millisecond, CPUSampleCount: 2}).Run().Result()
		var limit *ResourceLimitError
		Expect(errors.As(result.Error, &limit)).To(BeTrue(), "%v", result.Error)
		Expect(limit.Reason).To(ContainSubstring("CPU"))
	})
	It("kills descendants left behind when the child exits within the stop grace", func() {
		skipOnWindows()
		pidFile := filepath.Join(GinkgoT().TempDir(), "survivor.pid")
		script := "sleep 30 >/dev/null 2>&1 & echo $! > " + pidFile + "\nwhile :\ndo :\ndone"
		start := time.Now()
		result := NewExec("sh", "-c", script).WithTimeout(20 * time.Second).WithResourceLimits(ResourceLimits{MaxCPUPercent: 1, Interval: 50 * time.Millisecond, CPUSampleCount: 3, StopGrace: 15 * time.Second}).Run().Result()
		var limit *ResourceLimitError
		Expect(errors.As(result.Error, &limit)).To(BeTrue(), "%v", result.Error)
		Expect(time.Since(start)).To(BeNumerically("<", 10*time.Second))
		raw, err := os.ReadFile(pidFile)
		Expect(err).NotTo(HaveOccurred())
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() bool {
			exists, _ := gops.PidExists(int32(pid))
			return exists
		}).WithTimeout(5 * time.Second).Should(BeFalse())
	})
	It("preserves the native exit error for uncapped commands", func() {
		result := NewExec("sh", "-c", "exit 7").Run().Result()
		var exit *osexec.ExitError
		Expect(errors.As(result.Error, &exit)).To(BeTrue())
		Expect(exit.ExitCode()).To(Equal(7))
	})
	It("reports memory termination even when nonzero exits are accepted", func() {
		p := NewExec("sleep", "10").WithProcessGroup().WithResourceLimits(ResourceLimits{MaxRSSBytes: 1, Interval: 20 * time.Millisecond})
		p.SucceedOnNonZero = true
		start := time.Now()
		result := p.Run().Result()
		var limit *ResourceLimitError
		Expect(errors.As(result.Error, &limit)).To(BeTrue(), "%v", result.Error)
		Expect(time.Since(start)).To(BeNumerically("<", 3*time.Second))
	})

	It("applies child niceness without changing the parent", func() {
		skipOnWindows()
		result := NewExec("sh", "-c", "ps -o nice= -p $$").WithNiceness(10).Run().Result()
		Expect(result.Error).NotTo(HaveOccurred())
		nice, err := strconv.Atoi(strings.TrimSpace(result.Stdout))
		Expect(err).NotTo(HaveOccurred())
		Expect(nice).To(BeNumerically(">=", 10))
	})

	It("rejects invalid niceness before launching the command", func() {
		result := NewExec("echo", "should not execute").WithNiceness(20).Run().Result()
		Expect(result.Error).To(MatchError(ContainSubstring("niceness")))
		Expect(result.Stdout).To(BeEmpty())
	})
})
