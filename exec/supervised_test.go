//go:build unix

package exec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SupervisedProcess", func() {
	fast := ResourceLimits{Interval: 150 * time.Millisecond}

	// busyLoop pegs a CPU core and stays resident, giving the monitor non-zero
	// CPU and RSS to observe. WithProcessGroup makes Stop/KillTree atomic.
	busyLoop := func(opts SuperviseOptions) *SupervisedProcess {
		s := NewExec("while :; do :; done").WithProcessGroup().Supervise(opts)
		s.Start()
		DeferCleanup(s.Stop)
		Eventually(s.IsRunning, 3*time.Second, 20*time.Millisecond).Should(BeTrue())
		return s
	}

	Describe("resource sampling", func() {
		It("reports non-zero CPU, memory and open files while running", func() {
			s := busyLoop(SuperviseOptions{Limits: fast})

			Eventually(func() uint64 { return s.Resources().RSSBytes }, 5*time.Second, 100*time.Millisecond).
				Should(BeNumerically(">", uint64(0)))
			Eventually(func() float64 { return s.Resources().CPUPercent }, 6*time.Second, 100*time.Millisecond).
				Should(BeNumerically(">", 0.0))
			Expect(s.Resources().OpenFiles).ToNot(Equal(0))
			resources := s.Resources()
			peak := s.Peak()
			Expect(peak.RSSBytes).To(BeNumerically(">=", resources.RSSBytes))
		})

		It("exposes a per-process tree covering forked children in the group", func() {
			pidFile := filepath.Join(GinkgoT().TempDir(), "children")
			script := fmt.Sprintf("sleep 300 & echo $! >> %s; sleep 300 & echo $! >> %s; wait",
				shellQuote(pidFile), shellQuote(pidFile))
			s := NewExec(script).WithProcessGroup().Supervise(SuperviseOptions{Limits: fast})
			s.Start()
			DeferCleanup(s.Stop)
			Eventually(s.IsRunning, 3*time.Second, 20*time.Millisecond).Should(BeTrue())
			Eventually(func() int {
				data, err := os.ReadFile(pidFile)
				if err != nil {
					return 0
				}
				return len(strings.Fields(string(data)))
			}, 3*time.Second, 50*time.Millisecond).Should(Equal(2))

			if len(collectPids(int32(s.Pid()))) < 3 {
				Skip("process-group enumeration is unavailable in this environment")
			}
			Eventually(func() int { return len(s.Tree()) }, 5*time.Second, 100*time.Millisecond).
				Should(BeNumerically(">=", 3))
			for _, n := range s.Tree() {
				Expect(n.PID).To(BeNumerically(">", 0))
				Expect(n.RSSBytes).To(BeNumerically(">", uint64(0)))
			}
		})
	})

	Describe("runaway enforcement", func() {
		It("kills a process that stays over the CPU limit", func() {
			s := busyLoop(SuperviseOptions{
				Limits:        ResourceLimits{Interval: 150 * time.Millisecond, MaxCPUPercent: 1, CPUSampleCount: 2},
				RestartPolicy: RestartNo,
			})

			Eventually(s.Killed, 6*time.Second, 100*time.Millisecond).Should(BeTrue())
			Eventually(s.IsRunning, 4*time.Second, 100*time.Millisecond).Should(BeFalse())
		})

		It("leaves a process alone when no limit is configured", func() {
			s := busyLoop(SuperviseOptions{Limits: fast})
			Consistently(s.Killed, 1500*time.Millisecond, 150*time.Millisecond).Should(BeFalse())
			Expect(s.IsRunning()).To(BeTrue())
		})
	})

	Describe("lifecycle", func() {
		It("stops gracefully and reports stopped", func() {
			s := NewExec("sleep 300").WithProcessGroup().Supervise(SuperviseOptions{Limits: fast})
			s.Start()
			DeferCleanup(s.Stop)
			Eventually(s.IsRunning, 3*time.Second, 20*time.Millisecond).Should(BeTrue())

			s.Stop()
			Expect(s.IsRunning()).To(BeFalse())
			Expect(s.Status()).To(Equal(StatusStopped))
		})

		It("restarts on demand with a new pid", func() {
			s := NewExec("sleep 300").WithProcessGroup().Supervise(SuperviseOptions{Limits: fast})
			s.Start()
			DeferCleanup(s.Stop)
			Eventually(s.Pid, 3*time.Second, 20*time.Millisecond).Should(BeNumerically(">", 0))
			first := s.Pid()

			s.Restart()
			Eventually(func() int { return s.Pid() }, 4*time.Second, 50*time.Millisecond).
				Should(And(BeNumerically(">", 0), Not(Equal(first))))
		})

		It("does not restart under the default 'no' policy", func() {
			s := NewExec("exit 1").WithProcessGroup().Supervise(SuperviseOptions{Limits: fast})
			s.Start()
			DeferCleanup(s.Stop)
			Eventually(func() Status { return s.Status() }, 4*time.Second, 50*time.Millisecond).Should(Equal(StatusCrashed))
			Expect(s.Restarts()).To(Equal(0))
		})

		It("restarts a failing process up to MaxRestarts under on-failure", func() {
			s := NewExec("exit 1").WithProcessGroup().Supervise(SuperviseOptions{
				RestartPolicy: RestartOnFailure, MaxRestarts: 2, Limits: fast,
			})
			s.Start()
			DeferCleanup(s.Stop)
			// 500ms + 1s backoff ⇒ ~1.5s for two retries.
			Eventually(func() Status { return s.Status() }, 8*time.Second, 100*time.Millisecond).Should(Equal(StatusCrashed))
			Expect(s.Restarts()).To(Equal(2))
		})

		It("reports exited (not crashed) on a clean zero exit", func() {
			s := NewExec("true").WithProcessGroup().Supervise(SuperviseOptions{Limits: fast})
			s.Start()
			DeferCleanup(s.Stop)
			Eventually(func() Status { return s.Status() }, 4*time.Second, 50*time.Millisecond).Should(Equal(StatusExited))
		})
	})

	Describe("port detection", func() {
		var savedWindow, savedFast, savedSlow, savedGrace time.Duration

		BeforeEach(func() {
			savedWindow, savedFast, savedSlow, savedGrace = portFastWindow, portPollInterval, portPollIntervalSlow, portPromoteGrace
			portFastWindow = 80 * time.Millisecond
			portPollInterval = 10 * time.Millisecond
			portPollIntervalSlow = 20 * time.Millisecond
			portPromoteGrace = 30 * time.Millisecond
		})
		AfterEach(func() {
			portFastWindow, portPollInterval, portPollIntervalSlow, portPromoteGrace = savedWindow, savedFast, savedSlow, savedGrace
		})

		// startWatched supervises a resident process (its own real detector
		// disabled) and parks it in "starting" with the current run's proc/gen, so
		// a test can drive watchPorts with an injected detector.
		startWatched := func() (*SupervisedProcess, *Process, int) {
			s := NewExec("sleep 300").WithProcessGroup().Supervise(SuperviseOptions{Limits: fast})
			s.Start()
			DeferCleanup(s.Stop)
			Eventually(s.IsRunning, 3*time.Second, 20*time.Millisecond).Should(BeTrue())
			s.mu.Lock()
			proc, gen := s.current, s.gen
			s.status = StatusStarting
			s.mu.Unlock()
			return s, proc, gen
		}

		It("records a port that only binds after the startup window", func() {
			s, proc, gen := startWatched()
			start := time.Now()
			// Empty until well past the window, mimicking a slow `go run` compile
			// whose server binds its port only after detection used to give up.
			detect := func(int32) ([]int, error) {
				if time.Since(start) < 4*portFastWindow {
					return nil, nil
				}
				return []int{4321}, nil
			}
			go s.watchPorts(proc, gen, detect, nil)

			Eventually(s.Ports, 2*time.Second, 20*time.Millisecond).Should(Equal([]int{4321}))
			Expect(s.Status()).To(Equal(StatusRunning))
		})

		It("promotes a port-less process to running after the grace, not the full window", func() {
			// A long fast-window proves promotion is driven by the grace, not by the
			// window elapsing: if it waited for the window this would time out.
			portFastWindow = 10 * time.Second
			s, proc, gen := startWatched()
			none := func(int32) ([]int, error) { return nil, nil }
			go s.watchPorts(proc, gen, none, nil)

			Eventually(s.Status, 1*time.Second, 20*time.Millisecond).Should(Equal(StatusRunning))
			Expect(s.Ports()).To(BeEmpty())
		})

		It("reports compiling, then starting, then running during startup", func() {
			s, proc, gen := startWatched()
			var compiling atomic.Bool
			compiling.Store(true)
			none := func(int32) ([]int, error) { return nil, nil }
			detectCompile := func(int32) (bool, error) { return compiling.Load(), nil }
			go s.watchPorts(proc, gen, none, detectCompile)

			Eventually(s.Status, 1*time.Second, 20*time.Millisecond).Should(Equal(StatusCompiling))
			compiling.Store(false)
			Eventually(s.Status, 1*time.Second, 20*time.Millisecond).Should(Equal(StatusStarting))
			Eventually(s.Status, 1*time.Second, 20*time.Millisecond).Should(Equal(StatusRunning))
			Expect(s.Ports()).To(BeEmpty())
		})

		It("keeps reporting compiling past the normal port promotion grace", func() {
			s, proc, gen := startWatched()
			var compiling atomic.Bool
			compiling.Store(true)
			none := func(int32) ([]int, error) { return nil, nil }
			detectCompile := func(int32) (bool, error) { return compiling.Load(), nil }
			go s.watchPorts(proc, gen, none, detectCompile)

			Eventually(s.Status, 1*time.Second, 20*time.Millisecond).Should(Equal(StatusCompiling))
			Consistently(s.Status, portPromoteGrace+40*time.Millisecond, 10*time.Millisecond).Should(Equal(StatusCompiling))
			compiling.Store(false)
			Eventually(s.Status, 1*time.Second, 20*time.Millisecond).Should(Equal(StatusStarting))
			Eventually(s.Status, 1*time.Second, 20*time.Millisecond).Should(Equal(StatusRunning))
		})

		It("promotes to running when a port appears even while compiling", func() {
			s, proc, gen := startWatched()
			detect := func(int32) ([]int, error) { return []int{4321}, nil }
			detectCompile := func(int32) (bool, error) { return true, nil }
			go s.watchPorts(proc, gen, detect, detectCompile)

			Eventually(s.Ports, 1*time.Second, 20*time.Millisecond).Should(Equal([]int{4321}))
			Expect(s.Status()).To(Equal(StatusRunning))
		})
	})
})

var _ = Describe("compiler startup detection", func() {
	It("matches compiler and linker process names", func() {
		Expect(isCompilerExecutable("/tmp/go/pkg/tool/darwin_arm64/compile")).To(BeTrue())
		Expect(isCompilerExecutable("link")).To(BeTrue())
		Expect(isCompilerExecutable("compile.exe")).To(BeTrue())
	})

	It("matches common JavaScript compiler command lines", func() {
		Expect(isCompilerCommandLine([]string{"node", "/workspace/node_modules/esbuild/bin/esbuild"})).To(BeTrue())
		Expect(isCompilerCommandLine([]string{"/usr/local/bin/webpack"})).To(BeTrue())
		Expect(isCompilerCommandLine([]string{"go", "run", "."})).To(BeFalse())
		Expect(isCompilerCommandLine([]string{"npm", "run", "compile"})).To(BeFalse())
	})
})

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
