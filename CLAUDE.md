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

### Interactive forms (`schemaform`, `clicky.PromptForm`, `-i`)

The terminal counterpart of clicky-ui's `JsonSchemaForm`, built on charmbracelet/huh. The same schema drives both surfaces.

- `schemaform.Run(ctx, schema, schemaform.Options{...})` renders a JSON Schema as an inline terminal form: a bubbletea model (`schemaform/inline.go`) that uses huh fields only as editors. Widget resolution, `x-*` extensions, `allOf` if/then/else, `x-discriminator`, `x-on-change`/`x-on-load` listeners (`when.expr` is CEL over `key`/`value`/`self`/`root`) and `x-clicky-lookup` (through `Options.Lookup`) all follow `JsonSchemaForm`. `Accessible: true` runs line-based prompts (screen readers, scripted tests).
- `clicky.PromptForm(ctx, PromptFormOptions{Schema, Default, Lookup})` routes like the other prompts. A scoped context with a `prompt.Manager` sink goes to the dashboard (`Manager.Form`, rendered by `JsonSchemaForm`). Otherwise it uses the terminal, or fails with "interactive terminal required". `ACCESSIBLE=1` forces line-based prompts.
- `rpc.OperationInputSchema(op, OperationSchemaOptions{...})` turns an operation's arguments into that schema, through the same per-parameter conversion as the OpenAPI document (roles, lookups, array slice flags, typed defaults). `rpc.PromptOperation(ctx, op, PromptOperationOptions{Preset, Completions})` collects values ready for `ExecuteOperation`.
- `extensions.CobraExtensions(root).Interactive()` (also part of `.All()`) adds a persistent `-i/--interactive` flag. The global flag loses `-i` only when some command uses `-i` for a differently named flag. A command that defines its own `--interactive` keeps it and is left unwired. With the flag set, a command opens its form pre-filled with the flags and args already given, then runs with the answers. Call it after the command tree is complete.
- **Inline layout.** Each field is one row, `label  value`. Only the focused row (`› label`) opens its huh editor.
  - Single-line editors (inputs, confirms) open on focus. Multi-line ones (selects, multi-selects, text areas) open on enter.
  - An open select shows the highlighted option's description (`Option.Description`, from `x-enum-descriptions` or an item's description). The row shows the chosen option's description.
  - Keys:
    - tab and shift+tab move between rows. ↑/↓ also move on any row whose editor is closed or single-line.
    - enter moves on, and on the last field it submits.
    - esc closes an open editor and reverts it, or reverts a changed field. Otherwise it arms quit, and a second esc cancels the form.
    - ctrl+s submits from anywhere, and ctrl+c cancels.
  - Leaving a field validates it and shows any error under its row without blocking. Submit focuses the first invalid field.
- **Dependent object flags** (`x-clicky-schema`). `clicky.WithActionFlagSchema(action, "vars", clicky.FlagSchema{Scope: []string{"id"}, Load: fn})` declares that a JSON-object flag's schema depends on other arguments.
  - Under `-i`, once the scope is answered, the form loads the schema through `Load` and puts its fields right after the scope field. It rebuilds them when the scope changes and passes their answers as the flag's JSON. Captain's `prompt run --vars`, for example, is the chosen prompt's input schema.
  - A load error ends the form with that error.
  - `schemaform` takes these as `Options.LoadSchema`.
- **Advanced split** (`schemaform/inline_split.go`). Always shown, in this order: the discriminator, the positional argument (`x-clicky` role `positional`, which `-i` sets) and required fields. Fields with a value (`false` and `0` count) come next, then empty ones, for as long as they fit the terminal height. The rest fold under `▸ Advanced  N options`.
  - A field that a branch or listener makes required later is promoted to the row after the field before it.
  - Accessible mode asks the same up-front fields in document order, then asks `Configure N advanced options?`.
- Choices come from the command's shell completion: the positional's `ValidArgsFunction`/`ValidArgs` and each flag's registered completion become dropdowns, re-queried with the answers so far. File and directory completions stay free text. An optional-id action (`prompt run [id]`) gets its id back as the first field.
- Entities complete ids by default: `RegisterEntity` sets `ValidArgsFunction` to the entity's own `List` (`id<TAB>name`, prefix match on either) when the command declares none.
  - The form reads the same list through `entity.GetIDOptions(cmd)`, with each item's description when it implements `EntityDescriber` (`GetDescription()`).
  - To give a map-based action the lookups of a flags struct's filters, wrap it with `clicky.WithActionFilters(action, filters)`.
- A filter whose options come from a source that can fail (a DB, a config catalog) implements `entity.FallibleFilter` (`LookupOptions(ctx, opts) (map, error)`). Lookups and shell completion prefer it and fail with its error instead of offering nothing. Shell completion now resolves options through the same path as lookups, so `ContextFilter` and searchable filters apply there too.
- Demo: `examples/interactive_form.go` (`deploy -i`, `configure`). Its pty fixture is `examples/testdata/interactive-form.fixture.yaml` (`gavel fixtures examples/testdata/interactive-form.fixture.yaml`). It waits for the focus marker (`› <label>`) before each keystroke, because focus moves through a returned command and keys typed earlier land in the previous field. A cobra positional whose `Args` rejects zero args is `required` in the operation schema.

