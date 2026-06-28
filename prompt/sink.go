package prompt

import (
	"context"
	"sync"
	"time"
)

// Scope tags prompts a producer raises so the UI can filter them to a todo
// (Owner) or a session (Labels["session"]). It travels on the context because the
// generic clicky PromptSelect/PromptText entry points carry no domain identifiers;
// a host (e.g. gavel's auto-commit) wraps its context with WithScope so prompts it
// triggers inherit the right Owner/Labels/Kind.
type Scope struct {
	Owner  string
	Kind   string
	Labels map[string]string
}

type scopeKey struct{}

// WithScope returns a context that tags prompts raised under it with s.
func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}

// ScopeFrom returns the Scope attached to ctx, or the zero Scope.
func ScopeFrom(ctx context.Context) Scope {
	if s, ok := ctx.Value(scopeKey{}).(Scope); ok {
		return s
	}
	return Scope{}
}

// IsZero reports whether s carries no scoping information.
func (s Scope) IsZero() bool {
	return s.Owner == "" && s.Kind == "" && len(s.Labels) == 0
}

// PreferSink reports that a prompt under ctx should be routed to the interactive
// sink in preference to any attached TTY: a manager is installed and ctx carries a
// Scope, meaning the prompt was raised on behalf of a UI-driven operation (e.g. the
// dashboard's auto-commit). Without this, a dashboard server launched from a
// terminal would render its prompts on that terminal instead of the dashboard.
func PreferSink(ctx context.Context) bool {
	return HasInteractiveSink() && !ScopeFrom(ctx).IsZero()
}

var (
	defaultMu      sync.RWMutex
	defaultManager *Manager
)

// SetDefault installs the process-wide Manager that GlobalManager returns and that
// HasInteractiveSink reports on. A host installs one to route TTY-less prompts to
// the dashboard.
func SetDefault(m *Manager) {
	defaultMu.Lock()
	defaultManager = m
	defaultMu.Unlock()
}

// GlobalManager returns the installed process-wide Manager, or nil.
func GlobalManager() *Manager {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultManager
}

// HasInteractiveSink reports whether a Manager is installed — i.e. whether a
// TTY-less prompt has somewhere (the dashboard) to go instead of failing.
func HasInteractiveSink() bool { return GlobalManager() != nil }

func (p Prompt) withScope(s Scope) Prompt {
	if p.Owner == "" {
		p.Owner = s.Owner
	}
	if p.Kind == "" {
		p.Kind = s.Kind
	}
	if len(s.Labels) > 0 {
		merged := make(map[string]string, len(s.Labels)+len(p.Labels))
		for k, v := range s.Labels {
			merged[k] = v
		}
		for k, v := range p.Labels {
			merged[k] = v
		}
		p.Labels = merged
	}
	return p
}

// Select asks the user to choose among labels and returns the chosen indices.
// multi toggles checkbox vs radio. ok is false when the prompt was cancelled. The
// prompt inherits any Scope on ctx. This is the entry point the clicky root routes
// PromptSelect/PromptMultiSelect to when no TTY is attached.
func (m *Manager) Select(ctx context.Context, title string, labels []string, multi bool) ([]int, bool) {
	schema := SelectSchema(title, labels)
	if multi {
		schema = MultiSelectSchema(title, labels)
	}
	p := Prompt{Kind: "select", Title: title, Schema: schema, CreatedAt: time.Now()}.withScope(ScopeFrom(ctx))
	ans, err := m.Ask(ctx, p)
	if err != nil || ans.Cancelled {
		return nil, false
	}
	if multi {
		return SelectedIndexes(ans), true
	}
	if i := SelectedIndex(ans); i >= 0 {
		return []int{i}, true
	}
	return nil, false
}

// Text asks the user for a free-text value. ok is false when cancelled.
func (m *Manager) Text(ctx context.Context, title, def string, secret bool) (string, bool) {
	p := Prompt{Kind: "text", Title: title, Schema: TextSchema(title, secret), CreatedAt: time.Now()}.withScope(ScopeFrom(ctx))
	if def != "" {
		p.Default = map[string]any{textKey: def}
	}
	ans, err := m.Ask(ctx, p)
	if err != nil || ans.Cancelled {
		return "", false
	}
	return TextValue(ans), true
}
