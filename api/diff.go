package api

import (
	"html"
	"strings"

	"github.com/fatih/color"
	"github.com/pmezard/go-difflib/difflib"
)

// Diff renders a unified diff between two strings across the standard
// output formats (plain, ANSI, HTML, Markdown). Empty Before/After are
// allowed; identical inputs render to nothing.
type Diff struct {
	Before    string `json:"before,omitempty"`
	After     string `json:"after,omitempty"`
	FromLabel string `json:"from_label,omitempty"`
	ToLabel   string `json:"to_label,omitempty"`
	Context   int    `json:"context,omitempty"`
}

// NewDiff builds a Diff with sensible defaults (3 lines of context).
func NewDiff(before, after, fromLabel, toLabel string) Diff {
	return Diff{
		Before:    before,
		After:     after,
		FromLabel: fromLabel,
		ToLabel:   toLabel,
		Context:   3,
	}
}

// IsEmpty is true when Before and After are byte-identical.
func (d Diff) IsEmpty() bool { return d.Before == d.After }

// Unified returns the raw unified diff with no color. Empty when IsEmpty.
func (d Diff) Unified() string {
	if d.IsEmpty() {
		return ""
	}
	ctx := d.Context
	if ctx <= 0 {
		ctx = 3
	}
	out, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(d.Before),
		B:        difflib.SplitLines(d.After),
		FromFile: d.FromLabel,
		ToFile:   d.ToLabel,
		Context:  ctx,
	})
	if err != nil {
		return ""
	}
	return out
}

// String returns the plain unified diff (no color).
func (d Diff) String() string { return d.Unified() }

// ANSI returns the unified diff with terminal colors. Honors fatih/color's
// NoColor (NO_COLOR env, non-tty stdout, --no-color flag).
func (d Diff) ANSI() string {
	raw := d.Unified()
	if raw == "" || color.NoColor {
		return raw
	}
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			lines[i] = ansiStyle(line, "1")
		case strings.HasPrefix(line, "@@"):
			lines[i] = ansiStyle(line, "36")
		case strings.HasPrefix(line, "+"):
			lines[i] = ansiStyle(line, "32")
		case strings.HasPrefix(line, "-"):
			lines[i] = ansiStyle(line, "31")
		}
	}
	return strings.Join(lines, "\n")
}

func ansiStyle(s, code string) string {
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// HTML wraps the unified diff in a <pre> block with per-line span classes
// (diff-add / diff-remove / diff-hunk / diff-meta). Callers supply their
// own CSS — see GetDiffCSS for a starter palette.
func (d Diff) HTML() string {
	raw := d.Unified()
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<pre class="diff">`)
	for i, line := range strings.Split(raw, "\n") {
		if i > 0 {
			b.WriteString("\n")
		}
		class := ""
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			class = "diff-meta"
		case strings.HasPrefix(line, "@@"):
			class = "diff-hunk"
		case strings.HasPrefix(line, "+"):
			class = "diff-add"
		case strings.HasPrefix(line, "-"):
			class = "diff-remove"
		}
		if class == "" {
			b.WriteString(html.EscapeString(line))
		} else {
			b.WriteString(`<span class="`)
			b.WriteString(class)
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(line))
			b.WriteString(`</span>`)
		}
	}
	b.WriteString("</pre>")
	return b.String()
}

// Markdown returns the unified diff inside a ```diff fenced code block.
func (d Diff) Markdown() string {
	raw := d.Unified()
	if raw == "" {
		return ""
	}
	return "```diff\n" + strings.TrimRight(raw, "\n") + "\n```"
}

// GetDiffCSS returns a small starter stylesheet matching the class names
// emitted by Diff.HTML. Include alongside GetChromaCSS in HTML pages.
func GetDiffCSS() string {
	return `pre.diff { white-space: pre-wrap; word-break: break-word; overflow-wrap: anywhere; margin: 0.25rem 0; padding: 0.5rem 0.75rem; border-radius: 0.25rem; background: #0d1117; color: #c9d1d9; font-family: ui-monospace, SFMono-Regular, monospace; }
pre.diff .diff-meta { font-weight: bold; color: #c9d1d9; }
pre.diff .diff-hunk { color: #58a6ff; }
pre.diff .diff-add  { color: #3fb950; }
pre.diff .diff-remove { color: #f85149; }
`
}
