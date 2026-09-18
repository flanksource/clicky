---
title: Tables
description: Render slices as tables by reflection, or declare columns explicitly with TableProvider.
---

There are three ways to get a table. Pick the lightest one that does the job.

## 1. A slice of structs

Any slice of structs renders as a table. The columns are the fields (after `pretty:` tags are applied) and the headers are the labels.

```go
clicky.MustPrint([]Order{o1, o2})
```

```text
(the nested Lines column is trimmed here)
╭─────┬─────────┬─────┬───────────────────╮
│Order│Status   │Total│created            │
├─────┼─────────┼─────┼───────────────────┤
│ORD-1│completed│42.50│2026-09-01 10:00:00│
│ORD-2│failed   │42.50│2026-09-01 10:00:00│
╰─────┴─────────┴─────┴───────────────────╯
```

`label=`, `hide`, `style=` and a field type's `Pretty()` all apply per column. See [Structs & pretty tags](/pretty/structs/).

## 2. TableProvider: explicit columns

Implement `api.TableProvider` when you need a stable column order, computed columns or rich cells:

```go
func (c Check) Columns() []api.ColumnDef {
	return []api.ColumnDef{
		clicky.Column("name").Label("Check").Build(),
		clicky.Column("status").Label("Status").Build(),
		clicky.Column("duration").Label("Took").Build(),
	}
}

func (c Check) Row() map[string]any {
	status := clicky.Text("pass", "text-green-600")
	if !c.Passed {
		status = clicky.Text("fail", "text-red-600 font-bold")
	}
	return map[string]any{
		"name":     c.Name,
		"status":   status,
		"duration": clicky.Human(c.Duration, "text-gray-500"),
	}
}

clicky.MustPrint(api.NewTableFrom(checks))
```

```text
╭─────┬──────┬──────╮
│Check│Status│Took  │
├─────┼──────┼──────┤
│lint │pass  │1200ms│
│test │fail  │42.00s│
╰─────┴──────┴──────╯
```

```markdown
| CHECK |  STATUS  |  TOOK  |
|:-----:|:--------:|:------:|
| lint  |   pass   | 1200ms |
| test  | **fail** | 42.00s |
```

- `Columns()` defines the order. The keys returned by `Row()` must match the column names.
- Return rich values (`api.Text`, links, badges, `clicky.Human(...)`) from `Row()`, not pre-rendered strings.
- `api.NewTableFrom(items)` reads the columns from the first item. With an empty slice it calls `Columns()` on the zero value and renders the headers only, so `Columns()` must work on a zero receiver.
- An [entity](/entities/overview/) row type that implements `TableProvider` is used for list output. `clicky lint` warns when it does not.

### Column builder

`clicky.Column(name)` returns a builder:

| Method | Effect |
| --- | --- |
| `.Label("Took")` | header text (defaults to a prettified name) |
| `.Style("font-mono")`, `.HeaderStyle(…)` | cell / header classes |
| `.MaxWidth(40)` | truncate cells to N characters |
| `.Type("date")`, `.Format("currency")` | value coercion / formatting hints |
| `.Unit("bytes")` | numeric display unit (`percent`, `percentunit`, `bytes`, `decbytes`, `Bps`, `ms`, `s`, …); takes precedence over `Format` |
| `.Kind("status")` | semantic UI column kind (timestamp, tags, status) |
| `.SortKey("updated")` | public server-side [sort key](/entities/sorting-and-paging/) |
| `.FilterKey("status")` | bind the column to a server-side filter parameter |
| `.MinWidthPixels(n)`, `.MaxWidthPixels(n)` | browser column widths |
| `.Hidden()` | keep the column in data but hide it |
| `.FormatOption(k, v)` | a formatter-specific option |
| `.Build()` | the `api.ColumnDef` |

### Filterable raw values

When presentation changes a value's shape, for example a styled `42.00s` whose filter value should stay in milliseconds, return an `api.TableCell` from `Row()`:

```go
"duration": api.TableCell{Value: clicky.Human(d), FilterValue: d.Milliseconds()},
```

### Expandable row detail

Implement `api.DetailProvider` to attach expandable content to each row. It shows as a disclosure in HTML and below the row elsewhere:

```go
func (c Check) RowDetail() api.Textable {
	if c.Output == "" {
		return nil // no detail for this row
	}
	return clicky.CodeBlock("text", c.Output)
}
```

Return the detail body itself. Don't wrap it in another `Collapsed`.

## 3. PrettyRow: per-format cells

`PrettyRow(opts any) map[string]api.Text` lets a row choose its cells based on the output options. Use it only when `TableProvider` isn't enough.

## Compact cells: PrettyShort

A value that implements `PrettyShort() api.Textable` is shown in its short form inside table cells, typically a self-link, while `Pretty()` stays the full form elsewhere. A `pretty:"short"` tag forces this for a struct field.

```go
func (o Order) PrettyShort() api.Textable {
	return clicky.Link("/orders/" + o.ID).Append(o.ID)
}
```

## Filtering rows

Visual formats honour the `--filter` CEL expression from `clicky.BindAllFlags` (`FormatOptions.Filter`) for table rows:

```bash
app orders --filter "status == 'failed' && total > 40"
```

## Empty columns

`table.WithoutEmptyColumns()` drops columns in which every cell is empty.
