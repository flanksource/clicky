---
title: Sorting & paging
description: Validated server-side sort keys and paged list responses.
---

## Sorting

Server-side sorting is opt-in. It takes three things:

1. **The row type declares public sort keys** with `sort:"key"` tags, or with `SortKey` on the columns it returns from `api.TableProvider`.
2. **`ListOpts` embeds `clicky.SortOptions`**, which implements `SortCarrier`.
3. **The entity declares a default** with `Sort(SortSpec{...})`.

```go
type Stack struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"    sort:"name"`
	Updated time.Time `json:"updated" sort:"updated" pretty:"label=Updated"`
}

type StackListOpts struct {
	clicky.SortOptions
	Team string `flag:"team"`
}

clicky.NewEntity[Stack, StackListOpts, Stack]("stack").
	Sort(clicky.SortSpec{Default: clicky.SortOptions{Key: "updated", Direction: clicky.SortDirectionDesc}}).
	List(func(opts StackListOpts) ([]Stack, error) {
		// opts.SortOptions.Key is one of "name" | "updated"; Direction is "asc" | "desc".
		return store.List(opts.Team, opts.SortOptions)
	})
```

This adds `--sort` and `--order` flags, and the matching `sort`/`order` query parameters, with their allowed values:

```bash
app stack --sort name --order asc
curl 'localhost:8080/api/v1/stack?sort=name&order=desc'
```

Validation:

- An unknown key, an unknown direction, or `order` without `sort` returns **400** with code `invalid_sort`.
- If neither is sent, the handler receives `SortSpec.Default`.
- `sort` without `order` means `asc`.
- Misconfiguration panics at startup. `Register()` panics on a missing `SortCarrier`, a row type without sort keys, or a default key that is not declared. `GenerateCLI` panics if `ListOpts` also declares `sort` or `order` flags.

Sort keys are the public names from the response metadata, never column or SQL names. Map them to your query yourself.

## Paging

Return `clicky.PagedResult[T]` from `ListPaged` or `ListPagedWithContext` when the list knows its total row count:

```go
type StackListOpts struct {
	Team   string `flag:"team"`
	Limit  int    `flag:"limit"  default:"50"`
	Offset int    `flag:"offset"`
}

clicky.NewEntity[Stack, StackListOpts, Stack]("stack").
	ListPagedWithContext(func(ctx context.Context, opts StackListOpts) (clicky.PagedResult[Stack], error) {
		rows, total, err := store.Page(ctx, opts)
		if err != nil {
			return clicky.PagedResult[Stack]{}, err
		}
		return clicky.NewPagedResult(rows, opts.Limit, opts.Offset, total), nil
	})
```

Over HTTP the response is `{"data": [...], "page": {"limit": 50, "offset": 0, "total": 1234}}`. On the CLI only the rows are rendered. `NewPagedResult` never returns a `null` data array and clamps a negative offset to 0.

The paging window is part of your `ListOpts`, so you choose its flag names. clicky-ui recognises `limit` and `offset` query parameters and binds them to the table pager.

For cursor-based infinite scroll, live tail and exports, see [Paging, infinite scroll & live tail](/entities/long-results/).

## Tabular export

Operations that set an `RPCOperation.PagedFunc` (`entity.PagedFunc`) hand response ownership to clicky. clicky then handles content negotiation, `scope=page|all`, streaming, and download headers for CSV/JSON/XLSX exports. This is a lower-level seam than `ListPaged` and is mostly used by [dynamic entity families](/dynamic/families/).
