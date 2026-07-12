# Clicky Pretty-Printing API Patterns

## Contents

- [Interfaces](#interfaces)
- [Text composition](#text-composition)
- [Icons and status](#icons-and-status)
- [Code and collapsed content](#code-and-collapsed-content)
- [Key-value and list content](#key-value-and-list-content)
- [Tables and row detail](#tables-and-row-detail)
- [Human formatting](#human-formatting)
- [Validation](#validation)

## Interfaces

```go
type Pretty interface {
    Pretty() api.Text
}

type PrettyFull interface {
    PrettyFull() api.Textable
}

type PrettyShort interface {
    PrettyShort() api.Textable
}

type TreeNode interface {
    Pretty() api.Text
    GetChildren() []api.TreeNode
}

type TableProvider interface {
    Columns() []api.ColumnDef
    Row() map[string]any
}

type DetailProvider interface {
    RowDetail() api.Textable
}
```

Use `PrettyShort` for compact self-links or labels in table cells. Use `PrettyFull` when normal output should stay concise but a detail surface needs more content.

## Text composition

Prefer these constructors and methods:

| Intent | API |
|---|---|
| Start with content | `clicky.Text(content, styles...)` |
| Incremental empty builder | `api.Text{}` followed by `.Append(...)` |
| Append any supported value | `.Append(value, styles...)` |
| Append a `Textable` | `.Add(value)` |
| Append a string | `.AddText(value, styles...)` |
| Safe spacing | `.Space()` |
| Line break | `.NewLine()` |
| Style a node | `.Styles(classes...)` |
| Add a prefix or suffix | `.Prefix(value)` / `.Suffix(value)` |
| Wrap content | `.Wrap(prefix, suffix)` |

`Append` accepts strings, time values, durations, maps, and `Textable` values. Extract status/style selection into a helper when it would otherwise obscure the render chain.

## Icons and status

Import `github.com/flanksource/clicky/api/icons`. Add the icon value directly:

```go
return api.Text{}.
    Add(icons.Warning).
    Space().
    Append("Degraded", "warning")
```

Common status icons include `Success`, `Error`, `Warning`, `Info`, `Pending`, `Unknown`, and `Skip`. Use `icons.Filename(path)` when the icon should follow a filename or extension. Confirm less common icon names in `api/icons/` rather than guessing.

## Code and collapsed content

Use a language name or MIME type supported by `api.CodeBlock`:

```go
code := clicky.CodeBlock("application/yaml", manifest)
details := clicky.Collapsed("Manifest", code)
```

The formatter owns syntax highlighting and output-specific behavior. Do not manually add terminal color or HTML markup.

## Key-value and list content

Use `clicky.KeyValue` when empty values should be skipped and `clicky.Map` for deterministic map presentation:

```go
items := []api.KeyValuePair{
    clicky.KeyValue("namespace", obj.Namespace),
    clicky.KeyValue("owner", obj.Owner),
}

details := api.DescriptionList{Items: items, Style: "badge"}
labels := clicky.Map(map[string]string{"env": "prod", "team": "platform"})
```

Use `clicky.List` for explicit `Textable` items and `clicky.CompactList` for short scalar slices that should become vertical only when necessary.

## Tables and row detail

Define column order in `Columns()`. Keep `Row()` keys aligned with column names, and return rich values rather than pre-rendered strings. Use `MaxWidth` and style configuration on the column builder instead of truncating cell content manually.

`api.NewTableFrom(items)` uses the first item for column metadata and can build an empty table from the zero value. Make `Columns()` safe on a zero-value receiver.

If the row implements `DetailProvider`, Clicky collects its detail into expandable row content. Return `nil` when a row has no detail.
Return the detail body itself rather than nesting another `Collapsed` unless the product explicitly needs a second disclosure level.

## Human formatting

| Value | API |
|---|---|
| Duration, time, bool, numeric scalar | `clicky.Human(value, styles...)` |
| Bytes | `api.HumanizeBytes(value).Styles(...)` |
| Large integer | `api.HumanNumber(value, styles...)` |
| Formatted date | `api.HumanDate(value, format)` |

Pass the original typed value whenever possible so each output formatter can preserve meaning.
`clicky.Human(time.Duration(0))` currently renders empty. Omit zero durations or render an explicit `0s` according to the product contract.

## Validation

1. Run focused tests for the package containing the render implementation.
2. Run `clicky lint ./...` from an external consumer module when validating lint-enforced patterns; the analyzer intentionally skips the Clicky module itself.
3. Exercise the exact requested output plus one structurally different format, such as ANSI and Markdown or HTML.
4. Treat broad-suite environment failures separately when focused Clicky API, lint, and formatter checks pass.

Repository source locations:

- `api/types.go` — `Pretty`, `PrettyFull`, `PrettyShort`, and `PrettyRow`.
- `api/text.go` — text builders, key-value helpers, code blocks, and human formatting.
- `api/column.go` — table and row-detail interfaces.
- `api/meta.go` — tree interfaces.
- `format.go` and `aliases.go` — preferred root helper exports.
- `api/icons/` — current icon catalog.
