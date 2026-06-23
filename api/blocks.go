package api

import (
	"fmt"
	"strings"
)

// Heading is a Markdown-style section heading. Level is clamped to the
// semantic HTML/Markdown range of 1..6 when rendered.
type Heading struct {
	Level   int
	Content Textable
}

func clampHeadingLevel(level int) int {
	if level < 1 {
		return 1
	}
	if level > 6 {
		return 6
	}
	return level
}

func blockTextableString(t Textable) string {
	if t == nil {
		return ""
	}
	return t.String()
}

func blockTextableANSI(t Textable) string {
	if t == nil {
		return ""
	}
	return t.ANSI()
}

func blockTextableHTML(t Textable) string {
	if t == nil {
		return ""
	}
	return t.HTML()
}

func blockTextableMarkdown(t Textable) string {
	if t == nil {
		return ""
	}
	return t.Markdown()
}

func (h Heading) String() string {
	return blockTextableString(h.Content)
}

func (h Heading) ANSI() string {
	if h.Content == nil {
		return ""
	}
	return Text{Content: h.Content.String(), Style: "font-bold"}.ANSI()
}

func (h Heading) HTML() string {
	level := clampHeadingLevel(h.Level)
	return fmt.Sprintf("<h%d>%s</h%d>", level, blockTextableHTML(h.Content), level)
}

func (h Heading) Markdown() string {
	content := strings.TrimSpace(blockTextableMarkdown(h.Content))
	if content == "" {
		return ""
	}
	return strings.Repeat("#", clampHeadingLevel(h.Level)) + " " + content
}

// Blockquote is a quoted document block.
type Blockquote struct {
	Content Textable
}

func quoteLines(s string) string {
	if s == "" {
		return ">"
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = ">"
			continue
		}
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}

func (b Blockquote) String() string {
	return quoteLines(blockTextableString(b.Content))
}

func (b Blockquote) ANSI() string {
	return quoteLines(blockTextableANSI(b.Content))
}

func (b Blockquote) HTML() string {
	return fmt.Sprintf("<blockquote>%s</blockquote>", blockTextableHTML(b.Content))
}

func (b Blockquote) Markdown() string {
	return quoteLines(blockTextableMarkdown(b.Content))
}

// FootnoteRef is an inline reference to a matching footnote definition.
type FootnoteRef struct {
	ID string
}

func normalizedFootnoteID(id string) string {
	return strings.TrimSpace(id)
}

func (r FootnoteRef) String() string {
	id := normalizedFootnoteID(r.ID)
	if id == "" {
		return ""
	}
	return fmt.Sprintf("[^%s]", id)
}

func (r FootnoteRef) ANSI() string {
	return r.String()
}

func (r FootnoteRef) HTML() string {
	id := normalizedFootnoteID(r.ID)
	if id == "" {
		return ""
	}
	escaped := htmlEscapeString(id)
	return fmt.Sprintf(`<sup id="fnref-%s"><a href="#fn-%s">[^%s]</a></sup>`, escaped, escaped, escaped)
}

func (r FootnoteRef) Markdown() string {
	return r.String()
}

// Footnote is a single GFM-style footnote definition.
type Footnote struct {
	ID      string
	Content Textable
}

func (f Footnote) hasID() bool {
	return normalizedFootnoteID(f.ID) != ""
}

func footnoteDefinition(id, content string) string {
	if content == "" {
		return fmt.Sprintf("[^%s]:", id)
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	out := fmt.Sprintf("[^%s]: %s", id, lines[0])
	for _, line := range lines[1:] {
		out += "\n    " + line
	}
	return out
}

func (f Footnote) String() string {
	id := normalizedFootnoteID(f.ID)
	if id == "" {
		return ""
	}
	return footnoteDefinition(id, blockTextableString(f.Content))
}

func (f Footnote) ANSI() string {
	id := normalizedFootnoteID(f.ID)
	if id == "" {
		return ""
	}
	return footnoteDefinition(id, blockTextableANSI(f.Content))
}

func (f Footnote) HTML() string {
	id := normalizedFootnoteID(f.ID)
	if id == "" {
		return ""
	}
	escaped := htmlEscapeString(id)
	// Build the element with the content written as its own segment (element-body
	// position, between `>` and ` <a`) rather than interpolated into the quoted
	// attribute template. The id is htmlEscapeString'd; the content is already
	// rendered HTML and never lands inside an attribute's quotes.
	var b strings.Builder
	b.WriteString(`<li id="fn-`)
	b.WriteString(escaped)
	b.WriteString(`">`)
	b.WriteString(blockTextableHTML(f.Content))
	b.WriteString(` <a href="#fnref-`)
	b.WriteString(escaped)
	b.WriteString(`" aria-label="Back to reference">back</a></li>`)
	return b.String()
}

func (f Footnote) Markdown() string {
	id := normalizedFootnoteID(f.ID)
	if id == "" {
		return ""
	}
	return footnoteDefinition(id, blockTextableMarkdown(f.Content))
}

// Footnotes is an ordered block of footnote definitions.
type Footnotes struct {
	Items []Footnote
}

func (f Footnotes) validItems() []Footnote {
	items := make([]Footnote, 0, len(f.Items))
	for _, item := range f.Items {
		if item.hasID() {
			items = append(items, item)
		}
	}
	return items
}

func (f Footnotes) String() string {
	items := f.validItems()
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.String())
	}
	return strings.Join(parts, "\n")
}

func (f Footnotes) ANSI() string {
	items := f.validItems()
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.ANSI())
	}
	return strings.Join(parts, "\n")
}

func (f Footnotes) HTML() string {
	items := f.validItems()
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.HTML())
	}
	return fmt.Sprintf(`<section class="footnotes"><ol>%s</ol></section>`, strings.Join(parts, ""))
}

func (f Footnotes) Markdown() string {
	items := f.validItems()
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.Markdown())
	}
	return strings.Join(parts, "\n")
}
