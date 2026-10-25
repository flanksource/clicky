package schemaform

import (
	"sync"

	"github.com/flanksource/clicky/api/filter"
)

// exprSample declares the `when.expr` variables: key is a string, value is
// untyped, self and root are objects (see ExprInput).
var exprSample = map[string]any{"key": "", "value": nil, "self": map[string]any{}, "root": map[string]any{}}

// FilterEvaluator evaluates `when.expr` with the configured filter adapter
// (celfilter.Use or gomplatefilter.Use). Compiled predicates are cached.
func FilterEvaluator() Evaluator {
	var mu sync.Mutex
	predicates := map[string]filter.Predicate{}
	return func(in ExprInput) (any, error) {
		mu.Lock()
		predicate, ok := predicates[in.Expr]
		if !ok {
			compiled, err := filter.Compile(in.Expr, exprSample)
			if err != nil {
				mu.Unlock()
				return nil, err
			}
			predicate, predicates[in.Expr] = compiled, compiled
		}
		mu.Unlock()
		return predicate.Match(map[string]any{
			"key":   in.Key,
			"value": in.Value,
			"self":  emptyIfNil(in.Self),
			"root":  emptyIfNil(in.Root),
		})
	}
}

func emptyIfNil(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
