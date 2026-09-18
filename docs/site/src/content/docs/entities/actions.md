---
title: Actions
description: Custom verbs on a single entity, on the collection, and as the entity's primary action.
---

An **action** is a custom verb that is not CRUD, such as `restart`, `approve` or `sync`. Each action becomes one cobra subcommand and one HTTP route.

## Single-entity actions

```go
clicky.Action("restart", func(id string, flags map[string]string) (Stack, error) {
	return store.Restart(id)
}).WithShort("Restart a stack")
```

```bash
app stack restart stk-001
curl -X POST localhost:8080/api/v1/stack/stk-001/restart
```

The handler's return value `R` is rendered like any other result: a table, pretty output, or JSON over HTTP. `R` is also the response schema in OpenAPI.

### Action constructors

| Constructor | Handler signature |
| --- | --- |
| `Action(name, fn)` | `func(id string, flags map[string]string) (R, error)` |
| `ActionWithFlags(name, flags, fn)` | same, with typed flags bound on the command |
| `ActionWithContext(name, fn)` | `func(ctx, id, flags) (R, error)` |
| `ActionWithFlagsAndContext(name, flags, fn)` | context + typed flags, raw flag map |
| `TypedActionWithContext(name, opts, fn)` | `func(ctx, id string, opts Opts) (R, error)`, flags already decoded |

Prefer **`TypedActionWithContext`**. The raw-map constructors make you repeat every flag name the struct already declares, and those names drift apart silently when one is renamed.

```go
type RestartFlags struct {
	Reason string `flag:"reason" required:"true"`
	Drain  bool   `flag:"drain"  default:"true"`
}

func (RestartFlags) ClickyActionFlags() {}

clicky.TypedActionWithContext("restart", RestartFlags{},
	func(ctx context.Context, id string, opts RestartFlags) (Stack, error) {
		return store.Restart(ctx, id, opts.Reason, opts.Drain)
	}).WithShort("Restart a stack")
```

```bash
app stack restart stk-001 --reason rollout --drain=false
curl -X POST localhost:8080/api/v1/stack/stk-001/restart \
  -H 'Content-Type: application/json' -d '{"reason":"deploy","drain":false}'
```

### Modifiers

| Method | Effect |
| --- | --- |
| `WithShort(s)` | one-line help / operation summary |
| `WithFlags(flags)` | attach an `ActionFlags` struct after construction |
| `WithMethod("GET")` | override the inferred HTTP method. The method is inferred from the verb: `show`/`describe` → GET, `add`/`new` → POST, `set`/`edit`/`modify` → PUT, `remove`/`destroy` → DELETE, anything else → POST |
| `WithOptionalID()` | make `<id>` optional; the action becomes collection-scoped |
| `WithToolGroup`, `WithToolPermission`, `WithToolHints` | [AI tool metadata](/runtime/ai-tools/) |
| `WithSchedule(OperationScheduleMeta)` | mark the action as schedulable and suggest schedules in editors |

### Filters on action flags

If the action's options struct implements `Filterable[Opts]` (`Filters() []Filter[Opts]`), `TypedActionWithContext` wires those filters into the action. They drive lookups for its form, shell completion, and value normalization before your handler runs.

When a filter's options depend on the target row, let the options struct receive the ID:

```go
type AssignFlags struct {
	stackID string
	Owner   string `flag:"owner"`
}

func (AssignFlags) ClickyActionFlags() {}
func (f *AssignFlags) SetClickyActionID(id string) { f.stackID = id }            // ActionIDSetter
// or: SetClickyActionContext(ctx context.Context, id string)                    // ActionContextSetter
func (AssignFlags) Filters() []clicky.Filter[AssignFlags] { return []clicky.Filter[AssignFlags]{ownersForStack{}} }
```

The ID is set before filters resolve, so `ownersForStack.Options` can offer only the owners valid for that stack. The ID is not duplicated as a flag.

## Collection-scoped actions

`WithOptionalID()` turns `<id>` into `[id]`. Use it for actions whose target comes entirely from flags:

```go
clicky.Action("sync", func(_ string, flags map[string]string) (SyncReport, error) {
	return store.Sync(flags["source"])
}).WithFlags(SyncFlags{}).WithOptionalID()
```

The route is flat: `POST /api/v1/stack/sync`.

:::caution[Verbs must be unique per entity]
Each action's name becomes a subcommand name. Two actions with the same verb panic at `Register()`, even if their routes would differ. Give a per-ID action a different verb from any collection action, for example `execute` when `run` is taken.
:::

## Primary action

By default the bare entity command (`app stack`) runs `list`. A **primary action** replaces that with a typed collection action. It is served as `POST` on the collection path, next to the list's `GET`:

```go
type DeployOpts struct {
	Env   string `flag:"env" required:"true"`
	DryRun bool  `flag:"dry-run"`
}

func (DeployOpts) ClickyActionFlags() {}

clicky.NewEntity[Deployment, DeploymentListOpts, Deployment]("deploy").
	WithPrimaryAction(entity.PrimaryActionWithContext(DeployOpts{},
		func(ctx context.Context, opts DeployOpts) (DeployResult, error) {
			return deployer.Run(ctx, opts)
		})).
	List(store.ListDeployments).
	Register()
```

```bash
app deploy --env prod --dry-run          # runs the primary action
app deploy list                          # list is still available
curl -X POST localhost:8080/api/v1/deploy -d '{"env":"prod"}'
curl localhost:8080/api/v1/deploy        # list
```

`PrimaryActionWithContext` is in the `entity` package. It is an action named `run` with `WithOptionalID()` and `WithMethod("POST")` already applied.
