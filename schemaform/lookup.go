package schemaform

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/charmbracelet/huh"
)

// LookupRequest asks for the options of an `x-clicky-lookup` field. Query is
// the search text (empty for the head set); Values is the form's current
// value, which scopes dependent lookups.
type LookupRequest struct {
	Path   []string
	Spec   LookupSpec
	Query  string
	Values map[string]any
}

// LookupResult is a lookup's option set. Truncated means more options exist
// than were returned and the field should search as the user types.
type LookupResult struct {
	Options   []Option
	Multi     bool
	Truncated bool
	Total     int
}

// Lookup resolves `x-clicky-lookup` options, e.g. by calling an operation's
// lookup endpoint (`__lookup_filter` / `__lookup_q`).
type Lookup func(ctx context.Context, req LookupRequest) (LookupResult, error)

type lookupBinding struct {
	ctx   context.Context
	fn    Lookup
	mu    sync.Mutex
	cache map[string]LookupResult
}

func (b *lookupBinding) fetch(st *State, path []string, spec LookupSpec, query string) (LookupResult, error) {
	values := st.snapshot()
	scope, err := json.Marshal(values)
	if err != nil {
		return LookupResult{}, err
	}
	key := spec.Filter + "\x00" + query + "\x00" + string(scope)
	b.mu.Lock()
	cached, ok := b.cache[key]
	b.mu.Unlock()
	if ok {
		return cached, nil
	}
	result, err := b.fn(b.ctx, LookupRequest{Path: path, Spec: spec, Query: query, Values: values})
	if err != nil {
		return LookupResult{}, fmt.Errorf("lookup %s for %s: %w", spec.Filter, joinPath(path), err)
	}
	b.mu.Lock()
	b.cache[key] = result
	b.mu.Unlock()
	return result, nil
}

// buildLookup picks the widget from a head fetch: a multi lookup is a
// multi-select, a truncated single lookup searches as the user types (and
// accepts free text, as the web combobox does), anything else is a select.
func (f *field) buildLookup(b *lookupBinding) error {
	spec := *f.base.Lookup
	if b == nil || b.fn == nil {
		return fmt.Errorf("%s: x-clicky-lookup %q needs a Lookup in Options", joinPath(f.path), spec.Filter)
	}
	head, err := b.fetch(f.st, f.path, spec, "")
	if err != nil {
		return err
	}
	var lookupErr error
	options := func() []Option {
		result, err := b.fetch(f.st, f.path, spec, "")
		if err != nil {
			lookupErr = err
			f.st.fail(err)
			return nil
		}
		return withCurrentValues(result.Options, f.value())
	}
	withLookupErr := func(check func(any) error) func(any) error {
		return func(v any) error {
			if lookupErr != nil {
				return lookupErr
			}
			return check(v)
		}
	}
	switch {
	case spec.Multi || head.Multi:
		f.check = withLookupErr(func(v any) error {
			return f.checkItems(v, func(any) error { return nil })
		})
		f.buildMultiSelect(options)
	case head.Truncated:
		f.check = withLookupErr(f.checkString)
		in := f.buildInput(stringCodec, f.placeholder(fmt.Sprintf("type to search %d options", head.Total)))
		in.SuggestionsFunc(func() []string {
			text, _ := f.value().(string)
			result, err := b.fetch(f.st, f.path, spec, text)
			if err != nil {
				lookupErr = err
				f.st.fail(err)
				return nil
			}
			out := make([]string, len(result.Options))
			for i, o := range result.Options {
				out[i] = o.Value
			}
			return out
		}, &f.st.rev)
	default:
		f.check = withLookupErr(f.requiredCheck)
		f.buildSelect(options)
	}
	return nil
}

// withCurrentValues appends the field's current value(s) the lookup does not
// list — a suggest-only lookup (shell completion) accepts values outside its
// suggestions, e.g. one passed as a flag — so the select keeps them instead of
// replacing them with the highlighted option.
func withCurrentValues(options []Option, current any) []Option {
	values, ok := current.([]any)
	if !ok {
		values = []any{current}
	}
	out := options
	for _, v := range values {
		value := formatScalar(v)
		if value == "" || containsOption(out, value) {
			continue
		}
		out = append(append([]Option(nil), out...), Option{Value: value, Label: value, Raw: v})
	}
	return out
}

func containsOption(options []Option, value string) bool {
	for _, o := range options {
		if o.Value == value {
			return true
		}
	}
	return false
}

var _ huh.Accessor[string] = (*accessor[string])(nil)
