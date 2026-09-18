---
title: Structs & pretty tags
description: Render plain Go structs by reflection and control labels, formats and layout with pretty tags.
---

A plain struct needs no interfaces. clicky reflects over its exported fields and renders them as key/value pairs. A `pretty:` struct tag on a field adjusts how that field is shown.

```go
type Line struct {
	SKU   string  `json:"sku"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price" pretty:"format=currency"`
}

type Order struct {
	ID      string    `json:"id"      pretty:"label=Order,style=text-blue-600 font-bold"`
	Status  string    `json:"status"  pretty:"label=Status"`
	Total   float64   `json:"total"   pretty:"label=Total"`
	Created time.Time `json:"created" pretty:"format=date"`
	Lines   []Line    `json:"lines"   pretty:"table,title=Line Items"`
	Secret  string    `json:"secret"  pretty:"hide"`
}

clicky.MustPrint(order)
```

Terminal (default `clicky.Format(order)`):

```text
Status: completed
Total: 42.50
Created: 2026-09-01 10:00:00
Lines:
╭───┬───┬──────╮
│sku│qty│price │
├───┼───┼──────┤
│A-1│2  │$10.00│
│B-2│1  │$22.50│
╰───┴───┴──────╯
Id: ORD-1
```

Markdown (`Format: "markdown"`):

```markdown
Order: ORD-1
Status: completed
Total: 42.50
Created: 2026-09-01 10:00:00
Lines:
| SKU | QTY |   PRICE    |
|:---:|:---:|:----------:|
| A-1 |  2  | **$10.00** |
| B-2 |  1  | **$22.50** |
```

JSON uses the `json` tags and ignores `pretty:` entirely, so `Secret` still appears in JSON. `hide` only removes a field from the visual formats. Use `json:"-"` to keep a value out of the data.

A **slice** of structs renders as a table whose columns are the fields. A struct with a slice field renders the slice as a nested table.

## Tag reference

A `pretty:` tag is a comma-separated list of flags and `key=value` pairs.

| Tag | Effect |
| --- | --- |
| `label=Order` | display label (key in key/value output, header in tables) |
| `style=text-blue-600 font-bold` | value style ([classes](/pretty/text/#styles)) |
| `label_style=text-muted` | label style |
| `format=currency` | value formatter: `currency`, `date`, `float`, `integer`, `markdown`, `table`, `tree`, `list` |
| `digits=2` | decimals for numeric formats |
| `hide` | hide the field |
| `table` | render a slice field as a table |
| `title=Line Items` | table title |
| `sort=total`, `dir=desc` (or `asc`/`desc` flags) | sort a table by a field |
| `header_style=…`, `row_style=…` | table header / row styles |
| `tree` | render the field as a tree |
| `max_depth=3`, `indent=2`, `no_icons`, `ascii` | tree options |
| `struct` | force key/value rendering of a nested struct |
| `compact` | render slice items inline |
| `short` | render the value via its `PrettyShort()` (e.g. a self-link in a table cell) |
| `render=name` | use a render function registered in `api.RenderFuncRegistry` |

Commas separate tag entries, so a single value cannot contain a comma.

## When to implement an interface instead

Tags cover labels and layout. When a value's appearance depends on its data, for example a status icon or a colour based on thresholds, implement `Pretty()` on the field's type or on the struct:

```go
type Status string

func (s Status) Pretty() api.Text {
	switch s {
	case "completed":
		return clicky.Text(string(s), "text-green-600")
	case "failed":
		return clicky.Text(string(s), "text-red-600 font-bold")
	}
	return clicky.Text(string(s), "text-amber-600")
}

type Order struct {
	Status Status `json:"status" pretty:"label=Status"`
}
```

A field whose type implements `Pretty` (or `Textable`) is rendered through it, both in key/value output and in table cells (`[]Order` renders `failed` in bold red in the Status column).

## Schemas instead of tags

When the data is not a Go type, for example JSON or YAML from another tool, describe the fields in a YAML schema (`api.PrettyObject` / `api.PrettyField`) and format with the `clicky` CLI:

```bash
clicky pretty --schema order-schema.yaml order.json
```

The schema uses the same vocabulary as the tags (`label`, `format`, `style`, table and tree options).
