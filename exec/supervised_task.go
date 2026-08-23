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
	c.supervisor.mu.RUnlock()
	if !latest {
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
		runID := boundTask.ID()
		boundTask.SetBackground(s.opts.Task.Background)
		boundTask.SetController(&supervisedTaskController{supervisor: s, runID: runID})
		boundTask.SetOutputProvider(func() task.OutputSnapshot {
			return task.OutputSnapshot{Stdout: proc.GetStdout(), Stderr: proc.GetStderr()}
		})
		boundTask.SetDetailsProvider(func() any { return s.processDetails(runID, proc) })
		s.resetTaskMetrics()
		return nil
	}

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
	run.SetOutputProvider(func() task.OutputSnapshot {
		return task.OutputSnapshot{Stdout: proc.GetStdout(), Stderr: proc.GetStderr()}
	})
	run.SetDetailsProvider(func() any { return s.processDetails(runID, proc) })

	s.resetTaskMetrics()
	s.mu.Lock()
	s.taskRun = run
	s.mu.Unlock()
	return run
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
	s.mu.RUnlock()
	details.PID = proc.Pid()
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
