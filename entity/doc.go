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
//	stop := entity.RegisterOperationListener(func(ctx context.Context, event entity.OperationEvent) error {
//		if event.Entity != "widgets" || event.Verb == "list" || event.Verb == "get" {
//			return nil
//		}
//		return record(ctx, entity.OperationSurfaceFromContext(ctx), event)
//	})
//	defer stop()
//
// Callbacks run synchronously in registration order after the operation returns.
// All callbacks run even if one returns an error; they must not panic. The
// context retains caller values and cancellation, including for legacy handlers.
// Generated CLI, HTTP, and aichat tool entry points label it cli, http, and mcp.
// Listeners own parameter redaction and must treat the result as read-only.
//
// OperationListenerError retains Result, OperationError, and all ListenerErrors;
// errors.Is/As can reach each underlying failure. A nil OperationError means
// the business operation succeeded: listener failure does not roll it back and
// is not a reason to retry a mutation. HTTP returns status 500 with code
// operation_listener_failed, operation_succeeded, result, listener_error_count,
// and a sanitized operation_error when applicable. Listener details stay in
// server logs. CLI and tool errors state whether the operation succeeded.
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
