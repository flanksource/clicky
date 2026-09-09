// Package entity is the canonical home for clicky's entity + operation model:
// the entity registry and CLI generation (Entity, EntityBuilder, RegisterEntity,
// GenerateCLI), the command-function registries (AddCommand, the cobra
// annotation metadata), the RPC operation model (RPCOperation, Schema, …), and
// named, reusable filter/lookup definitions that work across both static
// (Go-struct) and dynamic (JSON-Schema) entities.
//
// A filter is defined once — in Go via RegisterFilter, or declaratively via
// RegisterFilterSpec — and reused by name from any number of entities. The core
// abstraction (FilterSource + FilterContext) never sees a typed ListOpts, so the
// same definition serves a compile-time Go entity and a schema-driven dynamic
// entity identically.
//
// On a static entity, attach a named filter with the typed adapter:
//
//	clicky.NewEntity[Task, TaskOpts, Task]("tasks").
//		Filters(entity.Use[TaskOpts]("users").As("owner")).
//		List(listTasks).
//		Register()
//
// # Observing operations
//
// Install listeners during application startup, even after entity registration
// and CLI generation. Each subscription is independent, including duplicates:
//
//	stop := entity.RegisterOperationListener(func(ctx context.Context, event entity.OperationEvent) {
//		if event.Entity != "widgets" || event.Verb == "list" || event.Verb == "get" {
//			return
//		}
//		if err := record(ctx, entity.OperationSurfaceFromContext(ctx), event); err != nil {
//			logger.Errorf("recording operation event: %v", err)
//		}
//	})
//	defer stop()
//
// Callbacks run synchronously in registration order after the operation returns.
// They return no error: Clicky returns the operation's original result and error
// unchanged. Subscribers own recording failures, logging, and any retries.
// Callbacks must not panic; a panic interrupts delivery to later listeners.
// The context retains caller values and cancellation, including for legacy handlers.
// Generated CLI, HTTP, and aichat tool entry points label it cli, http, and mcp.
// Listeners own parameter redaction and must treat the result as read-only.
//
// Keep callbacks fast: synchronous delivery adds their latency to the caller.
// Clicky provides no background queue or workers. A subscriber that hands off
// work asynchronously owns data snapshots, context lifetime, backpressure,
// and shutdown. Delivery is in-process notification, not durable delivery.
//
// Coverage starts at registered data functions, not parameter parsing or
// authorization before dispatch. It includes CRUD variants, primary/custom
// actions, selected/filtered bulk actions, dynamic specs, and registered admin
// list/get/actions. Lookups, completions, AddCommand, standalone PagedFunc or
// dynamic-family handlers, raw HTTP handlers, and direct model calls are outside
// this boundary. TargetID is the supplied ID or alias for single-target calls;
// creates, lists, primary actions, and bulk calls leave it empty. Bulk selections
// remain in Parameters and Args; no generic GUID resolution is attempted.
//
// The dependency runs one way: this package depends only on clicky
// subpackages (api, flags, formatters, task) plus cobra/pflag — never on the
// root clicky package. The root clicky package imports this one and re-exports
// the model via type aliases and thin wrappers (see entity_aliases.go), so
// callers keep using clicky.NewEntity, clicky.Entity, clicky.RegisterEntity,
// etc. unchanged. Host-owned globals (the CLI format flags, HTTPie argument
// parsing) are injected through the RenderResult / ParseArgs hooks in render.go.
package entity
