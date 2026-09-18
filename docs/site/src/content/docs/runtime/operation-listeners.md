---
title: Operation listeners
description: Observe every completed entity operation for auditing, metrics or change feeds.
---

An operation listener is called after every registered operation completes, whether it succeeded or failed, on every surface. Use it for audit trails, metrics or cache invalidation without touching individual handlers.

```go
import "github.com/flanksource/clicky/entity"

stop := entity.RegisterOperationListener(func(ctx context.Context, e entity.OperationEvent) {
	if e.Verb == "list" || e.Verb == "get" {
		return
	}
	if err := audit.Record(ctx, audit.Entry{
		Surface:  entity.OperationSurfaceFromContext(ctx), // cli | http | mcp
		Entity:   e.Entity,
		Verb:     e.Verb,
		Target:   e.TargetID,
		Admin:    e.Admin,
		Failed:   e.Error != nil,
		Duration: e.Duration,
	}); err != nil {
		logger.Errorf("recording audit entry: %v", err)
	}
})
defer stop()
```

## OperationEvent

| Field | Meaning |
| --- | --- |
| `Entity`, `Verb` | e.g. `stack`, `restart` |
| `Admin` | the operation belongs to the entity's [admin view](/entities/hierarchy/#admin-views) |
| `TargetID` | the ID or alias supplied to a single-target call. Empty for create, list, primary and bulk calls |
| `Parameters`, `Args` | raw transport values (a fresh copy per listener). Bulk selections stay here and are not resolved into IDs |
| `Result` | the operation's result, **borrowed**. Do not mutate it or anything reachable from it |
| `Error` | the operation's own error |
| `Duration` | the operation's own time, not including listeners |

## Delivery contract

- **Synchronous and in-process.** Listeners run in registration order after the operation returns, and they add latency to the caller. Keep them fast. If you hand work to a goroutine, you own snapshotting the data, the context lifetime, backpressure and shutdown. clicky provides no queue and no durability.
- **Cannot change the outcome.** Listeners return nothing. The caller gets the original result and error.
- **Must not panic.** A panic stops delivery to the listeners after it.
- **Independent subscriptions.** Registering the same function twice runs it twice. `stop()` is idempotent and does not wait for deliveries already in flight.
- **Install any time.** Each invocation takes a snapshot of the subscriptions when it starts, so listeners can be registered after the entities and after `GenerateCLI`.
- **Redaction is yours.** Parameters are raw. Strip secrets before persisting them.

## Coverage

**Observed:** CRUD verbs in every variant, primary and custom actions, bulk actions in ID and filter mode, dynamic entity specs, admin list/get/actions, and commands generated from options structs (`AddCommand` and similar).

**Not observed:** lookups, shell completions, standalone `PagedFunc` or dynamic-family handlers, raw HTTP handlers, and direct calls to your model. Observation starts at the registered data function. It does not include parameter parsing or any authorization that runs before dispatch.

To name a hand-written cobra command the same way in your own trail, use `clicky.CommandIdentity(cmd)`. It returns `(entity, verb)`.
