---
title: Filters
description: The Filter interface, how it normalizes values, labels selections and enumerates options.
---

A **filter** belongs to one `ListOpts` field. It has three jobs:

1. **Normalize.** It rewrites the raw value the user typed into the shape the backend expects, before the list handler runs.
2. **Label the selection.** It says how the currently selected values should render, for example as chips in the filter bar.
3. **Enumerate options.** It lists the values the user can pick. These feed dropdowns, type-ahead search and shell completion.

```go
type Filter[ListOpts any] interface {
	Key() string                                            // the flag name it is bound to
	Label() string                                          // human label for the control
	Lookup(opts *ListOpts) (map[string]api.Textable, error) // normalize + label the selection
	Options(opts ListOpts) map[string]api.Textable          // available values -> labels
}
```

## Example

```go
type teamFilter struct{}

func (teamFilter) Key() string   { return "team" }
func (teamFilter) Label() string { return "Team" }

// Lookup runs before List. It may mutate opts.
func (teamFilter) Lookup(opts *StackListOpts) (map[string]api.Textable, error) {
	if opts.Team == "" {
		return nil, nil
	}
	opts.Team = canonicalTeam(opts.Team) // "platform" -> "team/platform"
	return map[string]api.Textable{
		opts.Team: clicky.Text(labelForTeam(opts.Team), "font-semibold"),
	}, nil
}

// Options can cascade on sibling selections.
func (teamFilter) Options(opts StackListOpts) map[string]api.Textable {
	all := map[string]api.Textable{
		"team/platform": clicky.Text("Platform"),
		"team/core":     clicky.Text("Core"),
		"team/data":     clicky.Text("Data"),
	}
	if strings.EqualFold(opts.Region, "us-east-1") {
		return map[string]api.Textable{"team/core": all["team/core"]}
	}
	return all
}

clicky.NewEntity[Stack, StackListOpts, Stack]("stack").
	Filters(teamFilter{}, statusFilter{}).
	List(store.ListStacks)
```

## When filters run

| Situation | What clicky calls |
| --- | --- |
| `list`, the bare entity command, `GET /api/v1/<entity>` | `BuildOpts` → every filter's `Lookup(&opts)` → your `List(opts)` |
| Filter-mode bulk action | same resolution, then your filter handler |
| Lookup request (`?__lookup=filters`) | `Lookup` for the selected labels, then `Options` for each filter |
| Shell completion of `--team` | `Options(opts)` with the *other* flags already typed, then prefix-matched |

A `Lookup` error fails the request and names the filter: `team: <err>`.

### Cascading

`Options` receives the current `ListOpts`, with every other selection already decoded and normalized. Narrow the option set on sibling values, as the `us-east-1` → `team/core` case above does. Shell completion does the same: `app stack --region us-east-1 --team <TAB>` offers only `team/core`.

## Labels are rich text

Option and selection labels are `api.Textable`. You can use styling, icons and tooltips, and they render as ANSI in completions and as styled chips in the UI:

```go
api.Text{Content: "Degraded", Style: "text-amber-600", Tooltip: api.Text{Content: "status:degraded"}}
```

The map **key** is the value that is sent back. The **label** is only what the user sees.

## Optional interfaces

A filter can implement any of these to change its control or its option source:

| Interface | Package | Purpose |
| --- | --- | --- |
| `TypedFilter[T]`: `LookupType() string` | `clicky` | override the inferred control type (e.g. a `string` date-math flag shown as `from`) |
| `UnitFilter`: `LookupUnit() string` | `entity` | unit label for number/duration controls (`"ms"`, `"GiB"`) |
| `DefaultOperatorFilter`: `LookupDefaultOperator() string` | `entity` | default comparison (`>`, `>=`, `<`, `<=`) for number/duration controls |
| `LimitedFilter`: `LookupLimit() int` | `entity` | cap this filter's option set below the 200 ceiling |
| `SearchableFilter[T]`: `OptionsWithQuery(opts, q, limit)` | `clicky` | [server-side search](/filters/searchable/) for large option sets |
| `ContextFilter[T]`: `OptionsWithContext(ctx, opts)` | `clicky` | resolve options from request-scoped state |
| `ContextSearchableFilter[T]`: `OptionsWithQueryAndContext(ctx, opts, q, limit)` | `clicky` | both of the above |

A default operator on a filter that is not `number` or `duration`, or an operator outside the four above, fails the lookup.

## Reusable filters

A struct that implements `Filter` is tied to one `ListOpts` type. If the same filter (users, teams, regions) appears on many entities, register it once as a [named filter](/filters/named-filters/) and attach it with `entity.Use[ListOpts]("users").As("owner")`.
