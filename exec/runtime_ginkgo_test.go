package exec

import (
	"errors"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Process runtime limits", func() {
	It("does not execute the target when changing child priority fails", func() {
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "renice"), []byte("#!/bin/sh\nexit 1\n"), 0o755)).To(Succeed())
		GinkgoT().Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		result := NewExec("echo", "must not execute").WithNiceness(10).Run().Result()
		Expect(result.Error).To(HaveOccurred())
		Expect(result.Stdout).To(BeEmpty())
	})
	It("terminates sustained CPU usage with a typed limit error", func() {
		result := NewExec("sh", "-c", "while :\ndo :\ndone").WithProcessGroup().WithTimeout(5 * time.Second).WithResourceLimits(ResourceLimits{MaxCPUPercent: 1, Interval: 50 * time.Millisecond, CPUSampleCount: 2}).Run().Result()
		var limit *ResourceLimitError
		Expect(errors.As(result.Error, &limit)).To(BeTrue(), "%v", result.Error)
		Expect(limit.Reason).To(ContainSubstring("CPU"))
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
