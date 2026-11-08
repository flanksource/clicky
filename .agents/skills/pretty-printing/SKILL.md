---
name: pretty-printing
description: Implement and review Clicky render interfaces and builders in Go, including Pretty, PrettyFull, PrettyShort, TreeNode, TableProvider, icons, code blocks, collapsed sections, and humanized values. Use when adding or correcting rich terminal, HTML, Markdown, tree, or table output in Clicky or a consuming Go repository.
---

# Pretty Printing with Clicky

Build structured output and return Clicky values instead of rendering ANSI, HTML, or Markdown inside domain code.

## Workflow

1. Inspect the Clicky version used by the target module before choosing helpers or interfaces. In this repository, treat `api/`, `format.go`, and `aliases.go` as the source of truth.
2. Choose the narrowest render interface that matches the presentation:
   - `Pretty() api.Text` for a normal representation.
   - `PrettyFull() api.Textable` for expanded detail.
   - `PrettyShort() api.Textable` for compact table-cell links or labels.
   - `api.TableProvider` for table rows.
   - `api.TreeNode` for hierarchy.
3. Prefer top-level helpers such as `clicky.Text`, `clicky.CodeBlock`, `clicky.Collapsed`, `clicky.Map`, `clicky.List`, and `clicky.Human` when they express the intent. Use `api.Text{}.Append(...)` for incremental composition.
4. Keep format selection at the output boundary. Return `api.Text` or `api.Textable`; do not call `.ANSI()`, `.HTML()`, `.Markdown()`, or `.String()` inside render builders.
5. Run `clicky lint` in consumer repositories when available, then exercise the affected output formats.

## Core patterns

```go
import (
    "github.com/flanksource/clicky"
    "github.com/flanksource/clicky/api"
    "github.com/flanksource/clicky/api/icons"
)

func (r Result) Pretty() api.Text {
    text := clicky.Text(r.Name, "font-bold")
    if r.Err != nil {
        return text.Space().Add(icons.Error).Space().Append("Failed", "error")
    }
    return text.Space().Add(icons.Success).Space().Append("Passed", "success")
}
```

Use `.Space()` for layout, and add icons as values so their style is preserved:

```go
t := api.Text{}.
    Add(icons.Success).
    Space().
    Append("Running", "success")
```

Use root helpers for rich blocks:

```go
func (r Result) PrettyFull() api.Textable {
    return api.Text{}.
        Add(r.Pretty()).
        NewLine().
        Add(clicky.Collapsed(
            "Response",
            clicky.CodeBlock("application/json", r.Body),
        ))
}
```

Use human formatting instead of hand-written duration, time, number, or byte formatting:

```go
t := api.Text{}.
    Append(clicky.Human(elapsed), "muted").
    Space().
    Add(api.HumanizeBytes(size).Styles("muted"))
```

## Tables

Implement both methods on the row type. Return Clicky values in `Row()` when a cell needs rich rendering.

```go
func (r Result) Columns() []api.ColumnDef {
    return []api.ColumnDef{
        clicky.Column("name").Label("Name").Build(),
        clicky.Column("status").Label("Status").Build(),
        clicky.Column("duration").Label("Duration").Build(),
    }
}

func (r Result) Row() map[string]any {
    return map[string]any{
        "name":     r.Name,
        "status":   r.StatusText(),
        "duration": clicky.Human(r.Duration, "muted"),
    }
}

table := api.NewTableFrom(results)
```

Add `RowDetail() api.Textable` only when rows need expandable detail. Return `nil` when no detail exists.

## Trees

Render only the current node in `Pretty()` and expose hierarchy through `GetChildren()`:

```go
func (n Node) Pretty() api.Text {
    return clicky.Text(n.Name)
}

func (n Node) GetChildren() []api.TreeNode {
    children := make([]api.TreeNode, 0, len(n.Children))
    for _, child := range n.Children {
        children = append(children, child)
    }
    return children
}
```

Return `nil` from `GetChildren()` for leaves. Do not recursively render children inside `Pretty()`.

## Guardrails

- Prefer Go structs with `pretty` tags and the owning type's `Pretty`/`TableProvider`/`TreeNode` interfaces over manually rebuilding field, row, or child mappings at call sites. Compose nodes inside those implementations when their presentation requires it.
- Avoid repeated or nested `fmt.Sprintf` for structured presentation. Keep typed and rich values intact; use tags, builders, and human-formatting helpers. Concise prose interpolation and required domain formats remain valid.
- Do not hardcode ANSI escapes or HTML.
- Do not convert icons to strings before adding them.
- When `clicky lint` applies, name concrete `api.Text` returners as recognized `Pretty*` methods; use `api.Textable` for other render helpers. This is a repository lint contract, not a Go type-system restriction.
- Do not use direct `api.Text{Content: ...}` or child-slice literals when a helper or append chain exists.
- Do not assume terminal-only behavior; verify Markdown or HTML when the changed primitive has format-specific behavior.
- Preserve stable table column keys and unique tree identity separately from human-readable labels.

## Gavel lint and review prompt

The workflow pins the Gavel action to v0.0.67, which does not pin the CLI: the action uses a preinstalled binary or downloads the latest release by default. Check the installed CLI with `gavel version` and `gavel lint --help`. Gavel CLI v0.0.67 accepts linter names or file paths after `gavel lint` and does not provide `gavel lint prompt`. Run supported lint commands against the requested files:

```bash
gavel lint lint/summary.go
gavel lint --changed
gavel lint --dry-run lint/summary.go
```

Run from the reviewed repository. Pass files or directories explicitly for a full review. `--changed` and `--since` report only new issues relative to a baseline and can omit existing issues in the requested files. For a separate agent review, use [prompts/pretty-printing.prompt](prompts/pretty-printing.prompt), located at `.agents/skills/pretty-printing/prompts/pretty-printing.prompt` in this checkout. Consumers should use the absolute path to that prompt in their Clicky checkout, not the path to `SKILL.md`. A Gavel version that provides `gavel lint prompt` can run the prompt directly; verify support with `gavel lint --help` first.

## Detailed reference

Read [references/api-patterns.md](references/api-patterns.md) when choosing an interface, builder, icon, list, key-value layout, styling class, or validation strategy.
