// Package gomplatefilter adds the flanksource/gomplate CEL function set to
// filter expressions. It compiles gomplate's Kubernetes dependency tree, so
// only binaries that need those functions should import it.
package gomplatefilter

import (
	"cel.dev/cel-go/cel"
	"github.com/flanksource/clicky/api/filter"
	"github.com/flanksource/clicky/api/filter/celfilter"
	"github.com/flanksource/gomplate/v4"
)

// EnvOptions returns the gomplate CEL functions for other CEL environments.
func EnvOptions() []cel.EnvOption {
	return gomplate.GetCelEnv(map[string]any{})
}

// Use makes CEL with gomplate functions the filter engine.
func Use() {
	filter.SetAdapter(celfilter.New(EnvOptions()...))
}
