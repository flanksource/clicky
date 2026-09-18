---
title: Named filters
description: Define a filter once and attach it by name to static and dynamic entities.
---

A struct that implements `Filter[ListOpts]` belongs to one options type. A **named filter** is type-agnostic. You register it once under a unique name, then attach it to any number of entities, including [dynamic entities](/dynamic/schema-entities/) that have no Go options struct.

Named filters live in the `entity` package:

```go
import "github.com/flanksource/clicky/entity"
```

## Register

```go
func init() {
	entity.RegisterFilter(entity.NamedFilter{
		Name:  "users",
		Label: "User",
		Multi: true,
		Limit: 50,
		Source: entity.EntityOptions("users"), // options come from the users entity
	})
}
```

| Field | Meaning |
| --- | --- |
| `Name` | unique registry key; a duplicate panics |
| `Label` | control label; defaults to `Name` |
| `Type` | `select` (default), `multi-select`, `date`, `number`, `duration`, `from`, `to` |
| `Multi` | accepts multiple values (implies `multi-select` when `Type` is empty) |
| `Unit`, `DefaultOperator` | number/duration presentation metadata |
| `Limit` | per-filter option cap (≤ `entity.MaxLookupOptions`) |
| `Source` | where options come from; required |

`RegisterFilter` panics on an empty name, a nil source, a duplicate name or an invalid default operator. These are wiring bugs and should fail at startup.

## Attach

```go
clicky.NewEntity[Task, TaskOpts, Task]("tasks").
	Filters(
		entity.Use[TaskOpts]("users").As("owner"),   // bind "users" to the --owner flag
		entity.Use[TaskOpts]("teams"),              // binds to the --teams flag
	).
	List(listTasks).
	Register()
```

- `Use[ListOpts](name)` looks the filter up immediately and **panics if it is not registered**. Register named filters before the entities that use them. The simplest way is to call `RegisterFilter` earlier in the same setup function (or `init()`) that registers the entity.
- `.As(field)` binds to a different flag than the filter's name.
- The adapter implements every optional filter interface (searchable, context, typed, unit, default operator, limited). It also records the reference, so OpenAPI emits a `$ref` to the shared definition.

A named filter **does not transform** values. Its `Lookup` only labels the selection. If you need canonicalization, write a typed `Filter`.

## Sources

A `FilterSource` resolves options from a `FilterContext`, not from a typed struct:

```go
type FilterContext struct {
	Context context.Context   // request context (may be nil outside a request; use fc.Ctx())
	Key     string            // the bound key on this entity, e.g. "owner"
	Params  map[string]string // every current flag value — use for cascading
}
```

| Constructor | Options from | Search | Counts |
| --- | --- | --- | --- |
| `entity.StaticOptions(map[string]api.Textable)` | a fixed map | case-insensitive substring on key and label | — |
| `entity.EntityOptions("users")` | another registered entity's list: value = `GetID()`, label = `PrettyShort()` or `GetName()` | in-memory substring | — |
| `entity.CountedOptions(fn)` | your function returning `FilterOptions` | yours | ✓ |
| `entity.FuncOptions(fn)` | your function returning `(options, total, err)` | yours | — *(deprecated; use `CountedOptions`)* |
| custom type implementing `FilterSource` / `CountedFilterSource` | anything | yours | optional |

### Counted options

`CountedOptions` also reports how many rows hold each value. The UI shows the counts next to the options:

```go
entity.RegisterFilter(entity.NamedFilter{
	Name: "status",
	Source: entity.CountedOptions(func(fc entity.FilterContext, q string, limit int) (entity.FilterOptions, error) {
		// fc.Params["team"] narrows the counts to the selected team (cascading).
		rows, err := db.StatusCounts(fc.Ctx(), fc.Params["team"], q, limit)
		if err != nil {
			return entity.FilterOptions{}, err
		}
		out := entity.FilterOptions{Options: map[string]api.Textable{}, Counts: map[string]int{}, Total: len(rows)}
		for _, r := range rows {
			out.Options[r.Status] = clicky.Text(r.Status)
			out.Counts[r.Status] = r.N
		}
		return out, nil
	}),
})
```

Rules for `FilterOptions`:

- Key `Counts` like `Options`. A count for a value you did not offer is an error.
- Leave a value out of `Counts` if you do not know its count. Do not report `0`. Negative counts are an error.
- `Total` is the true number of options behind a possibly capped `Options`.

Source errors reach the lookup response as a failed request. They are not swallowed into an empty set.

### Resolving the selection

`Resolve(fc, values)` labels the currently selected values. The built-in sources label them from their option set. A value without a known label is echoed back as plain text, so a stale selection still renders as a chip.

## Declarative filters (JSON / YAML)

Static and entity sources can be declared as data:

```yaml
name: region
label: Region
type: select
source:
  kind: static
  options:
    eu-west-1: Europe (Ireland)
    us-east-1: US East (N. Virginia)
---
name: owner
multi: true
source:
  kind: entity
  entity: users
```

```go
var spec entity.FilterSpec
_ = yaml.Unmarshal(data, &spec)
entity.RegisterFilterSpec(spec) // panics on an invalid spec
// or: nf, err := entity.FilterFromSpec(spec) to validate without registering
```

`NamedFilter.Spec()` produces the same shape. That shape is what OpenAPI publishes under `components.x-clicky-filters`. A function-backed or custom source is published with `kind: func` or `kind: custom`, and its options are resolved on the server through the lookup endpoint.
