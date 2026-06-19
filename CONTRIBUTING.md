# Contributing to clicky

This guide covers building, testing, and developing **clicky itself**. For using clicky as a
library in your own application, see [CLAUDE.md](CLAUDE.md) and [README.md](README.md).

## Workspace layout

- **Go 1.26+.**
- `go.work` stitches this repo together with **local sibling checkouts** of
  `flanksource/arch-unit`, `flanksource/commons`, and `flanksource/gavel`. Clone those next to
  clicky so the workspace resolves — edits to commons are then picked up directly, without a
  `go get`/version bump.
- The tree contains **6 separate Go modules**, each with its own `go.mod`:
  `.` (root), `valkey/`, `aichat/`, `examples/`, `examples/uber_demo/`, `examples/enitity/`.
  The `examples/*` modules are `//go:build ignore` demos that are tidied and tested separately —
  keep their `go.mod` tidy or CI's Test job fails.

## Commands

```bash
make build          # build ./clicky from ./cmd/clicky (NEVER run `go build` directly)
make test           # go test -v ./...   (ROOT module only — example modules are separate)
make lint           # golangci-lint v2.8.0 (pinned into .bin/) + go vet ./...
make fmt            # gofmt -s -w . and `go mod tidy` across every module in GO_MODULES
make check          # fmt + lint + test
make task-ui        # rebuild the embedded Preact task-UI bundle (task/ui/dist/taskui.js)
make docs           # serve pkgsite Go docs locally on :8089
```

Run a single test (root module):
```bash
go test ./formatters/ -run TestReflect            # plain go test
ginkgo run --focus "detects bad api.Text" ./lint   # Ginkgo suites (preferred)
```

Integration tests are behind the `integration` build tag:
```bash
make test:openapi   # builds binary, starts server, runs ./rpc TestOpenAPIServe_E2E
go test -tags integration ./rpc/... -run TestOpenAPIServe_E2E
```

CI (`.github/workflows/gavel.yml`) runs tests and lint through the `flanksource/gavel` action, not
bare `go test`/`golangci-lint`. The `flanksource/dist.yml` workflow rebuilds and commits the
embedded `task/ui/dist/taskui.js` bundle on pushes to main.

## Conventions

- **Commits** use Conventional Commits with **sentence-case subjects**, lower-case scopes, no
  trailing period, and a ≤100-char header (enforced by `.commitlintrc.json`).
  E.g. `feat(exec): detect compiler activity during process startup`.
- **Tests** use **Ginkgo/Gomega** (`*_suite_test.go` register the suites). Colocate unit tests next
  to source; keep DB/integration tests behind the `integration` build tag.
- **Custom API linter** — `lint/` is a `go/analysis` analyzer (run as `clicky lint ./...`) that
  enforces clicky's render-code rules against the `github.com/flanksource/clicky` module. The rules
  are documented under "Writing render code" in [CLAUDE.md](CLAUDE.md); follow them or CI fails.
- **Generated artifacts** at the repo root (`out.*`, `*.pdf`, `*.html`, `react.html`, `*-demo`
  binaries) are gitignored scratch output — not source. `task/ui/dist/taskui.js` is committed build
  output auto-rebuilt by CI; regenerate locally with `make task-ui`.

## Known issues (from project memory)

- `gavel-action-lint-show-passed` — the gavel GitHub action can crash `gavel lint`; run lint as a
  raw step when that happens.
- `openapi-ansi-leak-bug` — `rpc/serve.go` must serve the `ExecutionResponse` envelope for
  json/yaml and not conflate render format with the HTTP wire format.
- `datafunc-wire-envelope` — the envelope substitution above is correct only for stdout-capture
  commands (payload in `metadata.Output`). Entity list/get go through `op.DataFunc` whose payload is in
  `data`, so gate the substitution on `!ExecutionResponse.DataIsStructured` (set true in the executor's
  DataFunc branch) — otherwise every `GET /api/v1/<entity>` returns an empty `{success,exit_code,cli}`
  envelope with no rows. Regression: `rpc/datafunc_wire_test.go`.
- `valkey-nested-module-pin` — `valkey/` is a **separate Go module** (own `go.mod`) tagged
  `valkey/vX.Y.Z` matching the parent version; the parent proxy zip intentionally contains no
  `valkey/`. Consumers require `clicky` and `clicky/valkey` as two lines; a `valkey` pseudo-version
  pointing at a commit predating an API breaks `GOWORK=off` (Docker/CI) builds with
  `undefined: clickyvalkey.X` even though a local `go.work` checkout compiles. Cut a real `valkey/vX`
  tag at the commit that has the API, then `GOWORK=off go get github.com/flanksource/clicky/valkey@vX`.
- `tree-multiline-label-gutter` — lipgloss prefixes *every* physical line of a multi-line `TreeNode`
  label with the `│   ` gutter, so blank/ANSI-only separator lines render as empty `│`-gutter rows.
  Normalize labels in `api/meta.go normalizeTreeLabel` (drop blank lines, ANSI-aware strip) — not in
  `formatters/tree_formatter.go`, which is a separate renderer.
