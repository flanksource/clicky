---
title: LLM list output
description: Read lists progressively with full Markdown cells and at most 25 rows.
---

Use `--llm` or `--format llm` on a Clicky command to render a list as a Markdown table with at most 25 rows. Cell text and entity links remain complete, and row details are omitted. Ordinary `--markdown` output keeps every returned row and also preserves full cell text.

```bash
app records list --llm --offset 0 --limit 25
app records list --format llm --offset 25 --limit 25
app records list --json --offset 0 --limit 25
app records list --format llm,json=.tmp/records.json
```

The footer reports the displayed window, total count (or `unknown`), and whether more rows exist. Exact totals and estimates are distinguished. A paged list provides continuation instructions; retain its filters, sort and entity scope when requesting the next page. Cursor pages describe the current page rather than inventing a global row number. Lists without paging say so instead of offering unsupported flags.

```text
Shown: 25 (rows 1–25); Total: 68; Has more: true.
Next: --offset 25 --limit 25. Keep the same filters, sort and entity scope.
Use --json for complete fields and all returned rows.
```

The footer also references `--json` for complete fields and all rows returned by the operation. Replace `--llm` with `--json` to read that structured output. This changes the output format; use the operation's normal paging or export options to fetch more rows from a paged backend.

CLI stdout and file sinks use their own formats. The JSON sink in the example receives all rows returned by the operation, while stdout displays up to 25.

## Provider integration

Return `clicky.PagedResult[T]` from buffered paged lists so the renderer can use their limit, offset and total. A negative total means the count is unknown. Return a plain slice for a list with no paging contract; its preview does not promise a continuation.

Native `entity.PagedFunc` operations receive an LLM request limit of at most 25 before querying. Their `PageResponse` supplies paging availability, an optional exact or estimated total, `HasMore`, and an optional next cursor. HTTP responses include a next-page link retaining the existing query parameters. `scope=all&format=llm` is rejected: use a full export format for all rows.

Aichat list tools and the standalone MCP server default to this same format. Detail and write operations retain their existing response shapes. Aichat's tool catalog declares list responses as Markdown strings. The standalone MCP server can override its default through its existing format configuration.

The 25-row preview is a presentation limit. Clicky does not impose a token budget or save oversized responses to files; those policies belong to the agent runtime. Long cells remain complete even when they exceed a model's response budget.
