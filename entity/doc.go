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
// The dependency runs one way: this package depends only on clicky
// subpackages (api, flags, formatters, task) plus cobra/pflag — never on the
// root clicky package. The root clicky package imports this one and re-exports
// the model via type aliases and thin wrappers (see entity_aliases.go), so
// callers keep using clicky.NewEntity, clicky.Entity, clicky.RegisterEntity,
// etc. unchanged. Host-owned globals (the CLI format flags, HTTPie argument
// parsing) are injected through the RenderResult / ParseArgs hooks in render.go.
package entity
