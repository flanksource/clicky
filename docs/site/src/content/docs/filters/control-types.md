---
title: Control types
description: How a ListOpts field's Go type picks its filter-bar control, and how to override it.
---

For each filter, the lookup response reports a `type`. clicky-ui uses it to choose the control. The type is inferred from the Go type of the `ListOpts` field the filter is bound to (`Key()`):

| Field type | `type` | `multi` | Control |
| --- | --- | --- | --- |
| `string` | *(empty)* | false | single select / free text |
| `[]string`, `[]int` | *(empty)* | true | multi-select |
| `clicky.MultiFilter` | `multi-filter` | true | multi-select with include / `!exclude` |
| `bool` | `bool` | false | toggle |
| `int` | `number` | false | numeric input with operator |
| `time.Time` named `from` / `*-from` | `from` | false | range start |
| `time.Time` named `to` / `*-to` | `to` | false | range end |
| other `time.Time` | `date` | false | date picker |

A filter that implements `TypedFilter[T]` (`LookupType() string`) overrides the inferred type when it returns a non-empty value. Named filters declare their type in `NamedFilter.Type`: `select` (default), `multi-select`, `date`, `number`, `duration`, `from` or `to`.

## Date ranges

The UI pairs a `from` and a `to` into one range picker only when **both** of these hold:

- both fields are `time.Time`, not `string` or `*time.Time`
- the flag names are `from`/`to`, or end in `-from`/`-to` (e.g. `deployed-from`/`deployed-to`)

```go
type StackListOpts struct {
	From time.Time `flag:"from" help:"Deployed after"`
	To   time.Time `flag:"to"   help:"Deployed before"`
}
```

You do not need `Filter`s for `from` and `to`, because the pairing comes from the field types. Avoid registering them as explicit filter entries: an explicit registration can override the pairing, and the UI then shows two separate text boxes.

If a range must stay a `string` date-math flag at the CLI layer, implement `TypedFilter` and return `"from"`/`"to"` so the web control is still a range.

## Numbers and durations

```go
type latencyFilter struct{}

func (latencyFilter) Key() string                  { return "p99" }
func (latencyFilter) Label() string                { return "p99 latency" }
func (latencyFilter) LookupType() string           { return "duration" }
func (latencyFilter) LookupUnit() string           { return "ms" }  // entity.UnitFilter
func (latencyFilter) LookupDefaultOperator() string { return ">=" } // entity.DefaultOperatorFilter
// Lookup / Options omitted
```

`Unit` and `DefaultOperator` are presentation metadata only. Parsing and converting the operand is still your handler's job.
