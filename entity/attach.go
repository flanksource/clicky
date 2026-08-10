package entity

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/flags"
)

// Use attaches a registered named filter to a typed (static) entity. The returned
// value implements clicky.Filter[ListOpts] (plus the searchable/context/typed
// extensions) so it drops straight into Entity.Filters. By default it binds to the
// flag/field named after the filter; override with As.
//
//	Filters(entity.Use[TaskOpts]("users").As("owner"))
//
// Use resolves the filter eagerly and panics if the name is unregistered, so a
// wiring mistake fails at setup rather than at request time.
func Use[ListOpts any](filterName string) *attachedFilter[ListOpts] {
	return &attachedFilter[ListOpts]{nf: MustGetFilter(filterName), key: filterName}
}

// attachedFilter adapts a type-agnostic NamedFilter to the typed
// clicky.Filter[ListOpts] interface, bridging the typed ListOpts to a
// FilterContext via reflection over its flag-tagged fields.
type attachedFilter[ListOpts any] struct {
	nf  NamedFilter
	key string
}

// As binds the filter to a specific flag/field on the entity's ListOpts.
func (a *attachedFilter[ListOpts]) As(field string) *attachedFilter[ListOpts] {
	if field == "" {
		panic("entity: As field must not be empty")
	}
	a.key = field
	return a
}

func (a *attachedFilter[ListOpts]) Key() string   { return a.key }
func (a *attachedFilter[ListOpts]) Label() string { return a.nf.label() }

// FilterName reports the reusable named filter this adapter references, so the
// OpenAPI layer can emit an x-clicky-lookup $ref to its shared definition.
func (a *attachedFilter[ListOpts]) FilterName() string { return a.nf.Name }

// LookupType returns the filter's own control type, which the root lookup uses
// to override field-inferred type only when non-empty (an empty type leaves the
// inferred bool/number/date/multi-filter type in place).
func (a *attachedFilter[ListOpts]) LookupType() string { return a.nf.Type }

// LookupLimit returns the filter's own option cap, or zero to take the default.
func (a *attachedFilter[ListOpts]) LookupLimit() int { return a.nf.Limit }

// Lookup labels the currently-selected value(s) of the bound field. It does not
// mutate opts — named filters carry no backend transform.
func (a *attachedFilter[ListOpts]) Lookup(opts *ListOpts) (map[string]api.Textable, error) {
	values := fieldValue(opts, a.key)
	if len(values) == 0 {
		return nil, nil
	}
	return a.nf.Source.Resolve(a.context(context.TODO(), opts), values)
}

// Options resolves the full option set. The plain (error-less) Filter interface
// forces a best-effort result here; the searchable/context variants the root
// prefers for web lookups surface errors and the request context.
func (a *attachedFilter[ListOpts]) Options(opts ListOpts) map[string]api.Textable {
	options, _, _ := a.nf.Source.Options(a.context(context.TODO(), &opts), "", 0)
	return options
}

func (a *attachedFilter[ListOpts]) OptionsWithQuery(opts ListOpts, query string, limit int) (map[string]api.Textable, int) {
	options, total, _ := a.nf.Source.Options(a.context(context.TODO(), &opts), query, limit)
	return options, total
}

func (a *attachedFilter[ListOpts]) OptionsWithContext(ctx context.Context, opts ListOpts) map[string]api.Textable {
	options, _, _ := a.nf.Source.Options(a.context(ctx, &opts), "", 0)
	return options
}

func (a *attachedFilter[ListOpts]) OptionsWithQueryAndContext(ctx context.Context, opts ListOpts, query string, limit int) (map[string]api.Textable, int) {
	options, total, _ := a.nf.Source.Options(a.context(ctx, &opts), query, limit)
	return options, total
}

func (a *attachedFilter[ListOpts]) context(ctx context.Context, opts *ListOpts) FilterContext {
	return FilterContext{
		Context: ctx,
		Key:     a.key,
		Params:  structToFlagMap(opts),
	}
}

// structToFlagMap reflects a typed opts struct back into the flag map a
// FilterSource expects, so sibling selections drive cascading. It is the inverse
// of the root buildOpts conversion; zero-valued fields are omitted.
func structToFlagMap(opts any) map[string]string {
	sv := addressableStruct(opts)
	if !sv.IsValid() {
		return map[string]string{}
	}
	fields, err := flags.ParseStructFields(sv.Type())
	if err != nil {
		return map[string]string{}
	}

	out := make(map[string]string, len(fields))
	for _, f := range fields {
		if f.FlagName == "" || f.FlagName == "-" {
			continue
		}
		if vals := fieldStrings(flags.GetFieldByPath(sv, f.FieldPath)); len(vals) > 0 {
			out[f.FlagName] = strings.Join(vals, ",")
		}
	}
	return out
}

// fieldValue returns the current string value(s) of the flag-tagged field bound
// to key, or nil when unset.
func fieldValue(opts any, key string) []string {
	sv := addressableStruct(opts)
	if !sv.IsValid() {
		return nil
	}
	fields, err := flags.ParseStructFields(sv.Type())
	if err != nil {
		return nil
	}
	for _, f := range fields {
		if f.FlagName == key {
			return fieldStrings(flags.GetFieldByPath(sv, f.FieldPath))
		}
	}
	return nil
}

// addressableStruct dereferences opts to its struct value and returns an
// addressable copy (GetFieldByPath initializes nil embedded pointers, which
// requires addressability).
func addressableStruct(opts any) reflect.Value {
	v := reflect.ValueOf(opts)
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	addr := reflect.New(v.Type())
	addr.Elem().Set(v)
	return addr.Elem()
}

// fieldStrings renders a field value to its string form(s), returning nil for
// zero values so they are treated as "unset".
func fieldStrings(v reflect.Value) []string {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		if v.String() == "" {
			return nil
		}
		return []string{v.String()}
	case reflect.Bool:
		if !v.Bool() {
			return nil
		}
		return []string{"true"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Named numeric types (e.g. duration.Duration) render via Stringer.
		if s, ok := v.Interface().(fmt.Stringer); ok {
			return nonEmpty(s.String())
		}
		if v.Int() == 0 {
			return nil
		}
		return []string{strconv.FormatInt(v.Int(), 10)}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.Uint() == 0 {
			return nil
		}
		return []string{strconv.FormatUint(v.Uint(), 10)}
	case reflect.Float32, reflect.Float64:
		if v.Float() == 0 {
			return nil
		}
		return []string{strconv.FormatFloat(v.Float(), 'g', -1, 64)}
	case reflect.Slice:
		var out []string
		for i := 0; i < v.Len(); i++ {
			out = append(out, fieldStrings(v.Index(i))...)
		}
		return out
	case reflect.Struct:
		if t, ok := v.Interface().(time.Time); ok {
			if t.IsZero() {
				return nil
			}
			return []string{t.Format(time.RFC3339)}
		}
		if s, ok := v.Interface().(fmt.Stringer); ok {
			return nonEmpty(s.String())
		}
		return nil
	default:
		if v.IsZero() {
			return nil
		}
		if s, ok := v.Interface().(fmt.Stringer); ok {
			return nonEmpty(s.String())
		}
		return []string{fmt.Sprint(v.Interface())}
	}
}

func nonEmpty(s string) []string {
	if s == "" || s == "0s" {
		return nil
	}
	return []string{s}
}