### Writing render code (enforced by `clicky lint ./...`)

The bundled analyzer enforces these API-usage rules — follow them in any code that builds clicky
render primitives. Diagnostics are split by severity: **errors** fail the run (non-zero exit);
**warnings** are advisory and do not. JSON output and the tree summary report the severity per rule.

**Errors** (structural — bypassing clicky's generated surfaces or corrupting the renderer):

- **No manual `cobra.Command` with `Run`/`RunE`** — register the operation as an entity
  (`clicky.NewEntity(...).Register()` + `GenerateCLI`) or via `entity.AddCommand`, so it joins the
  generated CLI/REST/MCP surfaces. Bare grouping commands (no `Run`/`RunE`) are fine.
- **No direct `net/http` handler registration** (`http.HandleFunc`/`http.Handle`/
  `(*http.ServeMux).HandleFunc`/`.Handle`) — expose data via a registered entity and serve it
  through the rpc layer instead of raw handlers that collide with the auto-routed `/api/v1/*` space.
- **No direct writes to `os.Stdout`/`os.Stderr`** in library code — route through
  `clicky.Infof`/the log writer so the task renderer's terminal accounting stays correct.
  File-level opt-out: `//clicky:allow-stdout`.

**Warnings** (preferences for how `Pretty()`/render builders are written, plus entity ergonomics):

- Build `Text` via `clicky.Text(...)` or `api.Text{}.Append(...)` chaining — **not** `api.Text{...}`
  struct literals, and not `Children:` slice literals.
- A function returning `api.Text` must be named `Pretty`/`PrettyFull`/`PrettyRow`; otherwise return
  the `api.Textable` interface. Don't call `.ANSI()`/`.HTML()`/`.Markdown()`/`.String()` inside a
  render builder — return `api.Text`/`api.Textable` and let the formatter render it.
- An entity registered via `NewEntity`/`RegisterEntity` whose item type does not implement
  `api.TableProvider` (`Columns()`/`Row()`) is flagged — add it so list output can render as a table.

### Actions, filters & error rendering (from project memory)

Practical notes from building entity surfaces on clicky:

- **Typed action flags via an interface, not reflection.** To attach optional typed flags to an
  `Action[T]`, define an `ActionFlags` interface the options struct implements plus an interface-typed
  field on the action, and check it at registration with a type assertion. Avoid a `Flags reflect.Type`
  field — the marker interface keeps the feature opt-in and the registration call site declarative.
- **An action's verb must be unique per entity.** Each `EntityAction` becomes a cobra subcommand
  named after its verb, so two actions sharing a verb collide even though their routes differ. A
  required-`<id>` action routes to `/api/v1/<entity>/{id}/<verb>`; a `.WithOptionalID()` action routes
  to the flat `/api/v1/<entity>/<verb>`. Give a per-id action a verb distinct from any bulk/optional-id
  action (e.g. `execute` when `run` is already taken); pass route selectors as string flags, not bool.
- **From/To filters auto-pair into one range picker** only when both fields are `time.Time` (not
  `string`/`*time.Time`) AND the flag names are `from`/`to` or end with `-from`/`-to`
  (`entity.go` `describeLookupField`/`isRangeStartFlag`/`isRangeEndFlag`). Do NOT also register them as
  explicit `Filter` entries — explicit registration overrides the pairing and renders two text boxes.
- **Searchable (type-ahead) lookups** are opt-in via the `SearchableFilter[ListOpts]` interface —
  `OptionsWithQuery(opts, query, limit) (map[string]api.Textable, total)`. `buildLookupFunc`
  type-asserts it; the handler reads `__lookup_filter`/`__lookup_q` from the query string (limit 200).
  Empty query → head + total (sets `Truncated`/`Total`); non-empty → matched results. The query is the
  only user input reaching SQL — keep it a bound parameter.
- **Prefer a command + filter struct over a raw `mux.HandleFunc`.** Registering an op as a command
  whose `ContextDataFunc` returns structured data auto-routes it under `/api/v1/<cmd>`; drive filtering
  with `flag:`/`json:`-tagged fields + `MultiFilter`. A raw handler in the executor-owned `/api/v1/*`
  namespace collides with the auto-routed command routes.
- **Errors that implement a render interface are rendered, not just logged.** The command runner's
  `renderableError` walks the `errors.Unwrap` chain and, if any error implements a clicky render
  interface (`api.TryTypedValue`), routes it through `PrintAndWriteSinks` (honouring `--format`) while
  still returning non-zero. Return an error type with `Pretty() api.Text` (and `MarshalJSON` to skip
  the struct envelope) to get rich failure output.

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
