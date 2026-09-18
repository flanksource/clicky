---
title: Paging, infinite scroll & live tail
description: The three ways a long result set reaches the user, what each needs from the server, and how clicky-ui picks one.
---

A list that can grow large is delivered in one of three modes. They answer different questions, so choose by the data, not by taste:

| Mode | User experience | Answers | Server contract | Selected by |
| --- | --- | --- | --- | --- |
| **Paged** | page footer (1 2 3 … / page size) | "show me page N of a stable, countable set" | `limit` + `offset` params, `X-Total-Count` | default |
| **Infinite scroll** | rows append as you scroll; no page numbers | "keep reading forward through a huge or unstable set" | a `cursor` param, `X-Has-More` + `X-Next-Cursor` | a parameter with `x-clicky.role: cursor`, or the host's `paginationMode="infinite"` |
| **Live tail** | new rows stream in as they happen | "what is happening *now*?" | `POST <list>/sessions` + SSE events | the host's `follow` prop on `OperationCatalog` |

**Export** (download all rows) sits beside all three. It is covered [below](#export-scopepageall).

## Paged: limit / offset

This is the default and the right choice when the backend can count and jump.

```go
type StackListOpts struct {
	Team   string `flag:"team"`
	Limit  int    `flag:"limit"  default:"50"`
	Offset int    `flag:"offset"`
}

clicky.NewEntity[Stack, StackListOpts, Stack]("stack").
	ListPagedWithContext(func(ctx context.Context, o StackListOpts) (clicky.PagedResult[Stack], error) {
		rows, total, err := store.Page(ctx, o)
		return clicky.NewPagedResult(rows, o.Limit, o.Offset, total), err
	})
```

**Server.** A `PagedResult` response carries these headers:

| Header | Value |
| --- | --- |
| `X-Total-Count` | total rows |
| `X-Total-Relation` | `eq` (exact) |
| `X-Page-Limit` | page size |
| `X-Page-Offset` | position |

The OpenAPI generator gives the parameters named `limit` and `offset` the roles `x-clicky.role: limit` and `offset`.

**UI.**
- The page footer appears only when a `limit`-role parameter exists. Page changes need an `offset` role.
- Page size comes from the response's `limit`, then the form value, then 25.
- Changing any filter resets `offset` to 0.
- An exact total gives page numbers. A `gte` total renders as "~N+".

**When not to use it.** Deep offsets on large tables get slower with every page. Rows inserted between requests also shift pages, so users see duplicates or miss rows. Past offset 10,000, clicky-ui prefers cursor stepping when the server offers one.

## Infinite scroll: cursor walk

Use a cursor when there is no cheap total, the data is huge, or rows keep arriving. The server hands back an opaque position, and the next request resumes from it.

**Server contract.**
- **Request:** a `cursor` query parameter carrying the opaque position.
- **Response headers:** `X-Has-More: true|false` and `X-Next-Cursor: <token>`. Both are needed. The UI stops walking when `X-Has-More` is false or no cursor is returned.
- **Offset:** combining `cursor` with `offset` is invalid. `entity.ParsePageRequest` rejects it with `400 invalid_cursor`.
- **Stale cursor:** a cursor that no longer resolves should fail with the error code `cursor_stale`. The UI then drops it and restarts from the first page.

These headers come from clicky's lower-level paged transport. An operation (or a [dynamic family](/dynamic/families/)) supplies an `entity.PagedFunc` that returns an `entity.PageResponse` with `HasMore`, `Next`, `Total` and `Pageable`, and clicky writes the headers:

```go
func(ctx context.Context, req entity.PageRequest, flags map[string]string) (entity.PageResponse, error) {
	rows, next, err := store.Scan(ctx, flags, req.Cursor, req.Limit)
	if err != nil {
		return entity.PageResponse{}, err
	}
	return entity.PageResponse{
		Rows: rows, Mode: entity.ModePage, Pageable: true,
		HasMore: next != "", Next: next,
		Total: &entity.Total{Value: estimate, Exact: false}, // X-Total-Relation: gte
	}, nil
}
```

**How clicky-ui picks walk mode.** Walk mode is on when the host passes `paginationMode="infinite"` to `OperationCatalog`, or when the prop is omitted and a parameter carries `x-clicky.role: cursor`. Forcing `infinite` on an operation without a cursor parameter throws.

In walk mode:
- a sentinel row at the bottom of the table loads the next page when it scrolls into view (200px early)
- rows are appended, not replaced
- page-number controls are hidden
- a refetch restarts the walk

:::caution[Not emitted by clicky's generator yet]
clicky's OpenAPI generator assigns roles by parameter name (`limit`, `offset`, `sort`, `order`, `from`/`since`, `to`/`until`, then `filter`). It does **not** currently emit `role: cursor` or `role: search`. With clicky alone, infinite scroll is therefore enabled by the host (`paginationMode="infinite"`), or by a producer that stamps the `cursor` role itself.
:::

## Live tail: follow sessions

Live tail is for event-like data such as logs, traces and audit streams, where the question is "what's arriving now". It is not a paging strategy. Rows are pushed by the server.

**Opt-in.** The host enables it on the list with `OperationCatalog`'s `follow` prop:

| `follow` | Behaviour |
| --- | --- |
| `true` or `{mode: "append", maxRows}` | new rows are merged into the table, deduplicated by row ID (or a `seq` column), newest first unless the sort is ascending. The total becomes a `gte` lower bound |
| `{mode: "reconcile", params, maxRows, onReconcile}` | each new event re-runs the list query 250ms later, which suits aggregated views |

**Server contract.** clicky-ui checks for these and shows an error if they are missing:

1. The list path contains `/profile/<name>` (e.g. `/api/v1/profile/jvm-trace`).
2. A `POST <listPath>/sessions` operation exists in the spec.
3. `POST {basePath}/profile/{name}/sessions?follow=true&<filters>` starts a session. The filters are the list's filter, search and time parameters, **without** `limit`, `offset`, `cursor`, `sort` or `order`. Return `409` when the session cap is reached.
4. `GET {basePath}/sessions/{id}/events` is a Server-Sent Events stream:
   - `event` frames carry `{sessionId, sequence, time, row | rows, clickyRow, error}`.
   - A `done` frame ends the stream.
   - `sequence` must increase. The browser reconnects with `Last-Event-ID`, and the UI deduplicates by the highest sequence seen.
   - Each row needs a `clickyRow`, the rendered form of the row. A row without one is reported as an error.

The client buffers up to `maxRows` (default 5,000) and counts the rows it evicts as dropped. There is no polling fallback. Without `EventSource` the tail fails loudly.

### Task output is different

The streaming output of clicky tasks (`task.SSEHandler`) is a separate SSE stream for **task progress**, not list rows. It emits changed task snapshots every 200ms, plus stdout/stderr as append-only deltas at absolute offsets (or a reset when the bounded tail rolls over), and ends with `event: done`. clicky-ui's task views consume it and fall back to polling only when `EventSource` is unavailable.

## Export: scope=page|all

Downloading is independent of how the table is being browsed:

| Request | Meaning |
| --- | --- |
| `?format=csv&scope=page&limit=50&offset=100` | exactly the rows on screen |
| `?format=csv&scope=all` | every matching row, up to a ceiling. `limit`, `offset` and `cursor` are ignored and the exact total is skipped, so the stream can start at once |
| `&filename=stacks.csv` / `&_download` | set `Content-Disposition` |

Response facts:
- `X-Export-Mode` is `page`, `buffered` or `streaming`.
- `X-Max-Rows` is the ceiling.
- `X-Truncated: true` means the rows were cut. When that is only known after streaming, it is sent as a trailer (which browsers cannot read).
- A failure mid-stream is noted in the `X-Stream-Error` trailer.

clicky-ui builds its download menu from the operation's `x-clicky.export` metadata (`formats`, `scopes`, `allRowsMode`, `formatMaxRows`). It labels the scopes "Current page" and "All N rows", and removes `limit`/`offset` for `scope=all`. Without `export` metadata, the UI falls back to its legacy format list.

## Choosing

- **Countable, stable, needs page jumps:** use paged (`ListPaged` + `limit`/`offset`).
- **Huge, unordered, or costly to count; reading forward only:** use infinite scroll (a `PagedFunc` with `Next`/`HasMore` plus a `cursor` parameter).
- **Events arriving continuously; the user watches:** use live tail (follow sessions), usually on top of a paged or infinite list for history.
- **The user wants the data elsewhere:** use export `scope=all`, whichever browsing mode is active.
