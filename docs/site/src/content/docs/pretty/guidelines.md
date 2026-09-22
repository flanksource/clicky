---
title: Guidelines & lint
description: Rules for render code, enforced by clicky lint.
---

Render code should **describe** output, never **produce** it. Keep format selection at the boundary (`clicky.Format` / `MustPrint` / the entity runner) and return structured values everywhere else.

| Avoid | Use instead |
| --- | --- |
| Calling `.ANSI()`, `.HTML()`, `.Markdown()` or `.String()` inside `Pretty()` | return `api.Text` / `api.Textable` |
| `fmt.Sprintf` column alignment or hand-drawn tables | a slice of structs, `TableProvider` or `PrettyRow` |
| Writing to stdout/stderr from `Pretty()`, `Row()` or tree nodes | return values; log with `clicky.Infof`/`Warnf` |
| Raw ANSI escapes or HTML in strings | Tailwind classes such as `text-green-600` and `font-bold` |
| Flattening links, badges, diffs and code to strings | `clicky.Link`, `clicky.Badge`, `clicky.Diff`, `clicky.CodeBlock` |
| Stringifying icons | `.Add(icons.Success)` |
| Rendering children inside `TreeNode.Pretty()` | `GetChildren()` |

## clicky lint

```bash
go install github.com/flanksource/clicky/cmd/clicky@latest
clicky lint ./...
```

Add `--source` to show the offending source beneath each displayed location. `--source-lines` is the total excerpt length, so `--source-lines=1` shows only the offending line:

```bash
clicky lint --source --source-lines=1 ./...
```

The analyzer reports **warnings** for render code:

- `api.Text{...}` struct literals and `Children:` slice literals. Use `clicky.Text(...)` or `api.Text{}.Append(...)` instead.
- Literals of helper-backed types. Use the constructor instead: `clicky.Table`, `clicky.Tree`, `clicky.CodeBlock`, `clicky.Map`, `clicky.KeyValue`, `clicky.Collapsed`, `clicky.Admonition` and so on.
- A function that returns `api.Text` but is not named `Pretty`, `PrettyFull` or `PrettyRow`. Return `api.Textable` from other helpers.
- Calling `.ANSI()`/`.HTML()`/`.Markdown()`/`.String()` inside a render builder.
- An entity whose row type does not implement `api.TableProvider`.

It reports **errors** for code that bypasses clicky's surfaces or corrupts the live renderer: direct writes to `os.Stdout`/`os.Stderr` in library code (opt out per file with `//clicky:allow-stdout`), hand-built `cobra.Command`s with `Run`/`RunE`, and raw `net/http` handler registration.

The analyzer skips the clicky module itself. Run it from a consuming module.

## Verifying output

Exercise the exact format you changed, plus one structurally different format:

```bash
app orders --format pretty
app orders --format markdown
app orders --format html > /tmp/orders.html
```

Terminal colours and HTML classes can differ, so check both whenever a style matters.
