# Clicky

Clicky is a Go toolkit for turning structured data and Cobra CLIs into polished command-line, web, and AI-facing interfaces. It includes:

- multi-format output for structs, maps, slices, and schema-driven data
- styled terminal and HTML rendering based on `pretty` tags and Tailwind-like classes
- concurrent task execution with progress rendering, retries, cancellation, and typed results
- Echo middleware configured from Go or YAML
- Cobra extensions for OpenAPI, Swagger UI, HTTP command execution, and MCP servers
- helper packages for text processing, command execution, entity commands, and linting

The module path is:

```bash
go get github.com/flanksource/clicky
```

## Install the CLI

```bash
go install github.com/flanksource/clicky/cmd/clicky@latest
```

For local development:

```bash
make build
./clicky --help
```

The `clicky` binary formats JSON/YAML data with a schema, validates schemas, runs the API linter, and can expose its own Cobra commands through OpenAPI and MCP.

```bash
clicky pretty --schema examples/order-schema.yaml examples/example-data.json
clicky pretty --schema examples/order-schema.yaml --format html --output order.html examples/example-data.json
clicky schema validate examples/order-schema.yaml
clicky schema example -o schema.yaml
clicky lint ./...
```

## Library Usage

### Format Structured Data

```go
package main

import (
	"fmt"

	"github.com/flanksource/clicky"
)

type Server struct {
	Name   string `pretty:"label=Server"`
	Status string `pretty:"color=green,sort"`
	CPU    int    `pretty:"label=CPU %,color=blue"`
}

func main() {
	servers := []Server{
		{Name: "api-01", Status: "ready", CPU: 31},
		{Name: "db-01", Status: "ready", CPU: 54},
	}

	out, err := clicky.Format(servers, clicky.FormatOptions{Format: "pretty"})
	if err != nil {
		panic(err)
	}
	fmt.Print(out)
}
```

Supported formats include `pretty`, `json`, `yaml`, `csv`, `markdown`, `html`, `html-react`, `html-static`, `pdf`, `slack`, `excel`, and `tree`. Common aliases such as `md`, `yml`, and `xlsx` are accepted.

`FormatOptions.Format` can also describe multiple sinks:

```go
clicky.PrintAndWriteSinks(data, clicky.FormatOptions{
	Format: "pretty,json=out.json,markdown=summary.md",
})
```

### Run Typed Tasks

```go
package main

import (
	"fmt"
	"time"

	"github.com/flanksource/clicky"
)

func main() {
	job := clicky.StartTask("load servers", func(ctx clicky.Context, t *clicky.Task) ([]string, error) {
		for i := 1; i <= 3; i++ {
			t.SetProgress(i, 3)
			time.Sleep(100 * time.Millisecond)
		}
		return []string{"api-01", "db-01"}, nil
	})

	servers, err := job.GetResult()
	if err != nil {
		panic(err)
	}
	fmt.Println(servers)

	clicky.WaitForGlobalCompletion()
}
```

For grouped work, use `clicky.StartGroup[T]` or `task.StartGroup[T]` with `task.WithConcurrency(n)`.

### Add Common Flags to a Cobra CLI

```go
rootCmd := &cobra.Command{Use: "myapp"}
flags := clicky.BindAllFlagsToCommand(rootCmd, "format", "tasks")

rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
	flags.UseFlags()
}
```

This adds grouped logging, formatting, and task flags such as `--format`, `--filter`, `--no-color`, `--no-progress`, and `--max-concurrent`.

### Add OpenAPI and MCP to a Cobra CLI

```go
import "github.com/flanksource/clicky/extensions"

extensions.CobraExtensions(rootCmd).All()
```

This adds:

- `openapi generate`
- `openapi validate <file>`
- `openapi serve`
- `mcp serve`
- `mcp config`
- `mcp tools`
- `mcp install`
- `mcp prompts`

Example commands:

```bash
myapp openapi generate --format yaml --output openapi.yaml
myapp openapi serve --port 8080 --enable-executor
myapp mcp tools --format markdown
myapp mcp serve --auto-expose
```

### Configure Echo Middleware

```go
package main

import (
	"log"

	"github.com/flanksource/clicky/middleware"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	config, err := middleware.LoadConfigFromYAML("config/production.yaml")
	if err != nil {
		log.Fatal(err)
	}
	if err := middleware.ValidateConfig(config); err != nil {
		log.Fatal(err)
	}

	middleware.ApplyMiddleware(e, config)
	log.Fatal(e.Start(":8080"))
}
```

Preset helpers are also available:

```go
middleware.ApplyMinimalMiddleware(e)
middleware.ApplyDefaultMiddleware(e)
middleware.ApplyProductionMiddleware(e)
```

### Stream Live Data over SSE

The `sse` package carries the server-sent-events plumbing:

