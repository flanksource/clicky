---
title: Admin views & hierarchy
description: Nest entities under parents, add admin-only views, and place entities in the UI tree.
---

## Parent

`Parent` nests the entity's command under a grouping command. The grouping command is created if it does not exist:

```go
clicky.NewEntity[Cluster, ClusterListOpts, Cluster]("cluster").
	Parent("catalog").
	List(store.ListClusters).
	Get(store.GetCluster).
	Register()
```

```bash
app catalog cluster --provider aws
app catalog cluster get c-01
```

Routes follow the command path: `GET /api/v1/catalog/cluster` and `GET /api/v1/catalog/cluster/{id}`.

## Aliases

```go
NewEntity[Stack, StackListOpts, Stack]("stack").Aliases("stacks", "svc")
```

Aliases apply to the cobra command (`app svc get …`). `clicky.GetEntity` and `EntityOptions` filter sources also resolve them.

## Path (UI hierarchy)

`Path` places the entity in the clicky-ui navigation tree without changing its command or route. It is published as `x-clicky.surfaces[].path`:

```go
NewEntity[Queue, QueueOpts, Queue]("queue").
	Parent("messaging").
	Path("jms", "incoming")
```

Build paths from segments. If your domain uses its own separator, convert it with `entity.SplitPath(name, ".")` and `entity.JoinPath(segments)`. The wire separator is always `/`.

## Admin views

`Admin` registers a second set of operations on the same types, under an `admin` command group. Use it for views that show different columns, include hidden rows, or need elevated flags:

```go
clicky.NewEntity[Stack, StackListOpts, StackDetail]("stack").
	List(store.ListStacks).
	Get(store.GetStack).
	Admin(clicky.Entity[Stack, StackListOpts, StackDetail]{
		Filters:      stackFilters,
		List:         store.ListAllStacksIncludingArchived,
		GetFlags:     AdminInspectFlags{},
		GetWithFlags: store.GetStackWithSecrets,
		Actions: []clicky.EntityAction{
			clicky.Action("reconcile", store.Reconcile).WithShort("Force a reconcile"),
		},
	}).
	Register()
```

```bash
app admin stack list --include-archived
app admin stack get stk-003 --include-secret
app admin stack reconcile stk-003
```

| Operation | Route |
| --- | --- |
| admin list | `GET /api/v1/admin/stack` |
| admin get | `GET /api/v1/admin/stack/{id}` |
| admin action | `POST /api/v1/admin/stack/{id}/reconcile` |

The admin entity supports list (all variants), get (all variants) and single-entity `Actions`. It inherits the parent's name and `ValidArgs` when its own are empty. It does not carry create/update/delete or bulk actions.
