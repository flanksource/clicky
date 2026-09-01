package api

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api/icons"
)

// Severity classifies an Admonition callout. The zero value is SeverityNote.
type Severity int

const (
	SeverityNote Severity = iota
	SeverityInfo
	SeverityTip
	SeverityWarning
	SeverityDanger
)

// String returns the canonical lower-case severity keyword, used both as the
// admonition header word and the HTML class suffix.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityTip:
		return "tip"
	case SeverityWarning:
		return "warning"
	case SeverityDanger:
		return "danger"
	default:
		return "note"
	}
}

// Icon returns the status icon conventionally paired with the severity.
func (s Severity) Icon() icons.Icon {
	switch s {
	case SeverityInfo:
		return icons.Info
	case SeverityTip:
		return icons.Success
	case SeverityWarning:
		return icons.Warning
	case SeverityDanger:
		return icons.Error
	default:
		return icons.Info
	}
}

// ParseSeverity maps an author-supplied severity word to a Severity. Unknown
// words fall through to SeverityNote so callers preserve the raw text via the
// admonition body rather than failing.
func ParseSeverity(s string) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "info", "information", "note-info":
		return SeverityInfo
	case "tip", "success", "hint":
		return SeverityTip
	case "warning", "warn", "caution":
		return SeverityWarning
	case "danger", "error", "critical", "failure":
		return SeverityDanger
	default:
		return SeverityNote
	}
}

// Admonition is a callout block (`!!! <severity> <title>`) with an indented
// body. Title and Body are each a single Textable; Title may be nil (header
// shows the severity word only) and Body may be nil (header only).
type Admonition struct {
	Severity Severity
	Title    Textable
	Body     Textable
}

// header returns the plain `!!! <severity> <title-text>` line used by the
// text-oriented renderers.
func (a Admonition) header() string {
	h := "!!! " + a.Severity.String()
	if a.Title != nil {
		if title := a.Title.String(); title != "" {
			h += " " + title
		}
	}
	return h
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func (a Admonition) String() string {
	if a.Body == nil {
		return a.header()
	}
	return a.header() + "\n" + indentLines(a.Body.String(), "    ")
}

func (a Admonition) ANSI() string {
	head := Text{Content: a.header(), Style: "font-bold"}.ANSI()
	if a.Body == nil {
		return head
	}
	return head + "\n" + indentLines(a.Body.ANSI(), "    ")
}

func (a Admonition) HTML() string {
	title := ""
	if a.Title != nil {
		title = a.Title.HTML()
	}
	body := ""
	if a.Body != nil {
		body = a.Body.HTML()
	}
	return fmt.Sprintf("<div class=%q><p>%s</p>%s</div>",
		"admonition admonition-"+a.Severity.String(), title, body)
}

func (a Admonition) Markdown() string {
	return a.MarkdownWithOptions(MarkdownOptions{})
}

func (a Admonition) MarkdownWithOptions(options MarkdownOptions) string {
	if a.Body == nil {
		return a.header()
	}
	return a.header() + "\n" + indentLines(RenderMarkdown(a.Body, options), "    ")
}
