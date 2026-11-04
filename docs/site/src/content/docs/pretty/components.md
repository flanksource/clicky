---
title: Components
description: Lists, key/value maps, code blocks, collapsible sections, callouts, diffs, stack traces and more.
---

All components are `api.Textable`. They can be returned directly, added to an `api.Text` with `.Add(...)`, or placed in table cells.

## Lists and key/values

```go
clicky.List(clicky.Text("one"), clicky.Text("two"))        // bulleted list in terminal, Markdown and HTML
clicky.CompactList(tags)                                   // short scalar slices; inline up to 3 items
clicky.TextList(a, b)                                      // join existing Textables
clicky.Map(map[string]string{"env": "prod", "team": "platform"}) // env: prod, team: platform (sorted keys)
clicky.KeyValue("namespace", obj.Namespace)                // single pair; empty values are skipped
```

For a sorted block of key/value pairs, use `clicky.KeyMap`. Rich values keep their links, badges, and nested structure in every human-readable format:

```go
clicky.KeyMap(map[string]api.Textable{
	"Owner": clicky.Link("https://example.test/people/alex").Append("Alex"),
	"Status": clicky.LabelBadge("status", "ready"),
	"Instructions": clicky.CodeBlock("sh", "make test"),
})
```

For explicit ordering, use `api.DescriptionList`:

```go
api.DescriptionList{Items: []api.KeyValuePair{
	clicky.KeyValue("namespace", obj.Namespace),
	clicky.KeyValue("owner", obj.Owner),
}}
```

## Code

```go
clicky.CodeBlock("go", source)
clicky.CodeBlock("application/json", body) // MIME types map to languages
```

Code is syntax-highlighted in the terminal and HTML, and fenced in Markdown. Fences grow to contain embedded backticks safely. Lists indent multiline items; `api.List{Ordered: true}` numbers items, and `Unstyled: true` removes markers.

Tables render HTML elements by default, including expanded row details. Set `TextTable.Interactive = true` for JavaScript sorting, search, and collapsible details in an HTML page that loads Clicky's browser assets.

## Collapsible sections

```go
clicky.Collapsed("Response", clicky.CodeBlock("json", body))
```

HTML and Markdown render a `<details>` disclosure. The terminal shows the content directly.

## Callouts

```go
clicky.Admonition(api.SeverityWarning, clicky.Text("Heads up"), clicky.Text("Disk is 91% full"))
```

```text
!!! warning Heads up
    Disk is 91% full
```

The severities are `SeverityNote`, `SeverityInfo`, `SeverityTip`, `SeverityWarning` and `SeverityDanger`. `api.ParseSeverity("warning")` parses one from a string.

## Diffs

```go
clicky.Diff(before, after, "old.yaml", "new.yaml")
```

This renders a unified diff with added and removed lines styled for the terminal and HTML.

## Stack traces

```go
clicky.StackTrace(trace, clicky.WithMaxStackFrames(20), clicky.WithStackExclude("runtime/"))
clicky.StackTraceJava(javaTrace)
```

Stack traces are parsed into frames and can include source context (`WithSourceResolver`, `WithStackContext`).

## Document structure

| Helper | Output |
| --- | --- |
| `clicky.Heading(level, text)` | heading |
| `clicky.Blockquote(text)` | quote |
| `clicky.FootnoteRef(id)`, `clicky.Footnote(id, text)`, `clicky.Footnotes(...)` | footnotes |
| `clicky.Comment("generated")` | HTML/Markdown comment, invisible in the terminal |
| `clicky.HTMLElement("span", "raw", attrs)` | custom HTML with a plain-text fallback |
| `clicky.WithKey("summary", text)` | renders like `text`, serializes to JSON as `{"summary": …}` |

## Buttons and badges

```go
clicky.ButtonGroup(
	clicky.Button("Open", "/orders/1"),
	clicky.Button("Cancel", "/orders/1/cancel", clicky.ButtonVariant("bg-red-600 text-white")), // variant = CSS classes
)
clicky.Badge("active", "bg-green-100 text-green-800")
clicky.LabelBadge("env", "prod", clicky.LabelBadgeColor("bg-blue-100"), clicky.LabelBadgeIcon("server"))
```

## Markdown documents

`clicky.ParseMarkdown(src)` parses Markdown into a document model that renders through the same formatters. Use it to embed authored Markdown in rich output.
