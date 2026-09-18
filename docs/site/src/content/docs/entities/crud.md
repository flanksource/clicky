---
title: CRUD operations
description: List, get, create, update and delete handlers and the commands and routes they generate.
---

## List

```go
List(func(opts StackListOpts) ([]Stack, error))
ListWithContext(func(ctx context.Context, opts StackListOpts) ([]Stack, error))
```

- CLI: `app stack [flags]` and `app stack list [flags]`
- HTTP: `GET /api/v1/stack?team=…`

Before your handler runs, clicky decodes the flag map into `ListOpts` and then calls each [filter](/filters/overview/)'s `Lookup(&opts)`. Filters can therefore normalize raw values (`platform` → `team/platform`) before your handler sees them.

Each returned row is serialized with an extra `_id` field taken from `GetID()`.

## Get

```go
Get(func(id string) (StackDetail, error))
GetWithContext(func(ctx context.Context, id string) (StackDetail, error))
```

- CLI: `app stack get <id>` (alias `inspect`)
- HTTP: `GET /api/v1/stack/{id}`

### Get with typed flags

Use `GetWithFlags` when the detail view takes options. The flags struct implements `ActionFlags` through a no-op marker method:

```go
type InspectFlags struct {
	Events       int  `flag:"events" default:"10" help:"Recent events to include"`
	IncludeAudit bool `flag:"include-audit"`
}

func (InspectFlags) ClickyActionFlags() {}

clicky.NewEntity[Stack, StackListOpts, StackDetail]("stack").
	GetWithFlags(InspectFlags{}, func(id string, flags map[string]string) (StackDetail, error) {
		opts, err := clicky.BuildOpts[InspectFlags](flags)
		if err != nil {
			return StackDetail{}, err
		}
		return store.Get(id, opts)
	})
```

```bash
app stack get stk-001 --events 5 --include-audit
curl 'localhost:8080/api/v1/stack/stk-001?events=5&include-audit=true'
```

`GetWithFlagsAndContext` is the context-aware form.

## Create and update

```go
Create(func(body map[string]any) (StackDetail, error))
Update(func(id string, body map[string]any) (StackDetail, error))
CreateWithContext(func(ctx context.Context, body map[string]any) (StackDetail, error))
UpdateWithContext(func(ctx context.Context, id string, body map[string]any) (StackDetail, error))
```

The CLI accepts HTTPie-style `key=value` arguments, or a JSON object on stdin:

```bash
app stack create name=search team=data region=eu-west-1 tags=internal,indexing
app stack update stk-001 status=degraded
echo '{"name":"search","team":"data"}' | app stack create
```

| Verb | Route | Body |
| --- | --- | --- |
| create | `POST /api/v1/stack` | JSON object |
| update | `PUT /api/v1/stack` | JSON object that includes `id`; clicky passes `id` as the first argument and removes it from `body` |

The `body` map is built from the flat flag map, so its values are strings. If you need the original nested JSON over HTTP, use the context variant and read the request:

```go
CreateWithContext(func(ctx context.Context, body map[string]any) (StackDetail, error) {
	if r, ok := rpc.RequestFromContext(ctx); ok {
		var full CreateStackRequest
		if err := json.NewDecoder(r.Body).Decode(&full); err != nil {
			return StackDetail{}, entity.NewStatusError(http.StatusBadRequest, "invalid_body", err.Error())
		}
		return store.Create(ctx, full)
	}
	return store.CreateFromFlags(ctx, body) // CLI path: no *http.Request
})
```

## Delete

```go
Delete(func(id string) error)
DeleteWithContext(func(ctx context.Context, id string) error)
```

- CLI: `app stack delete <id>`
- HTTP: `DELETE /api/v1/stack/{id}`

To delete many rows at once, add a [bulk action](/entities/bulk-actions/). A bulk action named `delete` is routed as `DELETE /api/v1/stack/{id}/delete`, with the IDs comma-joined in `{id}`. It does not collide with the entity's own delete.

## ID completion

```go
ValidArgs(func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return store.IDsWithPrefix(toComplete), cobra.ShellCompDirectiveNoFileComp
})
```

`ValidArgs` completes the `<id>` operand of `get`, `delete`, `update` and every single-row action.
