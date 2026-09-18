---
title: Entity model
description: The Entity type, the fluent builder, and what each field generates.
---

An entity is a typed CRUD resource. There are two equivalent ways to register one.

**Fluent builder.** This is the usual way:

```go
clicky.NewEntity[Stack, StackListOpts, StackDetail]("stack").
	Aliases("stacks", "svc").
	Filters(teamFilter{}, statusFilter{}).
	List(store.ListStacks).
	GetWithFlags(InspectFlags{}, store.GetStack).
	Create(store.CreateStack).
	Update(store.UpdateStack).
	Delete(store.DeleteStack).
	WithAction(clicky.ActionWithFlags("restart", RestartFlags{}, store.Restart)).
	WithBulkAction(clicky.BulkActionWithFilter("pause", store.Pause, store.PauseByFilter)).
	ValidArgs(store.CompleteStackIDs).
	Register()
```

**Struct literal.** This is useful when the definition is built as data, or when you reuse it as an `Admin` view:

```go
clicky.RegisterEntity(clicky.Entity[Stack, StackListOpts, StackDetail]{
	Name:    "stack",
	Filters: []clicky.Filter[StackListOpts]{teamFilter{}, statusFilter{}},
	List:    store.ListStacks,
	Get:     store.GetStack,
})
```

`Register()` and `RegisterEntity` only record the entity. Commands and routes are created when `clicky.GenerateCLI(root)` runs, so register everything first, typically from `init()`.

## Type parameters

```go
Entity[T EntityItem, ListOpts any, R any]
```

- **`T`**: the list row. It must implement `EntityItem` (`GetID() string`, `GetName() string`). List responses wrap each row so the JSON gains an `_id` field.
- **`ListOpts`**: a struct with `flag:` tags. It is decoded from the flag map for list calls, filter-mode bulk actions and lookups.
- **`R`**: the single-item result of `Get`, `Create` and `Update`. Use a detail type when the detail view has more fields than a table row.

If `Name` is empty, the name is the kebab-cased Go type name of `T`.

## Fields and builder methods

| Builder method | `Entity` field | Generates |
| --- | --- | --- |
| `List`, `ListWithContext` | `List`, `ListWithContext` | `list` (also bound to the bare entity command) |
| `ListPaged`, `ListPagedWithContext` | same | paged `list` returning `PagedResult[T]` |
| `Get`, `GetWithContext` | same | `get <id>` (alias `inspect`) |
| `GetWithFlags`, `GetWithFlagsAndContext` | `GetFlags` + handler | `get <id> [flags]` with typed flags |
| `Create`, `CreateWithContext` | same | `create [key=value ...]` |
| `Update`, `UpdateWithContext` | same | `update <id> [key=value ...]` |
| `Delete`, `DeleteWithContext` | same | `delete <id>` |
| `Filters(...)` | `Filters` | completions, lookup metadata, value normalization |
| `Sort(SortSpec)` | `Sort` | validated `--sort` / `--order` |
| `WithPrimaryAction` | `PrimaryAction` | the bare entity command runs an action instead of `list` |
| `WithAction` | `Actions` | `<verb> <id>` subcommands |
| `WithBulkAction` | `BulkActions` | `<verb> <id> [id...]` with optional filter mode |
| `Admin(Entity{...})` | `Admin` | `admin <entity> ...` subtree |
| `Parent`, `Path`, `Aliases` | same | command nesting and UI hierarchy |
| `ValidArgs` | `ValidArgs` | shell completion for `<id>` |
| `ToolGroup`, `ToolPermission`, `ToolHints` | `ToolGroup`, `ToolHints` | MCP / AI tool metadata |

Every handler is optional. An entity with only `List` is a read-only table.

## Precedence between handler variants

When more than one variant of the same verb is set, only one is used:

- **list**: `ListPagedWithContext` > `ListPaged` > `ListWithContext` > `List`
- **get**: `GetWithFlagsAndContext` > `GetWithFlags` > `GetWithContext` > `Get`
- **create / update / delete**: `…WithContext` > plain

Set exactly one variant per verb. Precedence exists to make migration safe, not to provide fallbacks.

## Row rendering

List output goes through clicky's normal formatters. You control how rows look with `pretty:` struct tags, or by implementing `api.TableProvider` (`Columns()` / `Row()`) or `PrettyRow` on `T`. The `clicky lint` analyzer warns when an entity's row type does not implement `api.TableProvider`.

## Registry access

```go
for _, info := range clicky.GetEntities() { /* EntityInfo */ }
info, ok := clicky.GetEntity("stacks") // name or alias
```

`EntityInfo` is the type-erased record the CLI, RPC and docs generators read.
