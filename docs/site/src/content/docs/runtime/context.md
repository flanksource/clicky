---
title: Request context
description: Context-aware handlers, request-scoped state, and what clicky puts on the context.
---

Every entity handler, action, bulk action, custom command and filter has a context-aware form. When one is set, clicky prefers it and passes in:

- **`cmd.Context()`** on the CLI
- **`r.Context()`** over HTTP, so cancellation follows the client connection

Use the context to resolve **request-scoped state**, such as the tenant, the database handle, the auth principal or the trace, instead of reaching for process globals. A process global is correct for a single-user CLI and wrong for a server that handles many users at once.

## Context-aware forms at a glance

| Plain | Context-aware |
| --- | --- |
| `List`, `ListPaged` | `ListWithContext`, `ListPagedWithContext` |
| `Get`, `GetWithFlags` | `GetWithContext`, `GetWithFlagsAndContext` |
| `Create`, `Update`, `Delete` | `CreateWithContext`, `UpdateWithContext`, `DeleteWithContext` |
| — | `DeleteWithFlagsAndContext` |
| `Action`, `ActionWithFlags` | `ActionWithContext`, `ActionWithFlagsAndContext`, `TypedActionWithContext` |
| — | `entity.PrimaryActionWithContext` |
| `BulkAction`, `BulkActionWithFilter` | `BulkActionWithFilterAndContext` |
| `AddCommand`, `AddNamedCommand` | `AddCommandWithContext`, `AddNamedCommandWithContext` |
| `Filter.Options` | `ContextFilter.OptionsWithContext` |
| `SearchableFilter.OptionsWithQuery` | `ContextSearchableFilter.OptionsWithQueryAndContext` |
| — | named filter sources get `FilterContext.Context` |

## Injecting request state

clicky gives you the request context. You decide what goes on it, usually in HTTP middleware for the server and in `PersistentPreRunE` for the CLI:

```go
type tenantKey struct{}

func WithTenant(ctx context.Context, t *Tenant) context.Context { return context.WithValue(ctx, tenantKey{}, t) }
func TenantFrom(ctx context.Context) (*Tenant, error) {
	t, ok := ctx.Value(tenantKey{}).(*Tenant)
	if !ok {
		return nil, entity.NewStatusError(http.StatusUnauthorized, "no_tenant", "request has no tenant")
	}
	return t, nil
}

clicky.NewEntity[Invoice, InvoiceOpts, Invoice]("invoices").
	ListWithContext(func(ctx context.Context, opts InvoiceOpts) ([]Invoice, error) {
		tenant, err := TenantFrom(ctx)
		if err != nil {
			return nil, err
		}
		return tenant.DB.ListInvoices(ctx, opts)
	})
```

Fail loudly when the state is missing. Silently falling back to a default tenant is exactly the bug this seam exists to prevent.

## What clicky puts on the context

| Accessor | Package | Value |
| --- | --- | --- |
| `entity.OperationSurfaceFromContext(ctx)` | `entity` | `"cli"`, `"http"` or `"mcp"`; empty for direct calls |
| `rpc.RequestFromContext(ctx)` | `rpc` | the originating `*http.Request`; `ok == false` on the CLI |
| `entity.TraceIDFromContext(ctx)` | `entity` | the trace ID used in [structured error](/runtime/errors/) responses |
| `task.TaskFromContext(ctx)` | `task` | the owning `*task.Task`; `nil` outside a task context |
| `task.GroupFromContext(ctx)` | `task` | the owning `*task.Group`; `nil` outside a group context |

`rpc.RequestFromContext` is how a create or update handler reads the **raw nested JSON body**. The executor re-buffers the body after flattening it into string flags, so the handler can still decode it:

```go
UpdateWithContext(func(ctx context.Context, id string, body map[string]any) (Connection, error) {
	r, ok := rpc.RequestFromContext(ctx)
	if !ok {
		return store.UpdateFromFlags(ctx, id, body) // CLI: key=value args
	}
	var req UpdateConnection
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return Connection{}, entity.NewStatusError(http.StatusBadRequest, "invalid_body", err.Error())
	}
	return store.Update(ctx, id, req)
})
```

You can also read headers, the authenticated principal set by your middleware, or the client IP from the same request.

## Progress from a task context

Task callbacks receive a context containing their own task. Pass that context to downstream functions so they can report progress without another task parameter:

```go
func loadSchema(ctx context.Context) error {
	current := task.TaskFromContext(ctx)
	if current == nil {
		return errors.New("loading a schema requires a task context")
	}
	current.SetDescription("Loading schema")
	return readSchema(ctx)
}

run := task.StartTask("Prepare", func(ctx commonscontext.Context, _ *task.Task) (struct{}, error) {
	return struct{}{}, loadSchema(ctx)
}, task.WithContext(requestCtx))
_, err := run.GetResult()
```

Here `requestCtx` is the handler's request context. Import `github.com/flanksource/clicky/task` and alias `github.com/flanksource/commons/context` as `commonscontext`. `TaskFromContext` accepts a standard `context.Context`, including contexts derived with deadlines, cancellation, or values. It also works with `Task.Context()` and `Task.FlanksourceContext()` for inline steps, planned steps, and managed runs. For those operations, explicitly pass the step's or run's task context to its work.

A child task identifies itself in its own context while its parent's context still identifies the parent. Retries retain task identity. Callers' contexts retain their original values and cancellation; creating a task does not modify its input context. `TaskFromContext` only looks up the task: callers decide whether a missing task is valid for their operation.

Use `task.WithOnProgress(publish)` when an owner needs to publish intermediate snapshots. `SetDescription`, `SetProgress`, and `SetProgressMetrics` invoke the callback after releasing the task lock, so the callback can read snapshots. Install it when creating the task or step; the callback must support concurrent calls. Task status and lifetime still belong to the task manager or the inline step's `Finish` call.

`Task.StartTime()` and `Task.Duration()` describe execution. A task cancelled while queued has no execution start and zero execution duration. `Task.WaitTime()` measures enqueue-to-start, or enqueue-to-cancellation when the task never starts; its value stops changing when execution starts.

## Context in filters

Filters are resolved during list calls, lookups and completions. They need the same request state as handlers:

```go
func (f projectFilter) OptionsWithContext(ctx context.Context, o IssueOpts) map[string]api.Textable {
	tenant, err := TenantFrom(ctx)
	if err != nil {
		return nil
	}
	return tenant.Projects(ctx)
}
```

Named filter sources receive it as `FilterContext.Context`. Use `fc.Ctx()`, which returns `context.Background()` when no request context exists, for example during shell completion.

### Actions that need the target ID

A typed action's filters may depend on the row being acted on. The action's options struct can implement one of these interfaces:

- `entity.ActionIDSetter`: `SetClickyActionID(id string)`
- `entity.ActionContextSetter`: `SetClickyActionContext(ctx context.Context, id string)`

clicky calls the setter after decoding flags and before resolving filters or running the handler. See [Actions](/entities/actions/#filters-on-action-flags).

## Guidelines

- Implement **one** variant per verb. If both are set, the context variant wins. Precedence exists to make migration safe, not to be a fallback.
- Pass `ctx` to every downstream call (DB, HTTP) so client disconnects cancel work.
- Do not store the context past the call. [Operation listeners](/runtime/operation-listeners/) that hand work to goroutines own their own context lifetime.
