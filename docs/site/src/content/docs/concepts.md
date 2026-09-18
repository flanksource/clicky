---
title: Core concepts
description: How entities, operations, filters and surfaces fit together.
---

clicky keeps **one registry** of entities and operations. The CLI, the HTTP executor, the OpenAPI document, the clicky-ui explorer and the MCP server are all projections of that registry. No surface has its own copy of the handlers.

```
 NewEntity(...).Register()           AddCommand / RegisterSubCommand
            │                                     │
            ▼                                     ▼
     entity registry  ───── GenerateCLI ────▶  cobra command tree
                                                  │
                     ┌────────────────────────────┼─────────────────────────┐
                     ▼                            ▼                         ▼
                CLI (cobra)             rpc: OpenAPI + /api/v1         mcp: tools
                                          │
                                          ▼
                                   clicky-ui explorer
```

## Vocabulary

| Term | Meaning |
| --- | --- |
| **Entity** | A named resource type such as `widgets`. It is registered with `NewEntity[T, ListOpts, R]` or `RegisterEntity(Entity{...})`. |
| **Operation** | One verb on an entity: `list`, `get`, `create`, `update`, `delete`, a custom **action**, or a **bulk action**. Each becomes one cobra command and one HTTP route. |
| **ListOpts** | The struct the list operation's flags bind to. Its `flag:` tags define CLI flags, OpenAPI query parameters and UI filter controls. |
| **Filter** | An object attached to an entity that labels and normalizes one `ListOpts` field. It also enumerates the options for that field (dropdowns and completion). |
| **Lookup** | The request that asks every filter for its options and selected labels: `?__lookup=filters`, or `HEAD`. |
| **Named filter** | A reusable filter registered once by name and attached to many entities with `entity.Use`. |
| **Flag map** | The transport-neutral `map[string]string` that every surface produces. The CLI builds it from visited flags and HTTP builds it from query and body. Handlers get it decoded into their typed struct. |
| **Surface** | Where an invocation came from: `cli`, `http` or `mcp`. Read it with `entity.OperationSurfaceFromContext(ctx)`. |

## The flag map is the contract

Every transport reduces a call to `(ctx, flags map[string]string, args []string)`:

- **CLI**: the visited cobra flags plus positional args. `get <id>` puts the ID in `args[0]`.
- **HTTP**: query parameters, JSON body fields and path parameters. `{id}` arrives as the `id` flag.
- **MCP**: tool arguments.

clicky then decodes the map into your typed struct with the same `flag:` tags the CLI used (`clicky.BuildOpts[T]`). A handler therefore never has to know which transport called it.

## Packages

- `github.com/flanksource/clicky` re-exports the entity API: `NewEntity`, `Entity`, `Filter`, `Action`, `BulkAction`, `GenerateCLI` and more. Most applications import only this package.
- `github.com/flanksource/clicky/entity` holds the canonical implementation. It also has APIs that the root package does not re-export: named filters (`RegisterFilter`, `Use`), filter sources, dynamic entities, operation listeners, `PrimaryActionWithContext` and `StatusError`.
- `github.com/flanksource/clicky/rpc` is the HTTP executor and OpenAPI generator. It also provides `rpc.RequestFromContext`.
