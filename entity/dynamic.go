package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api"
)

// DynamicListFunc lists rows of a dynamic entity for the given flag map.
type DynamicListFunc func(ctx context.Context, opts map[string]string) ([]map[string]any, error)

// DynamicGetFunc fetches a single dynamic-entity row by id.
type DynamicGetFunc func(ctx context.Context, id string) (map[string]any, error)

// DynamicEntityBuilder assembles a schema-driven entity and registers it through
// RegisterDynamicEntity. Filters referenced by x-clicky-filter must be
// registered (RegisterFilter / RegisterFilterSpec) before Register is called.
type DynamicEntityBuilder struct {
	name   string
	schema []byte
	listFn DynamicListFunc
	getFn  DynamicGetFunc
}

// NewDynamicEntity starts building a dynamic entity from a JSON schema.
func NewDynamicEntity(name string, schema []byte) *DynamicEntityBuilder {
	return &DynamicEntityBuilder{name: name, schema: schema}
}

func (b *DynamicEntityBuilder) List(fn DynamicListFunc) *DynamicEntityBuilder {
	b.listFn = fn
	return b
}

func (b *DynamicEntityBuilder) Get(fn DynamicGetFunc) *DynamicEntityBuilder {
	b.getFn = fn
	return b
}

// Register parses the schema, generates the runtime types, wires the named
// filters, and registers the entity. It panics on an invalid schema or an
// unregistered filter reference — both are setup-time wiring errors.
func (b *DynamicEntityBuilder) Register() {
	if b.name == "" {
		panic("entity.NewDynamicEntity: name must not be empty")
	}
	if b.listFn == nil {
		panic(fmt.Sprintf("entity.NewDynamicEntity(%q): a List func is required", b.name))
	}

	ps, err := parseSchema(b.schema)
	if err != nil {
		panic(fmt.Sprintf("entity.NewDynamicEntity(%q): %v", b.name, err))
	}

	filters, filterRefs := b.buildFilters(ps)

	spec := DynamicEntitySpec{
		Name:       b.name,
		Parent:     ps.Parent,
		Aliases:    ps.Aliases,
		Icon:       ps.Icon,
		Title:      ps.Title,
		ListType:   ps.listType(),
		ItemType:   ps.itemType(),
		List:       b.listDataFunc(ps),
		Get:        b.getDataFunc(ps),
		Filters:    filters,
		FilterRefs: filterRefs,
	}
	RegisterDynamicEntity(spec)
}

func (b *DynamicEntityBuilder) buildFilters(ps *parsedSchema) ([]DynamicFilter, map[string]string) {
	var filters []DynamicFilter
	refs := map[string]string{}
	for _, f := range ps.Fields {
		if f.Filter == "" {
			continue
		}
		nf := MustGetFilter(f.Filter)
		key := f.Name
		source := nf.Source
		label := f.Label
		if label == "" {
			label = nf.label()
		}
		filters = append(filters, DynamicFilter{
			Key:        key,
			Label:      label,
			Type:       nf.Type,
			Multi:      nf.Multi || f.isArray(),
			Searchable: true,
			Options: func(ctx context.Context, flags map[string]string, query string, limit int) (map[string]api.Textable, int) {
				options, total, err := source.Options(FilterContext{Context: ctx, Key: key, Params: flags}, query, limit)
				if err != nil {
					return nil, 0
				}
				return options, total
			},
			Selected: func(ctx context.Context, flags map[string]string) map[string]api.Textable {
				values := splitValues(flags[key])
				if len(values) == 0 {
					return nil
				}
				selected, err := source.Resolve(FilterContext{Context: ctx, Key: key, Params: flags}, values)
				if err != nil {
					return nil
				}
				return selected
			},
		})
		refs[key] = f.Filter
	}
	if len(refs) == 0 {
		refs = nil
	}
	return filters, refs
}

func (b *DynamicEntityBuilder) listDataFunc(ps *parsedSchema) ContextDataFunc {
	idKey, nameKey := ps.IDKey, ps.NameKey
	return func(ctx context.Context, flags map[string]string, _ []string) (any, error) {
		rows, err := b.listFn(ctx, flags)
		if err != nil {
			return nil, err
		}
		items := make([]dynamicItem, len(rows))
		for i, row := range rows {
			items[i] = dynamicItem{idKey: idKey, nameKey: nameKey, data: row}
		}
		return items, nil
	}
}

func (b *DynamicEntityBuilder) getDataFunc(ps *parsedSchema) ContextDataFunc {
	if b.getFn == nil {
		return nil
	}
	idKey, nameKey := ps.IDKey, ps.NameKey
	return func(ctx context.Context, flags map[string]string, args []string) (any, error) {
		id := flags["id"]
		if len(args) > 0 {
			id = args[0]
		}
		row, err := b.getFn(ctx, id)
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, nil
		}
		return dynamicItem{idKey: idKey, nameKey: nameKey, data: row}, nil
	}
}

// dynamicItem adapts a map[string]any row to EntityItem and injects the
// _id field on marshal, mirroring the static entityWithID wrapper.
type dynamicItem struct {
	idKey   string
	nameKey string
	data    map[string]any
}

func (d dynamicItem) GetID() string   { return toStringValue(d.data[d.idKey]) }
func (d dynamicItem) GetName() string { return toStringValue(d.data[d.nameKey]) }

// PrettyRow exposes the map-backed row to clicky's table formatter, which
// reflects exported struct fields and so would otherwise emit empty cells for a
// dynamic entity. Returning the data keyed by column name lets the clicky-json
// table (and every other renderer) populate cells from the underlying map.
func (d dynamicItem) PrettyRow(_ interface{}) map[string]api.Text {
	out := make(map[string]api.Text, len(d.data))
	for k, v := range d.data {
		out[k] = api.Text{Content: toStringValue(v)}
	}
	return out
}

func (d dynamicItem) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(d.data)+1)
	for k, v := range d.data {
		out[k] = v
	}
	if _, exists := out["_id"]; !exists {
		out["_id"] = d.GetID()
	}
	return json.Marshal(out)
}

func splitValues(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toStringValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}