- `sse.Snapshot` / `sse.ServeSnapshot` re-load a value on a tick or a wake and send it only when it changed. When nothing changed they send a keepalive ping instead. `Exclude` lists fields that churn without meaning anything, such as a fetch timestamp or a sampled counter. Those fields are left out of the change check, but a frame sent for another reason still carries their current values.
- `sse.Notifier` wakes every subscribed stream at once.
- `sse.Hub` multiplexes every stream of a browser tab over one connection. Browsers allow six HTTP/1.1 connections per host across all tabs. `Guard` refuses the direct per-topic EventSource connections that a stale page would otherwise hold open.

```go
package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/flanksource/clicky/sse"
)

type inventory struct {
	Items     []string  `json:"items"`
	FetchedAt time.Time `json:"fetchedAt"`
}

func main() {
	var changed sse.Notifier
	var mu sync.Mutex
	items := []string{"widget"}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/items", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		items = append(items, r.URL.Query().Get("name"))
		mu.Unlock()
		changed.Notify() // every open stream reloads now, not at its next tick
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/items/stream", func(w http.ResponseWriter, r *http.Request) {
		wake, cancel := changed.Subscribe()
		defer cancel()
		err := sse.ServeSnapshot(w, r, sse.SnapshotOptions{
			Load: func(context.Context) (any, error) {
				mu.Lock()
				defer mu.Unlock()
				return inventory{Items: append([]string(nil), items...), FetchedAt: time.Now()}, nil
			},
			Interval: 5 * time.Second,
			Wake:     wake,
			Exclude:  []string{"fetchedAt"}, // a new timestamp alone is not news
			Event:    "items",
		})
		if err != nil {
			log.Printf("items stream: %v", err)
		}
	})

	hub := sse.NewHub(sse.HubOptions{Build: "dev"})
	root := hub.Guard(mux)
	hub.Register(mux, root)
	log.Fatal(http.ListenAndServe(":8080", root))
}
```

Try it from a shell (these clients are not browsers, so `Guard` lets the direct stream through):

```bash
curl -N localhost:8080/api/items/stream          # one frame, then ": ping" every 5s
curl -X POST 'localhost:8080/api/items?name=gear' # the stream above sends a new frame at once
curl -N localhost:8080/api/events                # hello frame: {"conn":"<id>","build":"dev"}
curl -X POST localhost:8080/api/events/<id>/subs -d '{"id":"inv","path":"/api/items/stream"}'
```

Over the hub, each event is renamed `<subId>/<name>` (here `inv/items`), and a sub whose handler ends emits `<subId>/__closed`. In the browser, clicky-ui's `createEventHub` speaks this protocol.

## Schema Formatting

The CLI can format dynamic JSON/YAML data using schema files. A schema describes fields, labels, types, styles, color rules, table fields, tree fields, and format-specific options.

```yaml
fields:
  - name: id
    label: Order ID
    type: string
    style: text-blue-600 font-bold
  - name: status
    type: string
    color_options:
      green: completed
      yellow: processing
      red: failed
  - name: total_amount
    type: float
    format: currency
  - name: items
    type: array
    format: table
    table_options:
      fields:
        - name: product_name
          type: string
        - name: quantity
          type: int
        - name: price
          type: float
          format: currency
```

Generate a fuller example with:

```bash
clicky schema example
```

## Package Map

- `api`: render primitives such as text, tables, trees, code blocks, badges, links, stack traces, and schema parsing
- `formatters`: output managers and implementations for terminal, JSON, YAML, CSV, Markdown, HTML, PDF, Excel, Slack, and tree formats
- `task`: task manager, typed tasks, groups, progress rendering, retries, shutdown handling, and output capture
- `middleware`: Echo v4 middleware configuration, validation, auth, interceptors, and presets
- `rpc`: Cobra-to-OpenAPI generation, Swagger UI server, validation, and HTTP command execution
- `sse`: server-sent events: the frame writer, change-only snapshot streams, a fan-out notifier, and the multiplexing hub
- `mcp`: Model Context Protocol server and tool discovery for Cobra commands
- `extensions`: fluent helpers that attach OpenAPI and MCP commands to Cobra roots
- `exec`: command execution wrappers with logging and process-group handling
- `flags`: struct-tag flag binding helpers
- `text`: tokenization, redaction, and line processing utilities
- `lint`: Go analyzer for Clicky API usage

## Development

```bash
go mod download
make test
make lint
make build
```

Useful targeted commands:

```bash
go test ./formatters/...
go test ./task/...
go test ./middleware/...
go test -tags integration ./rpc/... -run TestOpenAPIServe_E2E
```

The task UI bundle is built separately:

```bash
make task-ui
```

## Examples

The `examples/` directory includes focused demos for:

- schema-driven formatting
- Cobra integration
- OpenAPI and Swagger serving
- MCP integration
- Echo middleware
- task manager behavior
- file tree, PDF widgets, and entity workflows

Start with `examples/README.md` and the schema/data pairs in `examples/order-schema.yaml` and `examples/example-data.json`.
