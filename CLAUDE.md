# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

clicky is a **library** (module `github.com/flanksource/clicky`). This file documents how to *use*
clicky as a dependency — its public API surface and architecture. For building, testing, and
contributing to clicky itself (commands, module layout, CI, conventions), see
**[CONTRIBUTING.md](CONTRIBUTING.md)**.

## What clicky is

A Go toolkit for turning structured data and Cobra CLIs into polished terminal, HTML/PDF, web, and
AI-facing interfaces. One struct, decorated with `pretty:` tags, can be rendered to ~15 formats,
exposed as CRUD CLI commands, served as a REST/OpenAPI API, and surfaced as MCP tools — all from the
same source of truth.

```bash
go get github.com/flanksource/clicky                    # use as a library
go install github.com/flanksource/clicky/cmd/clicky@latest  # the standalone CLI
```

## Using clicky

### Formatting data

`clicky.Format(obj, opts)` renders any value to a chosen format; companions are `MustPrint`,
`MustFormat`, `FormatToFile`, and `PrintAndWriteSinks` (one render → multiple sinks via a
`"pretty,json=out.json,md=summary.md"` format spec). Supported formats: `pretty`, `json`, `yaml`,
`csv`, `markdown`, `html`, `html-react`, `html-static`, `pdf`, `slack`, `excel`, `tree` (plus
aliases like `md`, `yml`, `xlsx`). Plain structs render via reflection over their fields and
`pretty:` tags; no interface implementation is required for basic output.

### Render interfaces (`api` package)

To control a type's appearance, implement one of these — formatters prefer the most specific:

- `Pretty() api.Text` — compact single-line representation.
- `PrettyFull() api.Textable` — fuller multi-line detail view.
- `PrettyShort() api.Textable` — compact self-link, preferred in table cells.
- `PrettyRow(opts) map[string]api.Text` — custom table-row columns.
- `Textable` — the universal interface; one object emits ANSI / HTML / Markdown / plain text.

Compose output from primitives via the top-level constructors in `format.go`/`aliases.go`:
`clicky.Text`, `Table`, `Tree`, `Collapsed`, `CodeBlock`, `Badge`, `LabelBadge`, `Button`, `Link`,
`Diff`, `StackTrace`, `Admonition`, `Map`, `List`. Styling uses **Tailwind-like class strings**
(`"text-red-600 font-bold"`) resolved against a `Theme` that auto-adapts to terminal capability.

### `pretty:` struct tags

Decorate fields to drive reflection-based rendering, e.g.
`pretty:"label=CPU %,color=blue,sort"`, `pretty:"format=table"`, `pretty:"format=currency"`,
`pretty:"-"` to hide, `pretty:"short"` to render a field via its value's `PrettyShort()`. The CLI
can also format data against an external YAML schema (`PrettyObject`/`PrettyField`) instead of tags.

### Tasks (`task` package)

`clicky.StartTask(name, fn)` and `StartGroup[T]` schedule typed, concurrent work on a global
`Manager` with progress rendering. Options: `WithConcurrency`, `WithDependencies`,
`WithRetryConfig`, `WithTimeout`, context cancellation. Wait for completion with
`task.Wait()` / `clicky.WaitForGlobalCompletion()`.

The interactive renderer **owns the terminal** while tasks run, redrawing the task tree in place.
Emit logs through `clicky.Infof/Errorf/Warnf/Debugf` (which route via a gated, secret-redacting
writer) rather than `fmt.Println`, so log lines never corrupt a live frame. A `task/ui/` Preact
bundle (embedded via `//go:embed`, streamed over SSE) provides a browser view of task state.

### Entities — generate CLI + REST + web UI from a struct

Define a data type once and get all three surfaces. Implement `EntityItem` (`GetID`/`GetName`) on
your struct, then register with the fluent builder:

```go
clicky.NewEntity[T, ListOpts, R]("name").List(listFn).Get(getFn).Create(createFn).Register()
clicky.GenerateCLI(rootCmd) // emits list/get/create/update/delete + custom-action subcommands
```

`flag:"..."`/`help:"..."` tags on the `ListOpts` struct become Cobra flags. The rpc layer mirrors
the same entities to `/api/v1/{entity}/...`, and the webapp in `examples/enitity/webapp` consumes
the OpenAPI + entity metadata to auto-render explorers. `Filter` types supply typeahead/dropdown
lookups; CEL expressions drive row filtering/transforms (see `CEL_FILTERS.md`).

### CLI extensions (OpenAPI + MCP)

`extensions.CobraExtensions(rootCmd).All()` attaches OpenAPI and MCP subcommands to any Cobra root:

- **`rpc`** walks the Cobra command tree into an `RPCService` of operations (each with a JSON
  schema), generates an OpenAPI spec, and serves it with Swagger UI. An **executor** maps an HTTP
  request → CLI invocation → captured output, returning an `ExecutionResponse` envelope.
- **`mcp`** exposes the same operations as MCP tools (`mcp serve`/`mcp tools`/`AutoExpose()`), with
  include/exclude and parameter-filtering knobs.

### Writing render code (enforced by `clicky lint ./...`)

The bundled analyzer enforces these API-usage rules — follow them in any code that builds clicky
render primitives:

- Build `Text` via `clicky.Text(...)` or `api.Text{}.Append(...)` chaining — **not** `api.Text{...}`
  struct literals, and not `Children:` slice literals.
- A function returning `api.Text` must be named `Pretty`/`PrettyFull`/`PrettyRow`; otherwise return
  the `api.Textable` interface.
- **No direct writes to `os.Stdout`/`os.Stderr`** in library code — route through
  `clicky.Infof`/the log writer so the task renderer's terminal accounting stays correct.
  File-level opt-out: `//clicky:allow-stdout`.

## Architecture map

- **`api/`** — render primitives (`Text`, `Table`, `Tree`, …), the `Pretty*`/`Textable` interfaces,
  `pretty:`-tag parsing, Tailwind-class styling (`api/tailwind`), and themes (`api/themes.go`).
- **`formatters/`** — `FormatManager` (the global `clicky.Formatter`) dispatches to per-format
  formatters; `formatters/reflect.go` walks plain structs into `FieldValue`/`PrettyData`.
- **`task/`** — `Manager`, `Task`, `TypedTask`, `Group`, the live TTY renderer + pluggable
  `LiveRenderer` hook, SSE, and the embedded `task/ui/` frontend.
- **`entity*.go`** (root) — the entity registry and fluent builder.
- **`extensions/`, `rpc/`, `mcp/`** — Cobra → OpenAPI/Swagger and Cobra → MCP layers.
- **`cmd/clicky/`** — the standalone binary: `pretty`, `schema`, `lint`, `version`, plus the
  OpenAPI/MCP subcommands.
- **`exec/`, `flags/`, `text/`, `middleware/`** — process execution, struct-tag flag binding, text
  tokenization/redaction, and Echo v4 middleware presets.
