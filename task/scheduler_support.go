package task

import (
	"errors"
	"fmt"
	"sync"

	flanksourceContext "github.com/flanksource/commons/context"
)

// Runner is the work behind a schedule. The group is already registered and
// stamped with the schedule's kind and labels; the runner adds tasks to it.
type Runner func(ctx flanksourceContext.Context, schedule Schedule, group *Group) error

var (
	runnersMu sync.RWMutex
	runners   = map[string]Runner{}
)

// RegisterRunner registers the work for one schedule kind. Registering a kind
// twice is a programming error and panics, matching the entity and provider
// registries elsewhere: two runners for one kind means one of them never runs.
func RegisterRunner(kind string, runner Runner) {
	if kind == "" || runner == nil {
		panic("task: RegisterRunner requires a kind and a runner")
	}
	runnersMu.Lock()
	defer runnersMu.Unlock()
	if _, exists := runners[kind]; exists {
		panic(fmt.Sprintf("task: runner for kind %q already registered", kind))
	}
	runners[kind] = runner
}

func runnerFor(kind string) (Runner, bool) {
	runnersMu.RLock()
	defer runnersMu.RUnlock()
	runner, ok := runners[kind]
	return runner, ok
}

func errsJoin(errs []error) error {
	return errors.Join(errs...)
}
