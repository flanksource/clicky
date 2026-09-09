package exec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/flanksource/clicky/task"
)

// SupervisedTaskOptions customizes the task run created for every supervised
// generation. Labels are merged with the default process label. Href may use
// the literal {id}, which is replaced with the generated run id.
type SupervisedTaskOptions struct {
	Name   string
	Kind   string
	Labels map[string]string
	Owner  string
	Href   string
	// Background marks the process task as long-lived, so global task waits
	// (clicky.WaitForGlobalCompletion, clicky.MustPrint) skip it rather than
	// block until it exits. Set it for a supervised server that must outlive the
	// waits its own client makes — a JSON-RPC agent provider that stays alive
	// across turns is the canonical case. Leave it false when the process IS the
	// work and a wait should drain it. See task.Task.SetBackground.
	Background bool
	// Metadata contributes caller-owned context to every process snapshot — the
	// run a supervised agent is working, the model it uses, the phase it is in.
	// It is evaluated once per snapshot, so the values may change while the
	// process runs; Labels, fixed at run creation, cannot. Any JSON-serializable
	// value is accepted, so structured context need not be flattened to strings.
	// Keep the callback cheap and non-blocking: it runs on the snapshot path.
	Metadata func() any
	// OnFinish runs after a generation freezes its terminal task snapshot.
	OnFinish func(runID string) error
}

// ProcessDetails is the structured supervised-process payload carried by task
// group snapshots.
type ProcessDetails struct {
	PID           int               `json:"pid,omitempty"`
	Command       string            `json:"command"`
	Args          []string          `json:"args,omitempty"`
	Status        Status            `json:"status"`
	Started       *time.Time        `json:"started,omitempty"`
	ExitCode      *int              `json:"exitCode,omitempty"`
	Ports         []int             `json:"ports,omitempty"`
	Restarts      int               `json:"restarts"`
	RestartPolicy RestartPolicy     `json:"restartPolicy"`
	MaxRestarts   int               `json:"maxRestarts,omitempty"`
	Limits        ResourceLimits    `json:"limits"`
	Latest        ResourceSnapshot  `json:"latest"`
	Peak          ResourceSnapshot  `json:"peak"`
	Metrics       map[string]string `json:"metrics"`
	Tree          []ProcessSample   `json:"tree,omitempty"`
	// Metadata is the caller-supplied context for this snapshot, from
	// SupervisedTaskOptions.Metadata — any JSON-serializable value. Nil when the
	// caller supplied no callback, or when the callback itself returned nil.
	Metadata any `json:"metadata,omitempty"`
}

type supervisedTaskController struct {
	supervisor *SupervisedProcess
	runID      string
}

func (c *supervisedTaskController) Actions() []task.ControlAction {
	c.supervisor.mu.RLock()
	latest := c.supervisor.taskRun != nil && c.supervisor.taskRun.ID() == c.runID
	if c.supervisor.boundTask != nil {
		latest = c.supervisor.boundTask.ID() == c.runID
	}
	active := c.supervisor.loopActive
	desired := c.supervisor.desired
	boundTask := c.supervisor.boundTask
	c.supervisor.mu.RUnlock()
	if !latest {
		return nil
	}
	if boundTask != nil && boundTask.Context().Err() != nil {
		return nil
	}
	if active && desired {
		return []task.ControlAction{task.ControlStop, task.ControlRestart}
	}
	return []task.ControlAction{task.ControlStart}
}

func (c *supervisedTaskController) Control(_ context.Context, action task.ControlAction) error {
	if !actionAllowed(c.Actions(), action) {
		return fmt.Errorf("supervised process generation %q does not support %q", c.runID, action)
	}
	switch action {
	case task.ControlStart:
		c.supervisor.Start()
	case task.ControlStop:
		c.supervisor.Stop()
	case task.ControlRestart:
		c.supervisor.Restart()
	}
	return nil
}

func actionAllowed(actions []task.ControlAction, action task.ControlAction) bool {
	for _, candidate := range actions {
		if candidate == action {
			return true
		}
	}
	return false
}

func (s *SupervisedProcess) beginTaskGeneration(proc *Process) *task.ManagedRun {
	s.mu.RLock()
	boundTask := s.boundTask
	s.mu.RUnlock()
	if boundTask != nil {
		// One task spans every generation here, so its output carries across the
		// restart and only gains a marker saying where the boundary was.
		runID := boundTask.ID()
		boundTask.SetBackground(s.opts.Task.Background)
		boundTask.SetController(&supervisedTaskController{supervisor: s, runID: runID})
		s.markGeneration()
		boundTask.SetOutputProvider(s.outputSnapshot)
		boundTask.SetDetailsProvider(func() any { return s.processDetails(runID, proc) })
		s.resetTaskMetrics()
		return nil
	}

	// Every generation gets its own run and its own output tab, so this one
	// starts clean. The run being replaced kept a copy of its own output when
	// finishTaskGeneration froze its providers.
	s.history.Reset()
	runID := uuid.NewString()
	name := s.opts.Task.Name
	if name == "" {
		name = s.Name()
	}
	kind := s.opts.Task.Kind
	if kind == "" {
		kind = "supervised-process"
	}
	labels := map[string]string{"process": s.Name()}
	for key, value := range s.opts.Task.Labels {
		labels[key] = value
	}
	controller := &supervisedTaskController{supervisor: s, runID: runID}
	run := task.StartManagedRun(
		name,
		task.WithGroupID(runID),
		task.WithKind(kind),
		task.WithLabels(labels),
		task.WithOwner(s.opts.Task.Owner),
		task.WithController(controller),
	)
	run.SetBackground(s.opts.Task.Background)
	href := s.opts.Task.Href
	if href == "" {
		href = "/tasks/{id}"
	}
	run.SetHref(strings.ReplaceAll(href, "{id}", runID))
	run.SetOutputProvider(s.outputSnapshot)
	run.SetDetailsProvider(func() any { return s.processDetails(runID, proc) })

	s.resetTaskMetrics()
	s.mu.Lock()
	s.taskRun = run
	s.mu.Unlock()
	return run
}

