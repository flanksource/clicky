---
title: API index
description: Where each entity-related symbol lives and which page covers it.
---

`clicky.*` is re-exported from the `entity` package. `entity.*` is available only from `github.com/flanksource/clicky/entity`.

## Registration

| Symbol | Page |
| --- | --- |
| `clicky.NewEntity`, `clicky.RegisterEntity`, `clicky.Entity`, `clicky.EntityBuilder` | [Entity model](/entities/overview/) |
| `clicky.EntityItem`, `clicky.GetEntities`, `clicky.GetEntity` | [Entity model](/entities/overview/) |
| `clicky.GenerateCLI` | [Getting started](/getting-started/) |
| `clicky.RegisterSubCommand`, `clicky.RegisterSubCommandFn` | [Custom commands](/entities/commands/) |
| `clicky.AddCommand`, `clicky.AddNamedCommand`, `…WithContext` | [Custom commands](/entities/commands/) |
| `clicky.BuildOpts` | [List options](/entities/list-options/#decoding) |

## Operations

| Symbol | Page |
| --- | --- |
| `clicky.Action`, `ActionWithFlags`, `ActionWithContext`, `ActionWithFlagsAndContext`, `TypedActionWithContext` | [Actions](/entities/actions/) |
| `entity.PrimaryActionWithContext` | [Actions](/entities/actions/#primary-action) |
| `clicky.BulkAction`, `BulkFilterAction`, `BulkActionWithFilter`, `BulkActionWithFilterAndContext` | [Bulk actions](/entities/bulk-actions/) |
| `clicky.ActionFlags` | [CRUD](/entities/crud/#get-with-typed-flags) |
| `clicky.SortSpec`, `SortOptions`, `SortCarrier`, `SortDirectionAsc/Desc` | [Sorting & paging](/entities/sorting-and-paging/) |
| `clicky.PagedResult`, `clicky.NewPagedResult`, `clicky.PageInfo` | [Sorting & paging](/entities/sorting-and-paging/#paging) |

## Filters

| Symbol | Page |
| --- | --- |
| `clicky.Filter`, `TypedFilter`, `SearchableFilter`, `ContextFilter`, `ContextSearchableFilter`, `Filterable`, `LiftFilters`, `MultiFilter` | [Filters](/filters/overview/) |
| `entity.UnitFilter`, `DefaultOperatorFilter`, `LimitedFilter`, `MaxLookupOptions` | [Control types](/filters/control-types/), [Searchable](/filters/searchable/) |
| `entity.RegisterFilter`, `RegisterFilterSpec`, `FilterFromSpec`, `GetFilter`, `MustGetFilter`, `Use` | [Named filters](/filters/named-filters/) |
| `entity.NamedFilter`, `FilterSpec`, `FilterSource`, `CountedFilterSource`, `FilterContext`, `FilterOptions` | [Named filters](/filters/named-filters/) |
| `entity.StaticOptions`, `EntityOptions`, `CountedOptions`, `FuncOptions` | [Named filters](/filters/named-filters/#sources) |

## Runtime

| Symbol | Page |
| --- | --- |
| `entity.OperationSurfaceFromContext`, `rpc.RequestFromContext`, `entity.TraceIDFromContext` | [Request context](/runtime/context/) |
| `entity.ActionIDSetter`, `entity.ActionContextSetter` | [Request context](/runtime/context/#actions-that-need-the-target-id) |
| `entity.StatusError`, `NewStatusError`, `NewStatusErrorf`, `WriteError`, `ErrorResponse` | [Errors](/runtime/errors/) |
| `entity.RegisterOperationListener`, `OperationEvent`, `clicky.CommandIdentity` | [Operation listeners](/runtime/operation-listeners/) |
| `clicky.MCPToolHints`, `ToolPermission*`, `AnnotateTool`, `MarkLocalOnly` | [AI tools](/runtime/ai-tools/) |

## Dynamic

| Symbol | Page |
| --- | --- |
| `entity.NewDynamicEntity`, `clicky.RegisterDynamicEntity`, `clicky.DynamicEntitySpec`, `clicky.DynamicFilter` | [Schema-driven entities](/dynamic/schema-entities/) |
| `entity.RegisterDynamicEntityFamily`, `UnregisterDynamicEntityFamily`, `UnknownDynamicEntity`, `ResolveDynamicLookup` | [Entity families](/dynamic/families/) |
| `entity.SplitPath`, `entity.JoinPath` | [Admin views & hierarchy](/entities/hierarchy/#path-ui-hierarchy) |

For the complete Go API, run `make docs` in the clicky repository (pkgsite), or see pkg.go.dev.
