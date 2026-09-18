---
title: Struct tags
description: Tags read from ListOpts, action flags and row types.
---

## Option structs (`ListOpts`, action flags, command options)

| Tag | Example | Meaning |
| --- | --- | --- |
| `flag` | `flag:"team"` · `flag:"limit,l"` | flag / query parameter name, optional shorthand |
| `short` | `short:"t"` | shorthand (wins over the comma form) |
| `help` | `help:"Owning team"` | help text and OpenAPI description |
| `default` | `default:"now-7d"` | default value |
| `required` | `required:"true"` | required flag |
| `hidden` | `hidden:"true"` | hidden from help |
| `enum` | `enum:"asc,desc"` | allowed values |
| `args` | `args:"true"` | bind positional arguments |
| `stdin` | `stdin:"true"` | receives piped stdin (one field per struct) |
| `clicky` | `clicky:"cli-file-read"` · `clicky:"rpc-file-read"` | opt into `@file` / `@url` expansion |

Supported field types: `string`, `int`, `bool`, `[]string`, `[]int`, `clicky.MultiFilter`, `time.Time` (with date-math) and `duration.Duration`. Embedded structs are walked. Named struct fields are not.

Marker methods on option structs:

| Method | Interface | Used by |
| --- | --- | --- |
| `ClickyActionFlags()` | `clicky.ActionFlags` | `GetWithFlags`, actions, bulk `WithFlags`, primary action |
| `Filters() []clicky.Filter[T]` | `clicky.Filterable[T]` | typed actions, `AddNamedCommand` |
| `SetClickyActionID(id)` | `entity.ActionIDSetter` | typed actions |
| `SetClickyActionContext(ctx, id)` | `entity.ActionContextSetter` | typed actions |
| `SetSort(SortOptions)` | `clicky.SortCarrier` (embed `clicky.SortOptions`) | sortable lists |
| `GetName() string` | `clicky.Name` | custom command name |
| `Help() api.Textable` | `clicky.Help` | custom command long help |

## Row types (`T`, `R`)

| Tag | Example | Meaning |
| --- | --- | --- |
| `json` | `json:"name"` | wire name |
| `pretty` | `pretty:"label=Updated,format=date"` | rendering: label, color, format, `short`, `-` to hide |
| `sort` | `sort:"updated"` | public sort key for [server-side sorting](/entities/sorting-and-paging/) |

Row types must implement `clicky.EntityItem` (`GetID()`, `GetName()`). They can also implement `api.TableProvider`, `PrettyRow` or `PrettyShort` to control table and cell rendering.
