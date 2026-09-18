---
title: Custom commands
description: Typed commands outside the CRUD model that still join the CLI, HTTP and MCP surfaces.
---

Some operations are neither CRUD nor actions on a row, such as a report, a summary or an import. You can register them as typed commands. They then get the same flag binding, rendering, HTTP route and MCP tool as entities.

## AddCommand

```go
type SummaryOptions struct {
	Team  string    `flag:"team"`
	Since time.Time `flag:"since" default:"now-7d"`
}

clicky.AddCommand(root, SummaryOptions{}, func(opts SummaryOptions) (Summary, error) {
	return store.Summarize(opts)
})
```

| Function | Name | Handler |
| --- | --- | --- |
| `AddCommand(parent, opts, fn)` | kebab-case of the opts type name minus `Options` (`SummaryOptions` → `summary`) | `func(T) (R, error)` |
| `AddNamedCommand(name, parent, opts, fn)` | `name` | `func(T) (R, error)` |
| `AddCommandWithContext(parent, opts, fn)` | derived | `func(ctx, T) (R, error)` |
| `AddNamedCommandWithContext(name, parent, opts, fn)` | `name` | `func(ctx, T) (R, error)` |

Optional interfaces on the opts struct:

- `GetName() string` (`clicky.Name`) overrides the command's `Use`.
- `Help() api.Textable` (`clicky.Help`) supplies the long help.
- `Filters() []clicky.Filter[T]` (`clicky.Filterable[T]`) attaches filters to this command. See below.

## Attach under an entity: RegisterSubCommand

Entity parent commands do not exist until `GenerateCLI` runs. To add a command under one, defer it:

```go
// A ready-made cobra command
clicky.RegisterSubCommand("stack", &cobra.Command{Use: "seed", RunE: seed})

// Built lazily against the parent — the usual pairing with AddNamedCommand
clicky.RegisterSubCommandFn("stack", func(parent *cobra.Command) {
	clicky.AddNamedCommand("summary", parent, StackSummaryOpts{}, store.SummarizeStacks)
})

// Slash paths create intermediate grouping commands as needed
clicky.RegisterSubCommandFn("billing/policy", func(parent *cobra.Command) { /* ... */ })
```

A command attached under an entity is annotated as a collection-scoped action of that entity. clicky-ui then lists it on the entity's page, and the route is `/api/v1/stack/summary`.

:::caution
The `clicky lint` analyzer reports a hand-built `cobra.Command` with `Run`/`RunE` as an error. It bypasses the generated surfaces. Prefer `AddNamedCommand` inside `RegisterSubCommandFn`.
:::

## Filters on custom commands: Filterable and LiftFilters

A subcommand's flags often differ from the parent list. For example, an `activities <id>` command drops the identifier filters that the positional ID already fixes. Implement `Filterable[T]` to give the command its own lookup and completion surface:

```go
type ActivityOpts struct {
	StackID string `args:"true" required:"true"`
	ActivityFilter // embedded, so its flag-tagged fields bind on the command
}

type ActivityFilter struct {
	Kind string    `flag:"kind"`
	From time.Time `flag:"from"`
	To   time.Time `flag:"to"`
}

func (ActivityOpts) Filters() []clicky.Filter[ActivityOpts] {
	return clicky.LiftFilters(activityFilters, func(o *ActivityOpts) *ActivityFilter { return &o.ActivityFilter })
}
```

`LiftFilters` adapts a `[]Filter[Inner]` to `[]Filter[Outer]` through a projection function. You write filter implementations once against the small filter struct and reuse them from any options struct that contains it. Searchable, context-aware, typed, unit and default-operator behaviour all pass through.
