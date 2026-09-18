---
title: HTTP routes
description: The command → route mapping for every generated operation.
---

These routes come from running a sample entity through `rpc.NewConverter(rpc.DefaultConfig()).ConvertCommandTree(root)` after `GenerateCLI`. The prefix is `/api/v1`.

| Operation | CLI | Route |
| --- | --- | --- |
| list | `app widgets [flags]` · `app widgets list` | `GET /api/v1/widgets` |
| get | `app widgets get <id>` | `GET /api/v1/widgets/{id}` |
| create | `app widgets create k=v…` | `POST /api/v1/widgets` |
| update | `app widgets update <id> k=v…` | `PUT /api/v1/widgets` (`id` in the body) |
| delete | `app widgets delete <id>` | `DELETE /api/v1/widgets/{id}` |
| action | `app widgets restart <id>` | `POST /api/v1/widgets/{id}/restart` |
| collection action (`WithOptionalID`) | `app widgets sync [id]` | `POST /api/v1/widgets/sync` |
| primary action | `app deploy [flags]` | `POST /api/v1/deploy` (list stays `GET /api/v1/deploy`) |
| bulk action | `app widgets pause <id> [id...]` | `POST /api/v1/widgets/{id}/pause` (`{id}` = comma-joined IDs) |
| bulk action named `delete` | shares its CLI name with the entity's own `delete`; prefer a distinct verb such as `purge` | `DELETE /api/v1/widgets/{id}/delete` |
| admin list / get | `app admin widgets [get <id>]` | `GET /api/v1/admin/widgets[/{id}]` |
| nested (`Parent("catalog")`) | `app catalog cluster` | `GET /api/v1/catalog/cluster` |
| custom command via `RegisterSubCommandFn("widgets", …)` | `app widgets summary` | `POST /api/v1/widgets/summary` (method inferred from the verb) |
| family instance | — | `/api/v1/{family}/{name}` |

## Method inference

CRUD verbs use fixed methods. For actions and commands, the method is inferred from the last word of the command path:

| Verb | Method |
| --- | --- |
| `get`, `list`, `show`, `describe` | GET |
| `create`, `add`, `new` | POST |
| `update`, `edit`, `modify`, `set` | PUT |
| `delete`, `remove`, `destroy` | DELETE |
| anything else | POST |

Override it with `.WithMethod(...)` on an action.

## Query conventions

| Parameter | Meaning |
| --- | --- |
| `format` / `Accept` | response format (`json`, `yaml`, `csv`, `markdown`, `html`, …) |
| `__lookup=filters` or a `HEAD` request | return [filter metadata](/filters/lookups/) instead of running the operation |
| `__lookup_filter`, `__lookup_q` | targeted [server-side search](/filters/searchable/) for one filter |
| `sort`, `order` | [sorting](/entities/sorting-and-paging/) when the entity declares `Sort` |
| `filter` | switch a bulk action into [filter mode](/entities/bulk-actions/) |

Parameters arrive from the query string, the JSON body and path segments, and are merged into one flag map. Nested JSON bodies are flattened to strings for the flag map. Read the raw body with `rpc.RequestFromContext` (see [Request context](/runtime/context/)).
