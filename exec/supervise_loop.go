package exec

import (
	"slices"
	"time"

	"github.com/flanksource/clicky/task"
)

// Lifecycle defaults / cadences. Vars (not consts) so tests can shrink the
// port-detection window and cadences to keep them fast.
var (
	defaultStopGrace = 5 * time.Second
	// Port detection runs for the whole life of a run. portPollInterval is the
	// fast cadence used during the startup window (portFastWindow), after which it
	// relaxes to portPollIntervalSlow — so a port that only binds after a cold
	// `go run` compile, well past the window, is still picked up without polling
	// lsof aggressively for the process's whole life. portPromoteGrace is how long
	// a DetectPorts process may sit in "starting" with no port before it is
	// promoted to "running" anyway, so a port-less worker doesn't look stuck while
	// a real server still gets its port annotated whenever it eventually binds.
	portPollInterval     = 300 * time.Millisecond
	portPollIntervalSlow = 3 * time.Second
	portPromoteGrace     = 2 * time.Second
	portFastWindow       = 30 * time.Second
)

// portDetector reports the listening ports of the process tree rooted at pid.
type portDetector func(pid int32) ([]int, error)

// compilationDetector reports whether compiler/linker work is active in the
// process tree rooted at pid.
type compilationDetector func(pid int32) (bool, error)

// detectGroupPorts is the production detector: the listening ports across the
// supervised process's whole group/tree.
func detectGroupPorts(pid int32) ([]int, error) {
	return listeningPortsForPids(collectPids(pid))
}

// Start begins (or resumes) supervising in the background. It is idempotent
// while a supervise loop is active; calling it again after the loop has ended
// (Stop, or a permanent exit) starts a fresh loop.
func (s *SupervisedProcess) Start() {
	s.mu.Lock()
	if s.loopActive {
		s.mu.Unlock()
		return
	}
	s.loopActive = true
	s.desired = true
	s.stopping = false
	s.restarts = 0
	s.exitCode = nil
	s.killed = false
	s.expectsPort = false
	s.status = StatusStarting
	clear(s.handles) // reset CPU-delta cache for the fresh run streak
	s.done = make(chan struct{})
	s.wake = make(chan struct{}, 1)
	done := s.done
	s.mu.Unlock()

	go s.monitorLoop(done)
	go s.runLoop()
}

// Stop gracefully stops the process and prevents further restarts, blocking
// until the supervise loop has ended (or the stop grace elapsed and it was
// force-killed). Safe to call when already stopped.
func (s *SupervisedProcess) Stop() {
	s.mu.Lock()
	if !s.loopActive {
		s.mu.Unlock()
		return
	}
	s.desired = false
	s.stopping = true
	c := s.current
	s.mu.Unlock()

	s.signalWake()
	if c != nil {
		s.killGracefully(c)
	}
	s.Wait()
}

// Restart restarts the process now, resetting the restart counter. If no loop is
// active it simply starts one.
func (s *SupervisedProcess) Restart() {
	s.mu.Lock()
	if !s.loopActive {
		s.mu.Unlock()
		s.Start()
		return
	}
	s.gen++
	s.restarts = 0
	s.status = StatusRestarting
	c := s.current
	s.mu.Unlock()

	s.signalWake()
	if c != nil {
		s.killGracefully(c)
	}
}

// Wait blocks until the current supervise loop has ended.
func (s *SupervisedProcess) Wait() {
	s.mu.RLock()
	d := s.done
	s.mu.RUnlock()
	if d == nil {
		return
	}
	<-d
}

func (s *SupervisedProcess) signalWake() {
	s.mu.RLock()
	w := s.wake
	s.mu.RUnlock()
	if w == nil {
		return
	}
	select {
	case w <- struct{}{}:
	default:
	}
}

func (s *SupervisedProcess) isStopping() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopping
}

func (s *SupervisedProcess) setStatus(status Status) {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
}

