package entity

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"
)

// OperationEvent describes one completed registered operation. Parameters are
// raw transport values, not resolved model identifiers. Result is read-only.
// Duration excludes listeners; Error is only the operation's error.
type OperationEvent struct {
	Entity     string
	Verb       string
	Admin      bool
	TargetID   string
	Parameters map[string]string
	Args       []string
	Result     any
	Error      error
	Duration   time.Duration
}

// OperationListener observes completion synchronously, including failures.
// Filter by entity/verb in the callback. Returning an error does not undo work.
type OperationListener func(context.Context, OperationEvent) error

var operationListeners struct {
	sync.RWMutex
	entries []*OperationListener
}

// RegisterOperationListener adds an independent subscription and returns an
// idempotent unsubscribe function. Duplicates run twice, in registration order.
// Each invocation snapshots subscriptions before running the operation, so init
// registration of entities does not depend on when listeners are installed.
// Unsubscribe does not wait for in-flight snapshots. Nil listeners are invalid.
func RegisterOperationListener(listener OperationListener) func() {
	if listener == nil {
		panic("entity.RegisterOperationListener: nil listener")
	}
	entry := &listener
	operationListeners.Lock()
	operationListeners.entries = append(operationListeners.entries, entry)
	operationListeners.Unlock()
	return func() {
		operationListeners.Lock()
		operationListeners.entries = slices.DeleteFunc(operationListeners.entries, func(v *OperationListener) bool { return v == entry })
		operationListeners.Unlock()
	}
}

// OperationListenerError distinguishes observation failure from business
// failure. Result and OperationError retain the operation outcome even when a
// transport cannot return both data and an error. Never retry a mutation solely
// because this error has a nil OperationError: the operation succeeded.
type OperationListenerError struct {
	Result         any
	OperationError error
	ListenerErrors []error
}

func (e *OperationListenerError) Error() string {
	if e.OperationError != nil {
		return fmt.Sprintf("operation failed: %v; operation listeners failed: %v", e.OperationError, errors.Join(e.ListenerErrors...))
	}
	return fmt.Sprintf("operation succeeded; operation listeners failed (operation was not rolled back): %v", errors.Join(e.ListenerErrors...))
}

// Unwrap preserves errors.Is/As for the operation and every listener failure.
func (e *OperationListenerError) Unwrap() []error {
	if e.OperationError == nil {
		return slices.Clone(e.ListenerErrors)
	}
	return append([]error{e.OperationError}, e.ListenerErrors...)
}

type operationSurfaceKey struct{}

// ContextWithOperationSurface identifies the invocation transport (cli/http/mcp).
func ContextWithOperationSurface(ctx context.Context, surface string) context.Context {
	return context.WithValue(ctx, operationSurfaceKey{}, surface)
}

// OperationSurfaceFromContext returns an empty string for direct invocations.
func OperationSurfaceFromContext(ctx context.Context) string {
	surface, _ := ctx.Value(operationSurfaceKey{}).(string)
	return surface
}

// observeDataFuncs wraps only registration-owned closures, never transport
// dispatch. Both entry points share one observed invocation; legacy handlers
// get a context-aware adapter so their listeners retain the caller's context.
func observeDataFuncs(info EntityInfo, verb string, target bool, data *func(map[string]string, []string) (any, error), contextual *ContextDataFunc) {
	legacy, execute := *data, *contextual
	if execute == nil {
		if legacy == nil {
			return
		}
		execute = func(_ context.Context, flags map[string]string, args []string) (any, error) {
			return legacy(flags, args)
		}
	}
	observed := func(ctx context.Context, flags map[string]string, args []string) (any, error) {
		operationListeners.RLock()
		listeners := slices.Clone(operationListeners.entries)
		operationListeners.RUnlock()
		if len(listeners) == 0 {
			return execute(ctx, flags, args)
		}
		event := OperationEvent{Entity: info.Name, Verb: verb, Admin: info.IsAdmin, Parameters: maps.Clone(flags), Args: slices.Clone(args)}
		if target {
			event.TargetID, _ = entityIDFrom(flags, args)
		}
		start := time.Now()
		result, err := execute(ctx, flags, args)
		event.Result, event.Error, event.Duration = result, err, time.Since(start)
		var failures []error
		for _, listener := range listeners {
			copy := event
			copy.Parameters, copy.Args = maps.Clone(event.Parameters), slices.Clone(event.Args)
			if failure := (*listener)(ctx, copy); failure != nil {
				failures = append(failures, failure)
			}
		}
		if len(failures) > 0 {
			return result, &OperationListenerError{Result: result, OperationError: err, ListenerErrors: failures}
		}
		return result, err
	}
	*contextual = observed
	*data = func(flags map[string]string, args []string) (any, error) {
		return observed(context.Background(), flags, args)
	}
}

func observeEntity(info *EntityInfo) {
	for i := range info.Operations {
		op := &info.Operations[i]
		observeDataFuncs(*info, op.Verb, op.Verb == "get" || op.Verb == "update" || op.Verb == "delete", &op.DataFunc, &op.ContextDataFunc)
	}
	if op := info.PrimaryAction; op != nil {
		observeDataFuncs(*info, op.Name, false, &op.DataFunc, &op.ContextDataFunc)
	}
	for i := range info.Actions {
		op := &info.Actions[i]
		observeDataFuncs(*info, op.Name, true, &op.DataFunc, &op.ContextDataFunc)
	}
	for i := range info.BulkActions {
		op := &info.BulkActions[i]
		observeDataFuncs(*info, op.Name, false, &op.DataFunc, &op.ContextDataFunc)
		observeDataFuncs(*info, op.Name, false, &op.FilterFunc, &op.ContextFilterFunc)
	}
}
