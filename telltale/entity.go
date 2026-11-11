package telltale

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/commons/logger"
	sdk "github.com/flanksource/telltale/sdk"
	"github.com/flanksource/telltale/sdk/protocol"
)

// EntityRecord is one completed clicky entity operation. Parameter values are
// masked, and the operation's result is never captured.
type EntityRecord struct {
	Entity     string            `json:"entity"`
	Verb       string            `json:"verb"`
	Surface    string            `json:"surface,omitempty"`
	TargetID   string            `json:"targetId,omitempty"`
	Admin      bool              `json:"admin,omitempty"`
	ReadOnly   *bool             `json:"readOnly,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Args       []string          `json:"args,omitempty"`
	StartedAt  time.Time         `json:"startedAt"`
	DurationMs float64           `json:"durationMs"`
	Error      string            `json:"error,omitempty"`
}

type entityParams struct {
	Entity     string `json:"entity"`
	Verb       string `json:"verb"`
	ErrorsOnly bool   `json:"errorsOnly"`
}

type entityProbe struct{}

func (entityProbe) Spec() protocol.ProbeSpec {
	return protocol.ProbeSpec{Name: "entity", Title: "Entity operations"}
}

func (entityProbe) Arm(_ context.Context, raw json.RawMessage, sink sdk.Sink) (func() error, error) {
	p, err := parseEntityParams(raw)
	if err != nil {
		return nil, err
	}
	release := entity.RegisterOperationListener(func(ctx context.Context, event entity.OperationEvent) {
		if p.matches(event) {
			sink.TryEmit(makeEntityRecord(ctx, event, time.Now()))
		}
	})
	return func() error { release(); return nil }, nil
}

func parseEntityParams(raw json.RawMessage) (entityParams, error) {
	var p entityParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return p, fmt.Errorf("entity params: %w", err)
		}
	}
	if p.Entity == "" {
		p.Entity = "*"
	}
	if p.Verb == "" {
		p.Verb = "*"
	}
	for _, glob := range []string{p.Entity, p.Verb} {
		if _, err := filepath.Match(glob, ""); err != nil {
			return p, fmt.Errorf("entity glob %q: %w", glob, err)
		}
	}
	return p, nil
}

func (p entityParams) matches(event entity.OperationEvent) bool {
	if p.ErrorsOnly && event.Error == nil {
		return false
	}
	entityMatch, _ := filepath.Match(p.Entity, event.Entity)
	verbMatch, _ := filepath.Match(p.Verb, event.Verb)
	return entityMatch && verbMatch
}

func makeEntityRecord(ctx context.Context, event entity.OperationEvent, finished time.Time) EntityRecord {
	r := EntityRecord{Entity: event.Entity, Verb: event.Verb, Surface: entity.OperationSurfaceFromContext(ctx),
		TargetID: event.TargetID, Admin: event.Admin, ReadOnly: event.ReadOnly,
		StartedAt: finished.Add(-event.Duration), DurationMs: float64(event.Duration) / float64(time.Millisecond)}
	if len(event.Args) > 0 {
		r.Args = sdk.MaskArgs(append([]string{event.Entity}, event.Args...))[1:]
	}
	if len(event.Parameters) > 0 {
		r.Parameters = make(map[string]string, len(event.Parameters))
		for key, value := range event.Parameters {
			if logger.IsSensitiveLogKey(key) {
				value = "****"
			} else {
				value = sdk.Mask(value)
			}
			r.Parameters[key] = value
		}
	}
	if event.Error != nil {
		r.Error = sdk.Mask(event.Error.Error())
	}
	return r
}