// runLoop runs the process, applies the restart policy on exit, and rebuilds it
// until the process is stopped or won't be restarted. Mirrors the per-process
// supervision gavel's Procfile supervisor used to own.
func (s *SupervisedProcess) runLoop() {
	defer func() {
		s.mu.Lock()
		s.current = nil
		s.started = nil
		s.loopActive = false
		d := s.done
		s.mu.Unlock()
		if d != nil {
			close(d)
		}
		if s.opts.OnExit != nil {
			s.opts.OnExit()
		}
	}()

	for {
		s.mu.Lock()
		if !s.desired || s.stopping {
			s.status = StatusStopped
			s.mu.Unlock()
			return
		}
		myGen := s.gen
		s.mu.Unlock()

		proc := s.template.clone()
		taskRun := s.beginTaskGeneration(proc)

		if s.opts.OnStart != nil {
			s.opts.OnStart()
		}

		runDone := make(chan struct{})
		go func() { proc.Run(); close(runDone) }()
		awaitStart(proc, runDone)

		if proc.IsRunning() {
			now := time.Now()
			s.mu.Lock()
			if !s.desired || s.stopping || s.gen != myGen {
				s.mu.Unlock()
				s.killGracefully(proc)
				<-runDone
				if s.isStopping() {
					s.setStatus(StatusStopped)
					return
				}
				continue
			}
			s.current = proc
			s.started = &now
			s.highCPU = 0
			clear(s.handles)
			if s.opts.DetectPorts {
				s.status = StatusStarting
			} else {
				s.status = StatusRunning
			}
			s.mu.Unlock()

			// Measure once as soon as the child is published. monitorLoop only
			// samples on tick boundaries, so a run shorter than the sample
			// interval would otherwise finish with an empty latest/peak.
			s.sample()

			// Notify after the child is current and running so the callback can
			// read proc.Stdin()/StdoutReader(). Must return promptly (see doc).
			if s.opts.OnStarted != nil {
				s.opts.OnStarted(proc)
			}
		}

		if s.opts.DetectPorts && proc.IsRunning() {
			go s.watchPorts(proc, myGen, detectGroupPorts, detectCompilers)
		}

		<-runDone
		res := proc.Result()

		s.mu.Lock()
		result := *res
		s.result = &result
		code := res.ExitCode
		s.exitCode = &code
		genChanged := s.gen != myGen
		desired := s.desired
		restarts := s.restarts
		stopping := s.stopping
		s.mu.Unlock()

		ok := res.IsOk()
		willRestart := shouldRestart(s.opts.RestartPolicy, ok) &&
			(s.opts.MaxRestarts <= 0 || restarts < s.opts.MaxRestarts)
		var taskStatus task.Status
		var lifecycle Status
		switch {
		case stopping || !desired:
			taskStatus = task.StatusCancelled
			lifecycle = StatusStopped
		case genChanged:
			taskStatus = task.StatusCancelled
			lifecycle = StatusRestarting
		case ok:
			taskStatus = task.StatusSuccess
			if willRestart {
				lifecycle = StatusRestarting
			} else {
				lifecycle = StatusExited
			}
		default:
			taskStatus = task.StatusFailed
			if willRestart {
				lifecycle = StatusRestarting
			} else {
				lifecycle = StatusCrashed
			}
		}
		s.setStatus(lifecycle)
		s.finishTaskGeneration(taskRun, taskStatus, res.Error)

		s.mu.Lock()
		s.current = nil
		s.started = nil
		s.ports = nil
		s.mu.Unlock()

		switch {
		case stopping || !desired:
			return
		case genChanged:
			s.mu.Lock()
			s.restarts = 0
			s.mu.Unlock()
			continue
		case !willRestart:
			return
		}

		s.mu.Lock()
		s.restarts++
		s.status = StatusRestarting
		s.mu.Unlock()
		select {
		case <-time.After(backoff(restarts + 1)):
		case <-s.wake:
		}
	}
}

// monitorLoop samples resources of the current run until done is closed.
func (s *SupervisedProcess) monitorLoop(done chan struct{}) {
	ticker := time.NewTicker(s.opts.Limits.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if s.IsRunning() {
				s.sample()
			}
		}
	}
}

