# Entity Example

This example packages the current clicky entity surface into one runnable app under `examples/enitity/`.

It covers:
- CRUD generation for entities
- `GetWithFlags` and action flag structs
- entity filters with lookup metadata for the filter bar
- single-item actions and bulk actions
- admin entities
- nested entity parents
- deferred subcommands via `RegisterSubCommand` and `RegisterSubCommandFn`
- executor-backed serve mode through `openapi serve --enable-executor`
- real `clicky.Link` and `clicky.LinkCommand` examples for `Dialog`, `Hover`, `Expand`, `_clicky`, `_self`, `_window`, and `_tab`

## Run

```bash
cd examples
go run ./enitity stack list --team platform --status healthy
go run ./enitity stack list --from now-7d --to now
go run ./enitity stack get stk-001 --events 5 --include-audit
go run ./enitity stack create name=search team=data status=healthy region=eu-west-1 tags=internal,indexing
go run ./enitity stack update stk-001 status=degraded tags=critical,customer
go run ./enitity stack restart stk-002 --reason rollout --drain=false
go run ./enitity stack pause stk-001 stk-002
go run ./enitity stack pause all --filter "team == 'platform'" --team platform --status healthy
go run ./enitity stack summary --team platform
go run ./enitity stack seed
go run ./enitity admin stack list --include-archived
go run ./enitity admin stack get stk-003 --include-secret
go run ./enitity catalog cluster list --provider aws
```

`stack pause` still needs a positional id because that is how entity bulk actions are currently generated. In filter mode the example uses `all` as a placeholder and the real selection comes from the populated filter flags.

## Serve Mode

Start the served executor mode:

```bash
cd examples
go run ./enitity openapi serve --enable-executor --port 8080
```

Then exercise the same entity registrations over HTTP:

```bash
curl 'http://localhost:8080/api/v1/stack?team=platform&status=healthy&from=now-7d&to=now'
curl 'http://localhost:8080/api/v1/stack?team=platform&status=healthy&from=now-7d&to=now&__lookup=filters'
curl -X HEAD 'http://localhost:8080/api/v1/stack?team=platform&status=healthy'
curl 'http://localhost:8080/api/v1/admin/stack/stk-001'
curl -X POST 'http://localhost:8080/api/v1/stack/stk-001/restart' \
  -H 'Content-Type: application/json' \
  -d '{"reason":"deploy","drain":false}'
```

This repo does not expose a separate gRPC entity server today, so the example uses the existing HTTP executor serve path that clicky already supports.

## Operation Catalog UI

A Vite + React webapp under `webapp/` renders the entity registrations as an
operation catalog using `@flanksource/clicky-ui`'s `OperationCatalog`
component. The built assets are embedded into the Go binary via `go:embed`
and served by a new `serve-ui` subcommand.

### Quick start with Taskfile

All workflows are wrapped in `examples/enitity/Taskfile.yaml`:

```bash
cd examples/enitity
task --list            # show available tasks

task build             # install webapp deps, vite build, embed into Go binary
task run               # rebuild clicky-ui, refresh go:embed assets, then start serve-ui

task webapp:dev        # Vite HMR (expects the Go server already running on 8080)
task dev               # start Go server + Vite HMR together (one terminal)

task cli -- stack list # run CLI commands through the compiled binary
task e2e:test          # build + start the real server, then run Playwright
task e2e:test:headed   # same suite, headed
task e2e:test:ui       # Playwright UI mode

task clean             # remove built assets and node_modules
task ci                # build webapp + Go binary (for CI)
```

`webapp/dist/index.html` is committed as a placeholder so `go build`
always succeeds before the frontend has been built; it renders a short
message telling you to run `task webapp:build` (or `task build`).

### Developing against a local clicky-ui

`serve-ui --dev` runs the Go executor API **and** launches the Vite dev server
(HMR) from `webapp/` in one process. Vite resolves `@flanksource/clicky-ui`
from the sibling checkout at `../../../../clicky-ui/packages/ui/dist` and proxies
`/api` back to this Go process (it injects `CLICKY_EXAMPLE_API_URL` so the proxy
follows `--host`/`--port`). Open the printed Vite URL (default
`http://localhost:5173/`) — edits to clicky-ui source show up on reload without
rebuilding the embedded bundle.

```bash
cd examples/enitity
task dev                              # = serve-ui --dev (Go API + Vite HMR)
# or directly, choosing ports:
go run . serve-ui --dev --port 8080 --ui-port 5173
```

Prerequisites (all from a source checkout):

- `pnpm install` has been run once in `webapp/`.
- `github.com/flanksource/clicky-ui` is checked out beside `clicky/` and its
  library dist is built. To pick up clicky-ui source edits live, run its build in
  watch mode in that repo (e.g. `pnpm --filter @flanksource/clicky-ui build`,
  re-run on change) — Vite excludes clicky-ui from its prebundle, so a rebuilt
  dist is reflected on the next reload.

Use `task webapp:dev` instead to run only the Vite dev server when the Go API is
already up on port 8080.

## Playwright E2E

The Playwright suite lives in `e2e/` and runs against the real demo app:

- it builds the React UI and embeds it into the Go binary
- it starts `./entity-demo serve-ui` in the background
- it exercises the live `/api/openapi.json` and `/api/v1/...` routes
- it does **not** mock network calls, intercept routes, or inject fixture data

Install the test dependencies once:

```bash
cd examples/enitity
task e2e:install
```

Run the suite:

```bash
cd examples/enitity
task e2e:test
```

For local debugging:

```bash
task e2e:test:headed
task e2e:test:ui
```

### Manual workflow (without Taskfile)

```bash
cd examples/enitity/webapp
pnpm install
pnpm build
cd ..
go build -o entity-demo .   # embed the fresh dist/ into the binary
./entity-demo serve-ui --port 8080
```

Then open `http://localhost:8080/`. The UI fetches `/api/openapi.json` and
calls the clicky executor at `/api/v1/...` for each entity action. Entity
detail pages and GET requests in the API Explorer render through shared
`<Clicky />`, so JSON/PDF view switching and downloads are available by default
on replayable Clicky results. The top-level `Link Examples` route exercises the
new inline `clicky.Link` and `clicky.LinkCommand` surfaces against the same
demo API, including deep-linked command pages at `/links/commands/:operationId`.
