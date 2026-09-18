---
title: Entity families
description: Serve entities that are created while the server is running, such as database rows or tenants.
---

The entity registry is a **snapshot taken at startup**. `GenerateCLI`, the cobra tree and the HTTP mux are all built once. An entity that comes into existence later has no route, for example a saved report a user just created. A **dynamic entity family** registers one route for a whole shape and resolves the instance on each request:

```
/api/v1/{family.Name}/{name}
```

```go
import "github.com/flanksource/clicky/entity"

entity.RegisterDynamicEntityFamily(entity.DynamicEntityFamily{
	Name:   "report",
	Parent: "reporting",
	Resolve: func(ctx context.Context, name string) (entity.DynamicEntitySpec, error) {
		def, err := reports.Find(ctx, name)
		if errors.Is(err, reports.ErrNotFound) {
			return entity.DynamicEntitySpec{}, entity.UnknownDynamicEntity("report", name) // 404
		}
		if err != nil {
			return entity.DynamicEntitySpec{}, err
		}
		return def.Spec(), nil // ListType, ItemType, List, Filters, ...
	},
	List: func(ctx context.Context) ([]entity.DynamicEntitySpec, error) {
		return reports.AllSpecs(ctx) // describes current instances in the OpenAPI document
	},
})
```

```bash
curl 'localhost:8080/api/v1/report/daily-revenue?region=eu'
curl 'localhost:8080/api/v1/report/daily-revenue?__lookup=filters'
```

## Contract

- **`Resolve` runs in front of every request** to the family, so it must be cheap. Nothing is cached, which means an instance created a moment ago is reachable immediately.
- **An unknown name is `entity.UnknownDynamicEntity(family, name)`**, a 404 `not_found`, never a zero spec. "No such report" and "a report that does nothing" are different answers.
- **`List` is called per request** when rendering the OpenAPI document, so the document always describes the instances that exist now.
- **`Paged`** (optional) serves pages and exports for an instance through the tabular export contract (`entity.PageRequest` / `entity.PageResponse`). When it is nil, the instance's `List` serves the data.
- Lookups (`?__lookup=filters`) resolve from the resolved spec's `Filters` via `entity.ResolveDynamicLookup`.

## Registration timing

Register every family **before** the server builds its mux (`rpc.RegisterRoutes` / `openapi serve`). The mux pattern for a family is written once from the families that are registered at that moment. A family registered later still appears in the OpenAPI document, which is resolved per request, but its paths return 404 because the mux has no pattern for them.

`RegisterDynamicEntityFamily` replaces an existing family with the same `Name`. `UnregisterDynamicEntityFamily(name)` removes it: its path then falls through to ordinary operation lookup and returns 404.

A family `Name` must not match the first path segment of a registered operation. Registered operations are resolved first, so a colliding family is never reached for those paths.

Families are an HTTP surface. They are not in the entity registry, so they generate no CLI commands and do not trigger [operation listeners](/runtime/operation-listeners/).
