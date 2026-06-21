// Package entity provides named, reusable filter/lookup definitions that work
// across both static (Go-struct) clicky entities and dynamic (JSON-Schema)
// entities.
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
// This package imports the root clicky package (for the Filter/EntityItem
// interfaces and the entity registry); the root never imports this package.
package entity
