package telltale

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/flanksource/clicky/exec"
	sdk "github.com/flanksource/telltale/sdk"
	"github.com/flanksource/telltale/sdk/protocol"
)

// ExecRecord is one clicky exec start or exit. ProcessKey is <pid>@<create
// unix second>, the key telltale's process sampler gives the same child, so a
// start record hands the child off to the process kind; it is the bare pid
// when the child exited before its create time could be read.
type ExecRecord struct {
	Phase      exec.ExecPhase `json:"phase"`
	Name       string         `json:"name"`
	Args       []string       `json:"args,omitempty"`
	Dir        string         `json:"dir,omitempty"`
	EnvKeys    []string       `json:"envKeys,omitempty"`
	PID        int            `json:"pid"`
	PPID       int            `json:"ppid"`
	PGID       int            `json:"pgid,omitempty"`
	ProcessKey string         `json:"processKey,omitempty"`
	CreatedAt  *time.Time     `json:"createdAt,omitempty"`
	StartedAt  time.Time      `json:"startedAt"`
	DurationMs float64        `json:"durationMs,omitempty"`
	ExitCode   *int           `json:"exitCode,omitempty"`
	Error      string         `json:"error,omitempty"`
	StdoutTail string         `json:"stdoutTail,omitempty"`
	StderrTail string         `json:"stderrTail,omitempty"`
}

type execParams struct {
	Name      string           `json:"name"`
	TailBytes int              `json:"tailBytes"`
	Phases    []exec.ExecPhase `json:"phases"`
}

type execProbe struct{}

func (execProbe) Spec() protocol.ProbeSpec { return protocol.ProbeSpec{Name: "exec", Title: "Exec"} }

func (execProbe) Arm(_ context.Context, raw json.RawMessage, sink sdk.Sink) (func() error, error) {
	p, err := parseExecParams(raw)
	if err != nil {
		return nil, err
	}
	release := exec.ObserveExec(func(event exec.ExecEvent) {
		match, _ := filepath.Match(p.Name, filepath.Base(event.Name))
		if match && p.wants(event.Phase) {
			sink.TryEmit(makeExecRecord(event, p.TailBytes))
		}
	})
	return func() error { release(); return nil }, nil
}

func parseExecParams(raw json.RawMessage) (execParams, error) {
	var p execParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return p, fmt.Errorf("exec params: %w", err)
		}
	}
	if p.Name == "" {
		p.Name = "*"
	}
	if _, err := filepath.Match(p.Name, ""); err != nil {
		return p, fmt.Errorf("exec name glob: %w", err)
	}
	if p.TailBytes < 0 || p.TailBytes > exec.TailBytes {
		return p, fmt.Errorf("exec tailBytes must be between 0 and %d", exec.TailBytes)
	}
	if len(p.Phases) == 0 {
		p.Phases = []exec.ExecPhase{exec.ExecStart, exec.ExecExit}
	}
	for _, phase := range p.Phases {
		if phase != exec.ExecStart && phase != exec.ExecExit {
			return p, fmt.Errorf("exec phase %q: expected %q or %q", phase, exec.ExecStart, exec.ExecExit)
		}
	}
	return p, nil
}

func (p execParams) wants(phase exec.ExecPhase) bool {
	for _, wanted := range p.Phases {
		if wanted == phase {
			return true
		}
	}
	return false
}

func makeExecRecord(event exec.ExecEvent, limit int) ExecRecord {
	args := sdk.MaskArgs(append([]string{event.Name}, event.Args...))
	r := ExecRecord{Phase: event.Phase, Name: event.Name, Args: args[1:], Dir: event.Dir, EnvKeys: event.EnvKeys,
		PID: event.PID, PPID: event.PPID, PGID: event.PGID, StartedAt: event.StartedAt}
	if event.PID > 0 {
		r.ProcessKey = strconv.Itoa(event.PID)
	}
	if !event.CreatedAt.IsZero() {
		created := event.CreatedAt
		r.CreatedAt = &created
		r.ProcessKey += "@" + strconv.FormatInt(created.Unix(), 10)
	}
	if event.Phase != exec.ExecExit {
		return r
	}
	exitCode := event.ExitCode
	r.ExitCode = &exitCode
	r.DurationMs = float64(event.Duration) / float64(time.Millisecond)
	r.StdoutTail = tail(sdk.Mask(event.Stdout), limit)
	r.StderrTail = tail(sdk.Mask(event.Stderr), limit)
	if event.Err != nil {
		r.Error = sdk.Mask(event.Err.Error())
	}
	return r
}

func tail(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	start := len(value) - limit
	for start < len(value) && !utf8.RuneStart(value[start]) {
		start++
	}
	return value[start:]
}
