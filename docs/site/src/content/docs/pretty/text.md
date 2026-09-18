---
title: Text
description: Compose styled, format-independent text with api.Text, Tailwind classes, icons and humanized values.
---

`api.Text` is the building block of clicky output. It is a tree of styled content that renders to ANSI in the terminal, `<span>`s in HTML and emphasis in Markdown. Build it once and let the formatter decide the output.

```go
import (
	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/icons"
)

func (c Check) Pretty() api.Text {
	icon, style := icons.Success, "text-green-600"
	if !c.Passed {
		icon, style = icons.Error, "text-red-600"
	}
	return api.Text{}.
		Add(icon).Space().
		Append(c.Name, "font-bold "+style).Space().
		Append(c.Duration, "text-gray-500")
}
```

```text
✗ test 42.00s          ← terminal (colored)
✗ **test** 42.00s      ← markdown
```

## Building text

| Intent | API |
| --- | --- |
| Start with content | `clicky.Text("healthy", "text-green-600 font-bold")` |
| Start empty | `api.Text{}` then chain |
| Formatted content | `clicky.Textf("%d pods", n)` |
| Append a value (string, time, duration, number, map, `Pretty`, `Textable`) | `.Append(v, styles...)` |
| Append a `Textable` as-is | `.Add(child)` |
| Append a plain string | `.AddText(s, styles...)` |
| Append formatted | `.Appendf("%s/%s", ns, name)` |
| Spacing / layout | `.Space()`, `.Tab()`, `.NewLine()`, `.HR()`, `.Indent(n)` |
| Prefix / suffix / wrap | `.Prefix("[")`, `.Suffix("]")`, `.Wrap("(", ")")` |
| Style the whole node | `.Styles("text-red-600")`, `.AppendStyle("underline")` |
| Tooltip (HTML) | `.WithTooltip(clicky.Text("details"))` |
| Conditional class | `clicky.Class(ok, "text-green-600", "text-red-600")` |

`Append` is type-aware. Strings with newlines become line breaks, `time.Time` and `time.Duration` are humanized, maps become key/value lists, and values that implement `Pretty` contribute their `Pretty()`.

`api.Text` is immutable. Every method returns a new value, so always use the result:

```go
t := clicky.Text("status:")
t = t.Space().Append("ok", "text-green-600") // not: t.Space().Append(...) on its own
```

## Styles

Styles are **Tailwind-like class strings**, resolved against the active theme and the terminal's capabilities.

| Kind | Examples |
| --- | --- |
| Colors | `text-red-600`, `text-green-600`, `text-gray-500`, `bg-blue-50`, `bg-green-100 text-green-800` |
| Weight / decoration | `font-bold`, `italic`, `underline`, `line-through` |
| Width | `max-w-[40ch] truncate` (truncate long values) |

Tailwind colour classes render as true-colour ANSI in the terminal and as inline `style="color: …"` in HTML.

:::caution[Semantic class names]
The names `success`, `error`, `warning`, `info` and `muted` exist in the theme palette. Used directly as a text class, however, they currently render **no colour** in the terminal and only a bare `class="success"` in HTML. Use explicit colour classes such as `text-green-600` and `text-red-600` for status.
:::

## Icons

```go
api.Text{}.Add(icons.Warning).Space().Append("Degraded", "text-amber-600")
```

Add icons as values with `.Add(icon)`, not as strings, so they keep their style. The terminal shows a Unicode glyph and HTML shows an Iconify icon. Common icons are `Success`/`Pass`/`Check`, `Error`/`Fail`/`Cross`, `Warning`, `Info`, `Pending`, `Unknown` and `Skip`. The full catalog is in `api/icons`, and `icons.Filename(path)` picks an icon by file extension.

## Human formatting

Pass typed values and let clicky format them:

```go
clicky.Human(42 * time.Second)       // 42.00s
clicky.Human(1200 * time.Millisecond) // 1200ms
clicky.Human(time.Now())             // 2026-09-18 08:10:00 (UTC) / RFC3339 otherwise
clicky.Human(true)                   // ✓   (false → ✗)
clicky.Human(1234567)                // humanized number
api.HumanNumber(52_000)              // 52K
api.HumanizeBytes(size)              // human-readable size
api.TimeAgo(&created)                // " 3h" (compact age: s / m / h / d)
```

Durations under 5s are shown in ms, then in seconds, minutes and hours, and anything of a day or more is written out. Zero and empty values render as nothing, so leave them out or render an explicit placeholder.

## Links and badges

```go
clicky.Link("/orders/ORD-1").Append("ORD-1")                // <a> in HTML, [ORD-1](…) in Markdown, the label in the terminal
clicky.Link("/orders/ORD-1").Append("ORD-1").WithTarget(clicky.LinkTargetDialog) // UI opens it in a dialog
clicky.LinkCommand("kubectl get pods").Append("pods")       // command link for UIs
clicky.Badge("active", "bg-green-100 text-green-800")       // pill
clicky.LabelBadge("env", "prod", clicky.LabelBadgeColor("bg-blue-100"))
```

Link targets (`LinkTargetDialog`, `LinkTargetHover`, `LinkTargetExpand`, `LinkTargetClicky`, `LinkTargetSelf`, `LinkTargetWindow`, `LinkTargetTab`) tell clicky-ui how to open the link.

`clicky.Button(label, href)` and `clicky.ButtonGroup(...)` render platform-neutral actions. They are buttons in HTML, links in Markdown and plain labels in the terminal.

## Don'ts

- Don't call `.ANSI()`, `.HTML()`, `.Markdown()` or `.String()` inside a render method. Return the `api.Text`.
- Don't embed ANSI escapes or HTML in strings. Use styles.
- Don't build text with `api.Text{Content: …, Children: …}` literals. Use `clicky.Text` or `.Append` chains. `clicky lint` warns about this.
- A function that returns `api.Text` should be named `Pretty`, `PrettyFull` or `PrettyRow`. Other helpers should return `api.Textable`.
