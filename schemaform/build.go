package schemaform

import (
	"sort"
	"strings"
)

// objectList is an array-of-objects field: its items are collected with one
// sub-form per item, between the form segments around it.
type objectList struct {
	st      *State
	path    []string
	key     string
	base    *Schema
	section string
}

type builder struct {
	st            *State
	lookup        *lookupBinding
	requiredFirst bool
	inline        bool
	loadSchema    SchemaLoader
}

// build flattens the schema into form steps (*field or *objectList) in render
// order. Every key any branch can declare gets a step per control kind it can
// take; visibility is decided live from the State, so one built form serves
// every branch, listener and discriminator state.
func (b *builder) build(schema *Schema, path []string, section string) ([]any, error) {
	keys, cands := candidates(schema)
	var steps []any
	for _, key := range b.order(schema, keys, cands) {
		childPath := appendPath(path, key)
		if slotted(cands[key]) {
			slot, err := b.slot(schema, key, childPath, section)
			if err != nil {
				return nil, err
			}
			steps = append(steps, slot)
			continue
		}
		for _, group := range groupByKind(cands[key]) {
			step, err := b.step(key, childPath, section, group)
			if err != nil {
				return nil, err
			}
			if key == schema.Discriminator {
				for _, s := range step {
					if f, ok := s.(*field); ok {
						f.discriminator = true
					}
				}
			}
			steps = append(steps, step...)
		}
	}
	return steps, nil
}

func (b *builder) step(key string, path []string, section string, group kindGroup) ([]any, error) {
	base := group.schemas[0]
	switch group.kind {
	case KindObject:
		object := base
		if len(group.schemas) > 1 {
			object = &Schema{Type: TypeList{"object"}, AllOf: group.schemas}
		}
		return b.build(object, path, joinSection(section, base.Label(key)))
	case KindObjectList:
		return []any{&objectList{st: b.st, path: path, key: key, base: base, section: section}}, nil
	}
	f := &field{st: b.st, path: path, key: key, kind: group.kind, base: base, section: section, inline: b.inline}
	if err := f.buildControl(b.lookup); err != nil {
		return nil, err
	}
	f.buildNote()
	b.st.watch(path, f.resync)
	return []any{f}, nil
}

// slotted reports whether any declaration of a key loads its schema at run
// time (x-clicky-schema).
func slotted(schemas []*Schema) bool {
	for _, s := range schemas {
		if s.SchemaSource != nil {
			return true
		}
	}
	return false
}

func joinSection(section, label string) string {
	if section == "" || label == "" {
		return section + label
	}
	return section + " › " + label
}

type kindGroup struct {
	kind    Kind
	schemas []*Schema
}

func groupByKind(schemas []*Schema) []kindGroup {
	var out []kindGroup
	index := map[Kind]int{}
	for _, s := range schemas {
		kind := ResolveControl(s).Kind
		if i, ok := index[kind]; ok {
			out[i].schemas = append(out[i].schemas, s)
			continue
		}
		index[kind] = len(out)
		out = append(out, kindGroup{kind: kind, schemas: []*Schema{s}})
	}
	return out
}

// order applies clicky-ui's render order: document order, then the object's
// x-order, then per-property x-clicky-order; optionally required first. The
// x-discriminator key always leads, since its value picks the branch.
func (b *builder) order(schema *Schema, keys []string, cands map[string][]*Schema) []string {
	ordered := orderByXOrder(keys, schema.Order)
	rank := func(key string) float64 {
		for _, s := range cands[key] {
			if s.ClickyOrder != nil {
				return *s.ClickyOrder
			}
		}
		return float64(1 << 62)
	}
	sort.SliceStable(ordered, func(i, j int) bool { return rank(ordered[i]) < rank(ordered[j]) })
	if b.requiredFirst {
		ordered = partition(ordered, func(k string) bool { return contains(schema.Required, k) })
	}
	if d := schema.Discriminator; d != "" {
		ordered = partition(ordered, func(k string) bool { return k == d })
	}
	return ordered
}

func orderByXOrder(keys []string, order []string) []string {
	if len(order) == 0 {
		return append([]string(nil), keys...)
	}
	out := make([]string, 0, len(keys))
	for _, k := range order {
		if contains(keys, k) {
			out = append(out, k)
		}
	}
	for _, k := range keys {
		if !contains(order, k) {
			out = append(out, k)
		}
	}
	return out
}

// partition stably moves the keys matching first to the front.
func partition(keys []string, first func(string) bool) []string {
	var head, tail []string
	for _, k := range keys {
		if first(k) {
			head = append(head, k)
		} else {
			tail = append(tail, k)
		}
	}
	return append(head, tail...)
}

// segments splits steps into runs of fields separated by object lists.
func segments(steps []any) [][]any {
	var out [][]any
	var current []any
	for _, step := range steps {
		if _, ok := step.(*objectList); ok {
			if len(current) > 0 {
				out = append(out, current)
			}
			out = append(out, []any{step})
			current = nil
			continue
		}
		current = append(current, step)
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

var defaultItemTitleKeys = []string{"title", "name", "label", "id", "key"}

// itemTitle summarises an array item the way the x-item row title does.
func itemTitle(spec *ItemSpec, item map[string]any, index int) string {
	keys := defaultItemTitleKeys
	if spec != nil && len(spec.Title) > 0 {
		keys = spec.Title
	}
	for _, k := range keys {
		if v := formatScalar(item[k]); strings.TrimSpace(v) != "" && !isEmpty(item[k]) {
			return v
		}
	}
	if spec != nil && spec.Fallback != "" {
		return spec.Fallback
	}
	return "Item " + formatScalar(float64(index+1))
}

func itemNoun(array *Schema) (noun, plural string) {
	noun, plural = "item", "items"
	if array.Items != nil && array.Items.Title != nil && *array.Items.Title != "" {
		noun = *array.Items.Title
	}
	if array.Title != nil && *array.Title != "" {
		plural = *array.Title
	}
	if array.Item != nil {
		if array.Item.Noun != "" {
			noun = array.Item.Noun
		}
		if array.Item.NounPlural != "" {
			plural = array.Item.NounPlural
		}
	}
	return noun, plural
}
