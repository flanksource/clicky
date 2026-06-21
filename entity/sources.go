package entity

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
)

// StaticOptions is a FilterSource backed by a fixed set of options, narrowed by
// case-insensitive substring match on the option key or its rendered label.
func StaticOptions(options map[string]api.Textable) FilterSource {
	return staticSource{options: options}
}

type staticSource struct {
	options map[string]api.Textable
}

func (s staticSource) Options(_ FilterContext, query string, limit int) (map[string]api.Textable, int, error) {
	options, total := selectOptions(s.options, query, limit)
	return options, total, nil
}

func (s staticSource) Resolve(_ FilterContext, values []string) (map[string]api.Textable, error) {
	return pickValues(s.options, values), nil
}

// FuncOptions is a FilterSource backed by a user-supplied function — the general
// escape hatch for DB-backed or computed options with server-side search. The
// function owns query matching and the limit cap.
func FuncOptions(fn func(fc FilterContext, query string, limit int) (map[string]api.Textable, int, error)) FilterSource {
	if fn == nil {
		panic("entity.FuncOptions: fn must not be nil")
	}
	return funcSource{fn: fn}
}

type funcSource struct {
	fn func(fc FilterContext, query string, limit int) (map[string]api.Textable, int, error)
}

func (s funcSource) Options(fc FilterContext, query string, limit int) (map[string]api.Textable, int, error) {
	return s.fn(fc, query, limit)
}

func (s funcSource) Resolve(fc FilterContext, values []string) (map[string]api.Textable, error) {
	// Without a get-by-id, label each selected value by matching it against the
	// head option set; unknown values echo back as plain text.
	options, _, err := s.fn(fc, "", 0)
	if err != nil {
		return nil, err
	}
	return pickValues(options, values), nil
}

// EntityOptions is a FilterSource that resolves its options from another
// registered entity: value = GetID(), label = PrettyShort()/GetName(). Search is
// an in-memory substring match over the source entity's list.
func EntityOptions(entityName string) FilterSource {
	if entityName == "" {
		panic("entity.EntityOptions: entityName must not be empty")
	}
	return entitySource{entityName: entityName}
}

type entitySource struct {
	entityName string
}

func (s entitySource) Options(fc FilterContext, query string, limit int) (map[string]api.Textable, int, error) {
	all, err := s.fetch(fc)
	if err != nil {
		return nil, 0, err
	}
	options, total := selectOptions(all, query, limit)
	return options, total, nil
}

func (s entitySource) Resolve(fc FilterContext, values []string) (map[string]api.Textable, error) {
	all, err := s.fetch(fc)
	if err != nil {
		return nil, err
	}
	return pickValues(all, values), nil
}

func (s entitySource) fetch(fc FilterContext) (map[string]api.Textable, error) {
	info, ok := findEntity(s.entityName)
	if !ok {
		return nil, fmt.Errorf("entity %q is not registered (referenced by an EntityOptions filter)", s.entityName)
	}
	op, ok := findListOperation(info)
	if !ok {
		return nil, fmt.Errorf("entity %q has no list operation to source options from", s.entityName)
	}

	var (
		result any
		err    error
	)
	switch {
	case op.ContextDataFunc != nil:
		result, err = op.ContextDataFunc(fc.Ctx(), map[string]string{}, nil)
	case op.DataFunc != nil:
		result, err = op.DataFunc(map[string]string{}, nil)
	default:
		return nil, fmt.Errorf("entity %q list operation has no data function", s.entityName)
	}
	if err != nil {
		return nil, err
	}
	return itemsToOptions(result), nil
}

func findEntity(name string) (EntityInfo, bool) {
	for _, info := range GetEntities() {
		if info.Name == name {
			return info, true
		}
		for _, alias := range info.Aliases {
			if alias == name {
				return info, true
			}
		}
	}
	return EntityInfo{}, false
}

func findListOperation(info EntityInfo) (EntityOperation, bool) {
	for _, op := range info.Operations {
		if op.Verb == "list" {
			return op, true
		}
	}
	return EntityOperation{}, false
}

// itemsToOptions reflects a list result (a slice of EntityItem, or a paged
// result whose Data field is such a slice) into id→label options.
func itemsToOptions(result any) map[string]api.Textable {
	rv := reflect.ValueOf(result)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		if data := rv.FieldByName("Data"); data.IsValid() && data.Kind() == reflect.Slice {
			rv = data
		}
	}
	if rv.Kind() != reflect.Slice {
		return nil
	}

	out := make(map[string]api.Textable, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		el := rv.Index(i)
		item, ok := el.Interface().(EntityItem)
		if !ok {
			continue
		}
		out[item.GetID()] = entityItemLabel(el, item)
	}
	return out
}

func entityItemLabel(el reflect.Value, item EntityItem) api.Textable {
	if short, ok := el.Interface().(api.PrettyShort); ok {
		if t := short.PrettyShort(); t != nil {
			return t
		}
	}
	return api.Text{Content: item.GetName()}
}

// selectOptions narrows options by a case-insensitive substring query over the
// key and rendered label, returning a deterministically-ordered, capped subset
// plus the true total. A non-positive limit means no cap.
func selectOptions(all map[string]api.Textable, query string, limit int) (map[string]api.Textable, int) {
	if len(all) == 0 {
		return nil, 0
	}

	keys := make([]string, 0, len(all))
	q := strings.ToLower(query)
	for key, label := range all {
		if q == "" || optionMatches(key, label, q) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	total := len(keys)
	if query == "" {
		total = len(all)
	}
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}

	out := make(map[string]api.Textable, len(keys))
	for _, key := range keys {
		out[key] = all[key]
	}
	return out, total
}

func optionMatches(key string, label api.Textable, lowerQuery string) bool {
	if strings.Contains(strings.ToLower(key), lowerQuery) {
		return true
	}
	if label != nil && strings.Contains(strings.ToLower(label.String()), lowerQuery) {
		return true
	}
	return false
}

// pickValues returns the labels for the given values, echoing unknown values as
// plain text so a stale selection still renders a chip.
func pickValues(all map[string]api.Textable, values []string) map[string]api.Textable {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]api.Textable, len(values))
	for _, v := range values {
		if label, ok := all[v]; ok {
			out[v] = label
		} else {
			out[v] = api.Text{Content: v}
		}
	}
	return out
}
