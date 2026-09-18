---
title: Searchable filters
description: Large option sets with a capped head, a true total, and server-side search.
---

`Options` returns every value at once. That is fine for a handful of statuses. It is not fine for a column with thousands of distinct owners. A **searchable** filter returns a capped *head* set plus the true total, and answers search queries on the server.

```go
type SearchableFilter[ListOpts any] interface {
	OptionsWithQuery(opts ListOpts, query string, limit int) (options map[string]api.Textable, total int)
}
```

- `query == ""`: return the first `limit` options (the **head**) and `total` = the true number of distinct values. The UI shows the head plus "… and N more".
- `query != ""`: return up to `limit` options that match the query, so values that sort after the head can still be found.

```go
type ownerFilter struct{ store *Store }

func (ownerFilter) Key() string   { return "owner" }
func (ownerFilter) Label() string { return "Owner" }
func (f ownerFilter) Lookup(o *StackListOpts) (map[string]api.Textable, error) {
	return f.store.OwnerLabels(o.Owner) // label the selected owner(s)
}
func (f ownerFilter) Options(o StackListOpts) map[string]api.Textable {
	head, _ := f.OptionsWithQuery(o, "", entity.MaxLookupOptions)
	return head
}

func (f ownerFilter) OptionsWithQuery(o StackListOpts, q string, limit int) (map[string]api.Textable, int) {
	// SELECT DISTINCT owner FROM stacks WHERE owner ILIKE $1 ORDER BY owner LIMIT $2
	owners, total := f.store.DistinctOwners(q, limit)
	options := make(map[string]api.Textable, len(owners))
	for _, owner := range owners {
		options[owner.ID] = clicky.Text(owner.Name)
	}
	return options, total
}
```

:::danger[Bind the query]
`query` is user input that reaches your data source directly. Always pass it as a bound parameter, never by string concatenation into SQL.
:::

## Limits

- The package ceiling is `entity.MaxLookupOptions` (200). This is the most any filter can return for a head or a search.
- Implement `entity.LimitedFilter` (`LookupLimit() int`) to ask for a smaller cap. A value of `0` or anything at or above the ceiling uses the ceiling. Choose a small cap for high-cardinality fields where scrolling a head set is pointless and users should type instead.
- A **non-searchable** filter is enumerated in full, with no cap and no `total`/`truncated`. It is complete by construction.

## Response behaviour

For a searchable filter, the lookup entry carries `total` and `truncated = total > len(options)`. This applies to search results as well as the head, because a search can also overflow its cap. Without it, a clipped result would look like the complete answer.

A [targeted search](/filters/lookups/#targeted-search) (`__lookup_filter=owner&__lookup_q=ali`) returns only that filter's entry.

## Context-aware variants

If the options come from request-scoped state, such as a per-tenant database handle carried on the context, implement the context forms. They are preferred whenever a context is available, which is always true on HTTP and CLI requests:

```go
func (ownerFilter) OptionsWithContext(ctx context.Context, o StackListOpts) map[string]api.Textable
func (ownerFilter) OptionsWithQueryAndContext(ctx context.Context, o StackListOpts, q string, limit int) (map[string]api.Textable, int)
```

The resolution order is `ContextSearchableFilter` → `SearchableFilter` → `ContextFilter` → `Options`.

:::note
Shell completion calls plain `Options(opts)`. It does not use the context or search forms, so `Options` should still return something sensible, such as the head set.
:::
