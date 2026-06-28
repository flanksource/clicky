// Package prompt is a generic, reusable question/prompt manager that mirrors the
// task manager: a Go API whose pending-prompt state streams to ready-made UI
// components. Every prompt is described by a JSON Schema (rendered by clicky-ui's
// JsonSchemaForm), state lives behind a swappable cache-backed Store (in-memory by
// default, valkey in the sibling submodule), and the global Manager doubles as the
// "interactive sink" the clicky root routes TTY-less prompts to.
//
// Producers (commit checks, tool-permission approvals, AI elicitation) call
// Manager.Ask and block; consumers (the dashboard via SSE/JSON, or an AI model via
// a validated tool result) resolve the prompt by id.
package prompt

import (
	"encoding/json"
	"time"
)

// State is the lifecycle of a prompt.
type State string

const (
	StatePending   State = "pending"
	StateAnswered  State = "answered"
	StateCancelled State = "cancelled"
	StateExpired   State = "expired"
)

// Prompt is a question awaiting an answer, described by a JSON Schema form.
// Owner and Labels are the scoping keys the UI filters on (e.g. Owner=todoID,
// Labels{"session": id}) — the same role kind/labels/owner play for task RunMeta.
type Prompt struct {
	ID          string
	Kind        string
	Title       string
	Description string
	// Schema is a JSON Schema (object) describing the shape of Answer.Values. It is
	// what clicky-ui renders and what Resolve validates the answer against.
	Schema    json.RawMessage
	Default   map[string]any
	Owner     string
	Labels    map[string]string
	CreatedAt time.Time
}

// Answer is the resolution of a Prompt. Values must satisfy the prompt's Schema
// (validated on Resolve) unless Cancelled is set.
type Answer struct {
	Values    map[string]any
	Cancelled bool
	At        time.Time
}

// PromptSnapshot is the JSON-serializable state of a prompt for UI/SSE consumption.
// It mirrors task.TaskSnapshot: a flat, omitempty-friendly wire shape.
type PromptSnapshot struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind,omitempty"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Schema      json.RawMessage   `json:"schema"`
	State       string            `json:"state"`
	Value       map[string]any    `json:"value,omitempty"`
	Cancelled   bool              `json:"cancelled,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   string            `json:"createdAt,omitempty"`  // RFC3339
	ResolvedAt  string            `json:"resolvedAt,omitempty"` // RFC3339
}

// Filter scopes a snapshot listing. Empty fields match everything; Labels entries
// must all match (AND).
type Filter struct {
	Owner  string
	Kind   string
	State  string
	Labels map[string]string
}

func (f Filter) matches(s PromptSnapshot) bool {
	if f.Owner != "" && s.Owner != f.Owner {
		return false
	}
	if f.Kind != "" && s.Kind != f.Kind {
		return false
	}
	if f.State != "" && s.State != f.State {
		return false
	}
	for k, v := range f.Labels {
		if s.Labels[k] != v {
			return false
		}
	}
	return true
}

func (p Prompt) snapshot() PromptSnapshot {
	created := p.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	return PromptSnapshot{
		ID:          p.ID,
		Kind:        p.Kind,
		Title:       p.Title,
		Description: p.Description,
		Schema:      p.Schema,
		State:       string(StatePending),
		Owner:       p.Owner,
		Labels:      p.Labels,
		CreatedAt:   created.UTC().Format(time.RFC3339),
	}
}
