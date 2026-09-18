---
title: Bulk actions
description: One verb applied to many rows, selected by explicit IDs or by the entity's filters.
---

A **bulk action** is one operation aimed at a selection of rows. clicky supports two ways to select them:

- **ID mode**: the caller names the rows (`pause stk-001 stk-002`). This is what a table's selection toolbar sends.
- **Filter mode**: the caller describes the rows with the entity's own `ListOpts` flags (`--team platform --status healthy`), and your handler receives the resolved `ListOpts`.

## Constructors

| Constructor | ID mode | Filter mode | Context |
| --- | --- | --- | --- |
| `BulkAction(name, fn)` | ✓ | | |
| `BulkFilterAction[ListOpts](name, fn)` | | ✓ | |
| `BulkActionWithFilter[ListOpts](name, byIDs, byFilter)` | ✓ | ✓ | |
| `BulkActionWithFilterAndContext[ListOpts](name, byIDs, byFilter)` | ✓ | ✓ | ✓ |

```go
clicky.NewEntity[Stack, StackListOpts, StackDetail]("stack").
	Filters(teamFilter{}, statusFilter{}).
	List(store.ListStacks).
	WithBulkAction(
		clicky.BulkActionWithFilterAndContext[StackListOpts](
			"pause",
			func(ctx context.Context, ids []string, flags map[string]string) (PauseReport, error) {
				return store.PauseIDs(ctx, ids)
			},
			func(ctx context.Context, opts StackListOpts, flags map[string]string) (PauseReport, error) {
				// opts has been decoded AND passed through every filter's Lookup,
				// so opts.Team is already canonical ("team/platform").
				return store.PauseMatching(ctx, opts)
			},
		).WithShort("Pause stacks by id or by filter"),
	).
	Register()
```

## Invoking

```bash
# ID mode
app stack pause stk-001 stk-002

# Filter mode: a non-empty --filter switches modes; the ListOpts flags select the rows
app stack pause all --filter "team == 'platform'" --team platform --status healthy
```

Over HTTP, the selection is carried in the `{id}` path segment, comma-joined:

```bash
curl -X POST 'localhost:8080/api/v1/stack/stk-001,stk-002,stk-003/pause'
```

The route is always `/api/v1/<entity>/{id}/<action>`, even when the action's name is a CRUD verb. The HTTP method is inferred from the verb in the same way as for [actions](/entities/actions/#modifiers): `pause` → `POST`, `delete` → `DELETE`. A bulk `delete` aimed at forty rows (`DELETE /api/v1/<entity>/{id}/delete`) is a different operation from the entity's own `DELETE /api/v1/<entity>/{id}`, and both routes exist side by side.

### How the mode is chosen

The generated command checks the flag map:

1. If `filter` is non-empty and a filter handler exists, it calls the **filter handler** with `ListOpts` resolved from the same flags. The resolution uses `BuildOpts` and then every entity filter's `Lookup`.
2. Otherwise it calls the **ID handler** with the positional IDs.
3. An action built with `BulkFilterAction` alone returns `bulk action "<name>" requires filter mode` when it is called without `--filter`.
4. On the CLI, calling it with neither IDs nor `--filter` is an error.

`filter` is the global CEL `--filter` flag that `clicky.BindAllFlags` registers. The filter handler receives it in `flags["filter"]` if you want to apply the expression yourself.

:::note
Filter mode needs a positional operand on the CLI (the `all` placeholder above) because the first ID is also the route's `{id}` segment. The actual selection comes from the filter flags.
:::

In filter mode the command also binds the entity's `ListOpts` flags, completions and lookup, so the UI can reuse the list's filter bar to build a selection.

## Action parameters

The selection (which rows) is independent of the operation's own parameters (what to do to them). Declare the parameters with `WithFlags`:

```go
type RetagFlags struct {
	Tag string `flag:"tag" required:"true"`
}

func (RetagFlags) ClickyActionFlags() {}

clicky.BulkActionWithFilter[StackListOpts]("retag", retagIDs, retagMatching).
	WithFlags(RetagFlags{})
```

```bash
app stack retag stk-001 stk-002 --tag critical
app stack retag all --filter "true" --status degraded --tag critical
```

The parameter flags are bound **after** the `ListOpts` selection flags. If a parameter has the same name as a selection flag, the selection flag wins and the parameter is skipped. This avoids a pflag redefinition panic at startup. Decode the parameters with `clicky.BuildOpts[RetagFlags](flags)`.

## UI and AI metadata

```go
clicky.BulkAction("purge", purgeIDs).
	WithToolPermission(clicky.ToolPermissionAsk).
	WithToolHints(clicky.MCPToolHints{Icon: "trash", DestructiveHint: ptr(true)})
```

A bulk action is the same verb aimed at many rows, so it is the operation most worth an approval prompt. `Icon`, `Group` and `DestructiveHint` are also what clicky-ui uses to render the button in the selection toolbar. See [AI tools](/runtime/ai-tools/).
