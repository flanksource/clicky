---
title: Pretty printing
description: Render any Go value to the terminal, Markdown, HTML, JSON and more from one structured value.
---

clicky separates **what** you show from **how** it is rendered. Domain code returns structured values: a struct, a slice, `api.Text`, a table or a tree. The formatter at the output boundary turns those values into ANSI, Markdown, HTML, JSON, YAML, CSV, PDF, Slack or Excel.

```go
import (
	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
)

clicky.MustPrint(orders)                                               // default (pretty) to stdout
out, err := clicky.Format(orders, clicky.FormatOptions{Format: "markdown"})
err = clicky.FormatToFile(report, clicky.FormatOptions{Format: "html"}, "report.html")
```

| Function | Use |
| --- | --- |
| `clicky.Format(v, opts...)` | render to a string |
| `clicky.MustFormat(v, opts...)` | same, panics on error |
| `clicky.MustPrint(v, opts...)` | render and write to stdout |
| `clicky.FormatToFile(v, opts, path)` | render to a file |
| `clicky.PrintAndWriteSinks(v, opts)` | one render to several sinks, e.g. `--format "pretty,json=out.json,md=summary.md"` |

## Formats

`pretty` (default), `json`, `yaml`, `csv`, `markdown` (`md`), `html`, `html-react`, `html-static`, `pdf`, `slack`, `excel` (`xlsx`), `tree`.

Bind the standard flags to your CLI once, and every command gets `--format`, `--no-color` and a CEL `--filter` for rows and nodes:

```go
clicky.BindAllFlags(root.PersistentFlags())
```

Entity commands render their results through the same formatter automatically. See [Entities](/entities/overview/).

## What gets rendered

The formatter picks the richest representation a value offers:

| Value | Rendered as |
| --- | --- |
| implements `Pretty() api.Text` | that text |
| implements `api.Textable` (`api.Text`, tables, trees, components) | itself, natively per format |
| implements `api.TreeNode` / `api.TreeMixin` | a tree |
| slice of `api.TableProvider` or of structs | a table |
| struct | key/value fields, driven by [`pretty:` tags](/pretty/structs/) |
| map | key/value list |
| scalar | humanized (durations, times, numbers, booleans) |

Structured formats (`json`, `yaml`) serialize the **data**, using `json`/`yaml` tags. `pretty:` tags and render interfaces only affect the visual formats.

## Render interfaces

| Interface | Method | Use for |
| --- | --- | --- |
| `api.Pretty` | `Pretty() api.Text` | the normal one-line or short representation |
| `api.PrettyFull` | `PrettyFull() api.Textable` | an expanded detail view |
| `api.PrettyShort` | `PrettyShort() api.Textable` | a compact label or self-link in table cells |
| `api.PrettyRow` | `PrettyRow(opts any) map[string]api.Text` | custom table-row cells |
| `api.TableProvider` | `Columns()`, `Row()` | explicit table schema, see [Tables](/pretty/tables/) |
| `api.DetailProvider` | `RowDetail() api.Textable` | expandable per-row detail |
| `api.TreeNode` | `Pretty()`, `GetChildren()` | hierarchy, see [Trees](/pretty/trees/) |
| `api.Textable` | `String()`, `ANSI()`, `HTML()`, `Markdown()` | a fully custom primitive |

Start with [Text](/pretty/text/). Almost everything else is built from it.
