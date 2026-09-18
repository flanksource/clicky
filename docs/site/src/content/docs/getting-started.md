---
title: Getting started
description: Register your first entity and serve it as a CLI and an HTTP API.
---

This walkthrough builds a small `widgets` entity. It covers a list with filters, a get by ID and one action, all served from the CLI and over HTTP.

## Install

```bash
go get github.com/flanksource/clicky
```

## 1. Define the row type

A list row must implement `clicky.EntityItem`:

```go
type Widget struct {
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Team   string    `json:"team"`
	Status string    `json:"status" pretty:"color=blue"`
	Seen   time.Time `json:"seen"`
}

func (w Widget) GetID() string   { return w.ID }
func (w Widget) GetName() string { return w.Name }
```

`GetID` is the value used in `get <id>`, in action routes and as the `_id` field that list responses add to each row. `GetName` is the default label when another filter uses this entity as an option source.

## 2. Define the list options

The `ListOpts` struct is the list operation's parameter surface. Each `flag:` tagged field becomes a cobra flag, an OpenAPI query parameter and a UI control:

```go
type WidgetListOpts struct {
	Team   string    `flag:"team"   help:"Owning team"`
	Status []string  `flag:"status" help:"One or more statuses"`
	From   time.Time `flag:"from"   help:"Seen after (e.g. now-7d)"`
	To     time.Time `flag:"to"     help:"Seen before"`
}
```

See [List options](/entities/list-options/) for the supported types and tags.

## 3. Register the entity

```go
func init() {
	clicky.NewEntity[Widget, WidgetListOpts, Widget]("widgets").
		Aliases("widget", "w").
		List(func(opts WidgetListOpts) ([]Widget, error) {
			return store.List(opts)
		}).
		Get(func(id string) (Widget, error) {
			return store.Get(id)
		}).
		WithAction(clicky.Action("restart", func(id string, _ map[string]string) (Widget, error) {
			return store.Restart(id)
		}).WithShort("Restart a widget")).
		Register()
}
```

The three type parameters are:

| Parameter | Meaning |
| --- | --- |
| `T` | The list row type. It must implement `EntityItem`. |
| `ListOpts` | The struct the list flags bind to. |
| `R` | The type returned by `Get`, `Create` and `Update`. It is often `T`, or a richer detail type. |

## 4. Generate the CLI

```go
func main() {
	root := &cobra.Command{Use: "app"}
	clicky.BindAllFlags(root.PersistentFlags())
	clicky.GenerateCLI(root)
	extensions.CobraExtensions(root).All() // adds the openapi (incl. `openapi serve`), mcp and docs subcommands
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
```

`GenerateCLI` must run after every `Register()` call. It also attaches deferred subcommands registered with `RegisterSubCommand` (see [Custom commands](/entities/commands/)).

```bash
app widgets --team platform --status healthy,degraded --from now-7d
app widgets get w-001
app widgets restart w-001
app widgets --format json
```

The bare entity command runs `list`. The `list` subcommand still exists, but only one list route is published over HTTP.

## 5. Serve it

```bash
app openapi serve --enable-executor --port 8080
```

```bash
curl 'localhost:8080/api/v1/widgets?team=platform&status=healthy'
curl 'localhost:8080/api/v1/widgets/w-001'
curl -X POST 'localhost:8080/api/v1/widgets/w-001/restart'
curl 'localhost:8080/api/v1/widgets?team=platform&__lookup=filters'   # filter-bar metadata
```

The same handlers run in both places. Over HTTP, clicky parses query and body parameters into the same flag map the CLI builds, then decodes it into `WidgetListOpts`.

## Next steps

- Add [filters](/filters/overview/) so `team` renders as a dropdown with labelled options.
- Make handlers [context-aware](/runtime/context/) to resolve per-request databases or tenants.
- Add a [bulk action](/entities/bulk-actions/) for the selection toolbar.
- The full runnable example is `examples/enitity` in the clicky repository.
