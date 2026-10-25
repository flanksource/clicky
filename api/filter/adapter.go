package filter

import (
	"errors"
	"fmt"
	"sync"
)

// Adapter is the expression engine behind TableRows, TreeNode and MapRows.
// The core package ships none: applications opt in with celfilter.Use() or
// gomplatefilter.Use() so library consumers don't compile an engine they never call.
type Adapter interface {
	Validate(expression string) error
	// Compile prepares expression for variables shaped like sample. A nil value
	// in sample declares a variable without a fixed type.
	Compile(expression string, sample map[string]any) (Predicate, error)
}

type Predicate interface {
	Match(variables map[string]any) (bool, error)
}

var ErrNoAdapter = errors.New("no filter adapter configured: call celfilter.Use() from github.com/flanksource/clicky/api/filter/celfilter or gomplatefilter.Use() from github.com/flanksource/clicky/api/filter/gomplatefilter")

// ErrMissingVariable is wrapped by adapters when a predicate references a
// variable absent from the evaluated row or node.
var ErrMissingVariable = errors.New("missing variable")

var (
	adapterMu sync.RWMutex
	adapter   Adapter
)

func SetAdapter(a Adapter) {
	adapterMu.Lock()
	defer adapterMu.Unlock()
	adapter = a
}

func currentAdapter(expression string) (Adapter, error) {
	adapterMu.RLock()
	defer adapterMu.RUnlock()
	if adapter == nil {
		return nil, fmt.Errorf("filter %q: %w", expression, ErrNoAdapter)
	}
	return adapter, nil
}

// Validate checks expression syntax with the configured adapter.
func Validate(expression string) error {
	if expression == "" {
		return nil
	}
	a, err := currentAdapter(expression)
	if err != nil {
		return err
	}
	return a.Validate(expression)
}

// Compile prepares expression with the configured adapter; see Adapter.Compile.
func Compile(expression string, sample map[string]any) (Predicate, error) {
	a, err := currentAdapter(expression)
	if err != nil {
		return nil, err
	}
	return a.Compile(expression, sample)
}
