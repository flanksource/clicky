---
title: Schema-driven entities
description: Register an entity from a JSON Schema at runtime, with no Go struct.
---

A **dynamic entity** is defined by a JSON Schema instead of Go types. Use one when the shape is only known at runtime, for example a user-defined table, a plugin's resource or a configuration-driven view. It goes through the same pipeline as a static entity: CLI generation, HTTP routes, OpenAPI and the lookup endpoint.

```go
import "github.com/flanksource/clicky/entity"

//go:embed incidents.schema.json
var incidentSchema []byte

func init() {
	entity.RegisterFilter(entity.NamedFilter{
		Name:   "severity",
		Source: entity.StaticOptions(map[string]api.Textable{
			"sev1": clicky.Text("SEV1", "text-red-600"),
			"sev2": clicky.Text("SEV2", "text-amber-600"),
		}),
	})

	entity.NewDynamicEntity("incidents", incidentSchema).
		List(func(ctx context.Context, flags map[string]string) ([]map[string]any, error) {
			return db.Incidents(ctx, flags["severity"], flags["owner"])
		}).
		Get(func(ctx context.Context, id string) (map[string]any, error) {
			return db.Incident(ctx, id)
		}).
		Filter("since", "time-window"). // a filter for a request key no property declares
		Register()
}
```

`List` is required. `Get` is optional. Filters referenced from the schema must already be registered as [named filters](/filters/named-filters/). `Register()` panics on an invalid schema or an unknown filter reference.

## Schema keywords

```json
{
  "type": "object",
  "x-clicky-title": "Incidents",
  "x-clicky-parent": "ops",
  "x-clicky-aliases": ["incident", "inc"],
  "x-clicky-icon": "siren",
  "x-clicky-path": "reliability/incidents",
  "properties": {
    "id":       { "type": "string", "x-clicky-id": true },
    "title":    { "type": "string", "x-clicky-name": true, "x-clicky-label": "Title" },
    "severity": { "type": "string", "x-clicky-filter": "severity" },
    "owner":    { "type": "string", "x-clicky-filter": "users", "x-clicky-filter-key": "owner" },
    "tags":     { "type": "array", "items": { "type": "string" }, "x-clicky-filter": "tags" },
    "opened":   { "type": "string", "format": "date-time", "x-clicky-format": "date" },
    "summary":  { "type": "string", "x-clicky-short": true }
  }
}
```

| Keyword | Where | Meaning |
| --- | --- | --- |
| `x-clicky-id` | property | **Required, exactly one.** The row ID (`GetID`) |
| `x-clicky-name` | property | the row name. At most one; defaults to the ID property |
| `x-clicky-filter` | property | named filter backing this property. The property becomes a CLI flag and a query parameter |
| `x-clicky-filter-key` | property | the flag or query key for that filter. Defaults to the property name |
| `x-clicky-label` | property | column label |
| `x-clicky-format` | property | clicky `pretty` format (e.g. `date`, `currency`) |
| `x-clicky-short` | property | render via `PrettyShort` in table cells |
| `x-clicky-parent`, `x-clicky-aliases`, `x-clicky-icon`, `x-clicky-path`, `x-clicky-title` | root | same as the static entity fields |

JSON types map to Go as `integer` → `int64`, `number` → `float64`, `boolean` → `bool`, `array` → slice, `object` → `map[string]any`, and anything else → `string`. These types are used only for the OpenAPI response schema. Rows stay `map[string]any`, and each row gets an `_id` field.

Only properties with `x-clicky-filter` become list flags. They are `string`, or `[]string` for arrays, and the list function receives them in `flags`.

## Filters on dynamic entities

Filters are wired from named-filter sources:

- each property with `x-clicky-filter`, bound under its filter key
- each `.Filter(key, filterName)` call, for keys that no property carries (for example a query-only window)

A schema property always owns its key. Declaring the same key twice panics. Dynamic filters are always searchable, and the source receives the whole flag map as `FilterContext.Params`, so cascading works as it does for static entities. When the selection is resolved, a leading `!` (exclude) is stripped before `Resolve` is called.

## Lower-level registration

`NewDynamicEntity` builds a `DynamicEntitySpec` and calls `clicky.RegisterDynamicEntity(spec)`. Call that directly if you generate the list and item types and the `DynamicFilter` closures yourself. You can supply `CountedOptions` for per-value counts, `TimeEnabled` for range controls, and `Limit`.
