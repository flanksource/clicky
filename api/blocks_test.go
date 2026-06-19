package api

import (
	"strings"
	"testing"
)

func TestHeadingRenderers(t *testing.T) {
	heading := Heading{Level: 9, Content: Text{Content: "Annual report"}}

	if got := heading.String(); got != "Annual report" {
		t.Fatalf("String() = %q", got)
	}
	if got := heading.Markdown(); got != "###### Annual report" {
		t.Fatalf("Markdown() = %q", got)
	}
	if got := heading.HTML(); got != "<h6>Annual report</h6>" {
		t.Fatalf("HTML() = %q", got)
	}
	if got := heading.ANSI(); !strings.Contains(got, "Annual report") || !strings.Contains(got, "\x1b[1m") {
		t.Fatalf("ANSI() = %q, want bold annual report", got)
	}
}

func TestBlockquoteRenderers(t *testing.T) {
	quote := Blockquote{Content: Text{Content: "first line\nsecond line"}}
	want := "> first line\n> second line"

	if got := quote.String(); got != want {
		t.Fatalf("String() = %q", got)
	}
	if got := quote.ANSI(); got != want {
		t.Fatalf("ANSI() = %q", got)
	}
	if got := quote.Markdown(); got != want {
		t.Fatalf("Markdown() = %q", got)
	}
	if got := quote.HTML(); got != "<blockquote>first line\nsecond line</blockquote>" {
		t.Fatalf("HTML() = %q", got)
	}
}

func TestFootnoteRenderers(t *testing.T) {
	ref := FootnoteRef{ID: "cash"}
	if got := ref.String(); got != "[^cash]" {
		t.Fatalf("String() = %q", got)
	}
	if got := ref.Markdown(); got != "[^cash]" {
		t.Fatalf("Markdown() = %q", got)
	}
	if got := ref.ANSI(); got != "[^cash]" {
		t.Fatalf("ANSI() = %q", got)
	}
	if got := ref.HTML(); got != `<sup id="fnref-cash"><a href="#fn-cash">[^cash]</a></sup>` {
		t.Fatalf("HTML() = %q", got)
	}

	note := Footnote{ID: "cash", Content: Text{Content: "Cash includes bank deposits\nand call accounts."}}
	wantMarkdown := "[^cash]: Cash includes bank deposits\n    and call accounts."
	if got := note.String(); got != wantMarkdown {
		t.Fatalf("String() = %q", got)
	}
	if got := note.Markdown(); got != wantMarkdown {
		t.Fatalf("Markdown() = %q", got)
	}
	if got := note.HTML(); !strings.Contains(got, `id="fn-cash"`) || !strings.Contains(got, `href="#fnref-cash"`) {
		t.Fatalf("HTML() = %q, want linked footnote definition", got)
	}
}

func TestFootnotesRenderersSkipEmptyIDs(t *testing.T) {
	notes := Footnotes{Items: []Footnote{
		{ID: "cash", Content: Text{Content: "Cash equivalents."}},
		{ID: "", Content: Text{Content: "skip me"}},
		{ID: "debt", Content: Text{Content: "Lease liabilities."}},
	}}

	want := "[^cash]: Cash equivalents.\n[^debt]: Lease liabilities."
	if got := notes.String(); got != want {
		t.Fatalf("String() = %q", got)
	}
	if got := notes.Markdown(); got != want {
		t.Fatalf("Markdown() = %q", got)
	}
	html := notes.HTML()
	if !strings.Contains(html, `<section class="footnotes"><ol>`) {
		t.Fatalf("HTML() = %q, want ordered footnotes section", html)
	}
	if strings.Contains(html, "skip me") {
		t.Fatalf("HTML() = %q, empty ID footnote was not skipped", html)
	}
}
