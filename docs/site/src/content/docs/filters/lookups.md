---
title: Lookups
description: The lookup request that powers the filter bar, and its wire format.
---

A **lookup** asks every filter on an operation for its option set and for the labels of the current selection. The operation's handler does not run. clicky-ui sends a lookup whenever the filter bar opens or a selection changes.

## Requesting a lookup

Send the operation's normal request with `__lookup=filters` added, or send it as a `HEAD` request:

```bash
curl 'localhost:8080/api/v1/stack?team=platform&status=healthy&__lookup=filters'
curl -X HEAD 'localhost:8080/api/v1/stack?team=platform'
```

The current selections are passed as ordinary parameters. They are decoded into `ListOpts` and normalized by each filter's `Lookup`, so every filter's `Options` sees the full, canonical selection. This is what makes [cascading](/filters/overview/#cascading) work.

These operations support lookups:

- entity `list` and the bare entity route
- filter-mode bulk actions (`POST /api/v1/<entity>/{id}/<action>?__lookup=filters`), which reuse the entity's filters
- `TypedActionWithContext` actions whose options implement `Filterable`
- commands registered with `AddNamedCommand` whose options implement `Filterable`
- dynamic entities with filters

## Response

```json
{
  "filters": {
    "team": {
      "label": "Team",
      "options": {
        "team/platform": { "kind": "text", "text": "Platform", "plain": "Platform" },
        "team/core":     { "kind": "text", "text": "Core",     "plain": "Core" }
      },
      "selected": {
        "team/platform": { "kind": "text", "text": "Platform", "plain": "Platform",
                           "style": { "className": "font-semibold" } }
      }
    },
    "owner": {
      "label": "Owner",
      "multi": true,
      "options": { "u-1": { "kind": "text", "text": "Alice", "plain": "Alice" } },
      "counts":  { "u-1": 42 },
      "truncated": true,
      "total": 3187
    }
  }
}
```

| Field | Meaning |
| --- | --- |
| `label` | control label (`Filter.Label()`) |
| `options` | value → rendered label. Labels are clicky text nodes (`kind`, `text`, `plain`, `style.className`, `children`, `tooltip`) |
| `selected` | labels for the current selection, from `Filter.Lookup` or `FilterSource.Resolve` |
| `counts` | optional rows-per-option, keyed like `options`. An option with no count is absent, not `0` |
| `multi` | the field accepts several values |
| `type`, `unit`, `defaultOperator` | control metadata. See [Control types](/filters/control-types/) |
| `timeEnabled` | offer a clock on a range control (set only when relevant) |
| `truncated`, `total` | only on searchable filters: more options exist than were returned |

clicky rejects inconsistent counts instead of forwarding them. A count for a value that is not among the options, or a negative count, fails the lookup with an error that names the filter.

## Targeted search

To search one searchable filter as the user types, add:

| Param | Meaning |
| --- | --- |
| `__lookup_filter` | key of the filter to search |
| `__lookup_q` | search text |

```bash
curl 'localhost:8080/api/v1/stack?__lookup=filters&__lookup_filter=owner&__lookup_q=ali'
```

A targeted search answers **only** the named filter. Re-enumerating every other filter on each keystroke would cost one backend round trip per filter for data the client throws away. See [Searchable filters](/filters/searchable/).

The `__lookup*` parameters are removed before `ListOpts` is decoded, so they never reach your filters.

## Shell completion

The same filters drive cobra completion on the CLI. Completing `--team` builds `ListOpts` from the other flags typed so far, runs every filter's `Lookup`, calls `teamFilter.Options(opts)`, and returns the keys that match the typed prefix. Each key's label is shown as its description.

## OpenAPI

The response contains only filters that are registered on the operation. `ListOpts` fields without a filter still appear as query parameters, but they have no lookup entry.

In the generated OpenAPI document, each query parameter of a list operation carries an `x-clicky.role` so the UI can route it to the right widget:

| Parameter name | Role |
| --- | --- |
| `limit`, `offset` | pager |
| `sort`, `order` | column sorting |
| `from` / `since`, `to` / `until` | time-range picker |
| any other query parameter, when the operation has filters | `filter` (a filter-bar chip) |

clicky-ui also understands the roles `search` and `cursor`, which clicky's generator does not emit. See the [x-clicky reference](/reference/x-clicky/#parameter-x-clicky-role).

Parameters backed by a [named filter](/filters/named-filters/) also carry `x-clicky-lookup`. It holds a `$ref` to the shared definition under `components.x-clicky-filters`, the lookup `url`, the `filter` key to send as `__lookup_filter`, and `searchParam: "__lookup_q"`.