// outputSnapshot projects the supervisor's retained output onto the task. The
// offsets say how much the ring has already discarded, which is what lets a
// streaming client append rather than redraw the whole pane every poll.
func (s *SupervisedProcess) outputSnapshot() task.OutputSnapshot {
	stdout, stderr := s.history.Tail()
	return task.OutputSnapshot{
		Stdout:       stdout.Data,
		Stderr:       stderr.Data,
		StdoutOffset: stdout.Offset,
		StderrOffset: stderr.Offset,
	}
}

// markGeneration separates one generation's output from the next in a task that
// spans both, so a reader can see where the process was replaced instead of
// finding two runs' output silently concatenated. The marker is written to the
// supervisor's ring only — never to the child's own capture, its ExecResult, or
// a destination the caller tee'd with Stream — and only to a stream that has
// something to separate, so a process that never wrote to stderr does not grow
// a stderr pane containing nothing but markers.
func (s *SupervisedProcess) markGeneration() {
	stdout, stderr := s.history.Tail()
	if stdout.Data == "" && stderr.Data == "" {
		return
	}
	s.mu.RLock()
	exitCode := s.exitCode
	s.mu.RUnlock()
	// A negative code is a signal, not an exit status — the usual case here,
	// since a restart terminates the previous generation. Reporting it as
	// "exit -1" would name something the process never did.
	reason := "restarted"
	if exitCode != nil && *exitCode >= 0 {
		reason = fmt.Sprintf("restarted after exit %d", *exitCode)
	}
	marker := []byte(fmt.Sprintf("\n── %s ──\n", reason))
	if stdout.Data != "" {
		_, _ = s.history.GetStdoutWriter().Write(marker)
	}
	if stderr.Data != "" {
		_, _ = s.history.GetStderrWriter().Write(marker)
	}
}

func (s *SupervisedProcess) resetTaskMetrics() {
	s.mu.Lock()
	s.latest = ResourceSnapshot{}
	s.peak = ResourceSnapshot{}
	s.tree = nil
	s.highCPU = 0
	s.killed = false
	clear(s.handles)
	s.mu.Unlock()
}

func (s *SupervisedProcess) finishTaskGeneration(run *task.ManagedRun, status task.Status, err error) {
	if run != nil {
		run.Finish(status, err)
		if archive := s.opts.Task.OnFinish; archive != nil {
			if archiveErr := archive(run.ID()); archiveErr != nil {
				log.Errorf("archive supervised task generation %s: %v", run.ID(), archiveErr)
			}
		}
	}
}

func (s *SupervisedProcess) processDetails(runID string, proc *Process) ProcessDetails {
	s.mu.RLock()
	details := ProcessDetails{
		Command:       proc.commandLabel(),
		Args:          append([]string(nil), proc.Args...),
		Status:        s.status,
		Started:       ptrCopy(s.started),
		ExitCode:      ptrCopy(s.exitCode),
		Ports:         append([]int(nil), s.ports...),
		Restarts:      s.restarts,
		RestartPolicy: s.opts.RestartPolicy,
		MaxRestarts:   s.opts.MaxRestarts,
		Limits:        s.opts.Limits,
		Latest:        s.latest,
		Peak:          s.peak,
		Tree:          append([]ProcessSample(nil), s.tree...),
		Metrics: map[string]string{
			"cpu":       task.MetricID(runID, "cpu"),
			"rss":       task.MetricID(runID, "rss"),
			"vms":       task.MetricID(runID, "vms"),
			"openFiles": task.MetricID(runID, "open-files"),
		},
	}
	metadata := s.opts.Task.Metadata
	s.mu.RUnlock()
	details.PID = proc.Pid()
	// Evaluated outside the lock: the callback is caller code and must not be
	// able to deadlock the supervisor by reaching back into it. Whatever it
	// returns is carried as-is — an empty-but-present value stays present, so a
	// viewer sees the block go quiet rather than disappear between snapshots.
	if metadata != nil {
		details.Metadata = metadata()
	}
	return details
}

func (s *SupervisedProcess) recordTaskMetrics(snapshot ResourceSnapshot) {
	s.mu.RLock()
	run := s.taskRun
	boundTask := s.boundTask
	s.mu.RUnlock()
	runID := ""
	if boundTask != nil {
		runID = boundTask.ID()
	} else if run != nil {
		runID = run.ID()
	}
	if runID == "" {
		return
	}
	for name, value := range map[string]float64{
		"cpu": float64(snapshot.CPUPercent),
		"rss": float64(snapshot.RSSBytes),
		"vms": float64(snapshot.VMSBytes),
	} {
		if err := task.RecordMetric(runID, name, value, snapshot.SampledAt); err != nil {
			log.Debugf("record %s metric for %s: %v", name, s.Name(), err)
		}
	}
	if snapshot.OpenFiles >= 0 {
		if err := task.RecordMetric(runID, "open-files", float64(snapshot.OpenFiles), snapshot.SampledAt); err != nil {
			log.Debugf("record open-files metric for %s: %v", s.Name(), err)
		}
	}
}
