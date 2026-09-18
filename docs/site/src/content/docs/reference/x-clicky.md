---
title: x-clicky annotations
description: Every x-clicky OpenAPI / JSON-Schema extension, who emits it, and what clicky-ui does with it.
---

clicky publishes UI semantics as vendor extensions in its OpenAPI document. clicky-ui (`@flanksource/clicky-ui`) reads them to build navigation, tables, filter bars, action buttons, forms and AI tools. This page lists each extension, where it sits in the document, what emits it, and what the UI does with it.

**Emitted by** says whether clicky's Go generator produces the extension or another producer has to set it. The UI treats both the same way.

## At a glance

| Extension | Location | Emitted by clicky | Read by clicky-ui |
| --- | --- | --- | --- |
| [`x-clicky`](#document-x-clicky-surfaces) | document root | ✓ `surfaces[]` | ✓ |
| [`x-clicky`](#operation-x-clicky) | operation | ✓ | ✓ |
| [`x-clicky`](#parameter-x-clicky-role) | parameter | ✓ (`role`) | ✓ |
| [`x-clicky-lookup`](#x-clicky-lookup-parameter) | parameter | ✓ (named filters) | `$ref` only |
| [`x-clicky-filters`](#x-clicky-filters-components) | `components` | ✓ | ✓ |
| [`x-clicky-placeholder`](#x-clicky-placeholder) | parameter | — | ✓ |
| [`x-clicky-lookup`](#x-clicky-lookup-schema-property) | JSON-Schema property | — | ✓ |
| [`x-clicky-component`](#x-clicky-component) | JSON-Schema property | ✓ (`clicky:"type=…"`) | ✓ |
| [`x-clicky-order`](#x-clicky-order) | JSON-Schema property | ✓ (`clicky:"order=…"`) | ✓ |
| [`x-clicky-default-source`](#x-clicky-default-source) | JSON-Schema property | ✓ (`clicky:"source=…"`) | ✓ |
| [`x-clicky-unit`](#other-form-extensions) | JSON-Schema property | — | ✓ |
| [`x-clicky-cel-scope`, `x-clicky-presets`, `x-clicky-section`](#other-form-extensions) | JSON-Schema property | — | ✓ |
| [`x-clicky-property`](#emitted-but-not-read) | JSON-Schema property | ✓ (`clicky:"property=…"`) | — |

## Document: x-clicky surfaces

```json
{
  "x-clicky": {
    "surfaces": [
      { "key": "entity:cluster:parent:catalog", "entity": "cluster", "title": "Cluster", "parent": "catalog",
        "icon": "server", "path": "infra/k8s", "description": "…", "admin": false }
    ]
  }
}
```

Each registered entity becomes one **surface**, which is a navigable page in the explorer.

| Field | clicky source | UI effect |
| --- | --- | --- |
| `key` | `entity:<name>[:parent:<parent>][:admin]` | surface identity; operations reference it in `x-clicky.surface`, and detail routes are `/<key>/<id>` |
| `entity` | entity name | entity identity for the explorer and chat tools |
| `title` | prettified name, or `x-clicky-title` (dynamic) | sidebar label and page title |
| `description` | — | page description (defaults to "Manage X resources.") |
| `parent` | `Parent(...)` | sidebar section; surfaces without one go under "Other" |
| `icon` | `x-clicky-icon` (dynamic entities) | sidebar / picker icon |
| `path` | `Path(...)` / `x-clicky-path` | nested groups inside the parent section; flat when no surface sets one |
| `admin` | `Admin(...)` | not chosen as the default landing surface |

## Operation: x-clicky

```json
"get": {
  "x-clicky": {
    "surface": "entity:stack", "command": "stack/restart", "verb": "action", "scope": "entity",
    "actionName": "restart", "idParam": "id", "supportsLookup": true, "supportsFilterMode": false,
    "group": "infrastructure",
    "toolHints": { "title": "Restart stack", "destructiveHint": true, "icon": "refresh", "defaultPermission": "ask" },
    "schedule": { "suggestions": [{ "label": "Nightly", "cron": "0 2 * * *" }] },
    "export": { "formats": ["csv", "json"], "scopes": ["page", "all"], "allRowsMode": "streaming",
                "formatMaxRows": { "pdf": 1000 } }
  }
}
```

| Field | UI effect |
| --- | --- |
| `surface` | attaches the operation to a surface |
| `verb` + `scope` | its role on the surface: `list`+`collection` is the table, `get`+`entity` is the detail view, any other `collection` verb is a toolbar action, any other `entity` verb is a row/detail action |
| `actionName` | button label; key for host label and initial-value overrides |
| `idParam` | the ID parameter used when attaching a row as context (falls back to the first path parameter, then `id`) |
| `supportsLookup` | an action form prefetches `?__lookup=filters` |
| `supportsFilterMode` | the collection action becomes a **selection** action for [bulk actions](/entities/bulk-actions/). It runs against the current filters plus the selected row count |
| `command` | the CLI command shown or copied for the operation |
| `group`, `toolHints.*` | AI tool metadata ([AI tools](/runtime/ai-tools/)). `toolHints.icon` is the button icon, and `destructiveHint` gives a destructive button style plus a confirmation |
| `schedule.suggestions[]` | makes the operation schedulable, with suggested crons (the first is the default) |
| `export.*` | the download menu: available `formats`, `scopes` (`page`/`all`), streaming vs buffered notes, per-format row caps. See [Export](/entities/long-results/#export-scopepageall) |

`schedule.timeout` is emitted in nanoseconds and is not read by the UI. Entity, parent, icon, path and title are not operation fields. They reach the UI only through `surfaces[]`.

## Parameter: x-clicky role

```json
{ "name": "offset", "in": "query", "x-clicky": { "role": "offset" } }
```

| `role` | UI effect | Emitted by clicky for |
| --- | --- | --- |
| `filter` | a filter-bar chip | any other query parameter of a list that has filters |
| `search` | the filter bar's search box | — (set by other producers) |
| `limit` | page size; the pager appears only when this exists | `limit` |
| `offset` | page position; reset when filters change | `offset` |
| `cursor` | opaque position; its presence switches the table to [infinite scroll](/entities/long-results/#infinite-scroll-cursor-walk) | — (set by other producers) |
| `sort`, `order` | server-side sort from column headers (both required) | `sort`, `order` |
| `time-from`, `time-to` | the edges of a time-range picker | `from`/`since`, `to`/`until` |

Parameters with the `limit`, `offset`, `cursor`, `sort` or `order` role are never sent with lookup or follow requests.

## x-clicky-lookup (parameter)

```json
{ "name": "owner", "in": "query",
  "x-clicky-lookup": { "$ref": "#/components/x-clicky-filters/users", "url": "/api/v1/tasks",
                       "filter": "owner", "searchParam": "__lookup_q", "multi": true } }
```

clicky emits this for parameters backed by a [named filter](/filters/named-filters/). clicky-ui reads only `$ref`, which gives the control its shape. Options are always fetched from the list operation itself with `?__lookup=filters` (`__lookup_filter` / `__lookup_q` for search). See [Lookups](/filters/lookups/).

## x-clicky-filters (components)

```json
"components": { "x-clicky-filters": {
  "users": { "name": "users", "label": "User", "type": "multi-select", "multi": true,
             "source": { "kind": "entity", "entity": "users" } } } }
```

These are the shared filter definitions from `NamedFilter.Spec()`. The UI reads `label`, `type`, `unit`, `defaultOperator` and `multi` so each control has its final shape on first paint, before any lookup returns.

## x-clicky-placeholder

Placeholder text for a parameter's input. It takes precedence over `placeholder`. clicky does not emit it.

## JSON-Schema form extensions

These sit on properties of request-body and form schemas and drive `JsonSchemaForm`. clicky emits some of them from the `clicky:"…"` struct tag on Go fields (`rpc.SchemaForStruct`):

```go
type Connection struct {
	URL      string `json:"url"      clicky:"type=k8s-url-selector,source=value,order=1"`
	Password string `json:"password" clicky:"type=k8s-secret-selector,format=password,order=2"`
	Region   string `json:"region"   clicky:"title=Region,desc=Cloud region,property=region,required"`
}
```

| `clicky:` token | Produces |
| --- | --- |
| `type=<component>` | `x-clicky-component` |
| `order=<n>` | `x-clicky-order` |
| `source=<value\|secret>` | `x-clicky-default-source` |
| `property=<key>` | `x-clicky-property` |
| `title=`, `desc=`, `format=`, `required` | standard `title`, `description`, `format`, `required[]` |

A type can also describe itself by implementing `rpc.SchemaDescriber` (`JSONSchema() map[string]any`). Any `x-clicky-*` keys it returns are passed through unchanged.

### x-clicky-component

Selects a custom form widget. It is resolved by form extensions:

| Value | Widget |
| --- | --- |
| `k8s-secret-selector` | pick a value or a Kubernetes secret reference |
| `k8s-url-selector` | pick a URL or a Kubernetes service/ingress |
| `cel-editor` | CEL expression editor |
| `profile-query-builder`, `es-query-builder`, `es-query-operand` | query builders |
| `processor-pipeline` | processor pipeline editor |

Hosts register more through the explorer's `formExtensions`.

### x-clicky-order

A number. Lower values render first. Properties without one keep document order, and the ordering holds across merged `if`/`then` branches.

### x-clicky-default-source

`value` or `secret`. It sets the initial mode of the secret and URL selectors.

### x-clicky-lookup (schema property)

A searchable lookup field inside a form:

```json
"owner": { "type": "string",
  "x-clicky-lookup": { "url": "/api/v1/tasks", "filter": "owner", "searchParam": "__lookup_q", "multi": false,
                       "scope": { "param": "team", "from": "team" } } }
```

- `url` and `filter` are required. Without them the extension is ignored.
- `multi` selects multi-select. A single select also accepts custom values.
- `scope` narrows options by a sibling field's value.
- `hierarchy.delimiters` renders dotted or slashed values as a tree.

### Other form extensions

| Extension | Effect |
| --- | --- |
| `x-clicky-unit` | `count` or `bytes`: unit-aware formatting plus ×2 / ÷2 stepper buttons |
| `x-clicky-cel-scope` | `row` (default), `batch` or `boundary`: the variable environment for a `cel-editor` |
| `x-clicky-presets` | presets for the `processor-pipeline` editor |
| `x-clicky-section` | groups runtime-spec fields: `model`, `prompt`, `workspace`, `sandbox`, `permissions`, `environment`, `cli`. Any other value is an error |

clicky does not emit these. They come from other producers (e.g. commons-db, captain).

## Emitted but not read

| Extension | Status |
| --- | --- |
| `x-clicky-property` | emitted from `clicky:"property=…"`; available to consumers, not used by clicky-ui |
| `schedule.timeout` | emitted, not read |
| `x-clicky-lookup.url/filter/searchParam/multi` on parameters | emitted; the UI uses only `$ref` |
| `x-clicky-filters.*.name/source` | emitted; the UI does not need them |

## Inputs, not outputs

The [dynamic-entity schema keywords](/dynamic/schema-entities/#schema-keywords) (`x-clicky-id`, `x-clicky-name`, `x-clicky-filter`, `x-clicky-filter-key`, `x-clicky-label`, `x-clicky-format`, `x-clicky-short`, `x-clicky-aliases`, `x-clicky-parent`, `x-clicky-icon`, `x-clicky-path`, `x-clicky-title`) are read by clicky's Go code when you **register** a schema-driven entity. They do not appear in the generated OpenAPI document. Their effect reaches clicky-ui indirectly, as surface fields, parameters and response schemas.