// watchPorts polls the current process tree for startup signals and listening
// ports for the lifetime of the run. During startup it reports active compiler
// or linker children as "compiling"; after compilation quiets down it reports
// "starting" while waiting for ports; a detected port or a quiet grace period
// promotes the run to "running". Once running, compiler activity never moves the
// lifecycle backwards. gen guards against a stale watcher from a previous run
// clobbering the current one.
func (s *SupervisedProcess) watchPorts(proc *Process, gen int, detect portDetector, detectCompile compilationDetector) {
	s.mu.RLock()
	done := s.done
	s.mu.RUnlock()
	pid := proc.Pid()
	if pid <= 0 {
		return
	}

	start := time.Now()
	waitingSince := start
	fastUntil := start.Add(portFastWindow)
	for {
		s.mu.RLock()
		current := s.gen == gen && s.current == proc && !s.stopping
		status := s.status
		s.mu.RUnlock()
		if !current || !proc.IsRunning() {
			return
		}

		ports, err := detect(int32(pid))
		if err != nil {
			log.Warnf("detect ports for %s: %v", s.Name(), err)
			s.promoteIfStarting(gen, proc)
			return
		}
		compiling := false
		if status != StatusRunning && len(ports) == 0 && detectCompile != nil {
			if active, err := detectCompile(int32(pid)); err != nil {
				log.Warnf("detect compiler activity for %s: %v", s.Name(), err)
			} else {
				compiling = active
			}
		}

		now := time.Now()
		s.mu.Lock()
		if s.gen != gen || s.current != proc {
			s.mu.Unlock()
			return
		}
		if len(ports) > 0 {
			if !slices.Equal(s.ports, ports) {
				s.ports = ports
			}
			s.expectsPort = true
		}
		if s.status == StatusStarting || s.status == StatusCompiling {
			switch {
			case len(ports) > 0:
				s.status = StatusRunning
			case compiling:
				s.status = StatusCompiling
				waitingSince = time.Time{}
			default:
				if waitingSince.IsZero() {
					waitingSince = now
				}
				s.status = StatusStarting
				if !now.Before(waitingSince.Add(portPromoteGrace)) {
					s.status = StatusRunning
				}
			}
		}
		s.mu.Unlock()

		interval := portPollInterval
		if !time.Now().Before(fastUntil) {
			interval = portPollIntervalSlow
		}
		select {
		case <-time.After(interval):
		case <-done:
			return
		}
	}
}

// promoteIfStarting flips a startup-phase current run to "running" — used when
// port detection can't continue (lsof failed) so a known server isn't left
// wedged in "starting" or "compiling". The gen/proc guard ignores a stale prior
// run.
func (s *SupervisedProcess) promoteIfStarting(gen int, proc *Process) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen == gen && s.current == proc && (s.status == StatusStarting || s.status == StatusCompiling) {
		s.status = StatusRunning
	}
}

// killGracefully sends SIGTERM to the process group, then escalates to SIGKILL
// (KillTree) if it does not exit within the configured stop grace.
func (s *SupervisedProcess) killGracefully(p *Process) {
	_ = terminateTree(p.Pid(), p.newProcessGroup)
	deadline := time.Now().Add(s.opts.StopGrace)
	for time.Now().Before(deadline) {
		if !p.IsRunning() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = p.KillTree()
}

// awaitStart blocks until the process has a pid (it actually launched) or has
// already exited, so a status read after start sees a real pid. Capped so a
// never-starting command can't block the loop forever.
func awaitStart(proc *Process, runDone <-chan struct{}) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if proc.Pid() > 0 {
			return
		}
		select {
		case <-runDone:
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func shouldRestart(policy RestartPolicy, ok bool) bool {
	switch policy {
	case RestartAlways:
		return true
	case RestartOnFailure:
		return !ok
	default:
		return false
	}
}

// backoff returns the delay before the nth restart: 500ms doubling up to 30s.
func backoff(n int) time.Duration {
	d := 500 * time.Millisecond
	for i := 1; i < n; i++ {
		d *= 2
		if d >= 30*time.Second {
			return 30 * time.Second
		}
	}
	return d
}
