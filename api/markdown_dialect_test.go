package api

import (
	"strings"
	"testing"
)

// TestMarkdownDialectMDXEmitsClassNames covers the JSX dialect. MDX parses raw
// HTML as JSX, where `style` must be an object expression and the attribute is
// `className` — so an inline-CSS span is a compile error, and every consumer
// that feeds markdown to an MDX compiler has to strip or rewrite it. Emitting
// the class the Text already carries avoids the round trip entirely.
func TestMarkdownDialectMDXEmitsClassNames(t *testing.T) {
	for name, tc := range map[string]struct {
		text Text
		want string
	}{
		"foreground": {
			Text{Content: "Error", Style: "text-red-600"},
			`<span className="text-red-600">Error</span>`,
		},
		"background": {
			Text{Content: "Highlight", Style: "bg-yellow-200"},
			`<span className="bg-yellow-200">Highlight</span>`,
		},
		"foreground and background": {
			Text{Content: "Alert", Style: "text-red-600 bg-red-100"},
			`<span className="text-red-600 bg-red-100">Alert</span>`,
		},
		// A colour outside the two the xero-cli sanitiser used to allow.
		"amber": {
			Text{Content: "42", Style: "text-amber-500"},
			`<span className="text-amber-500">42</span>`,
		},
		"no style is untouched": {
			Text{Content: "Plain"},
			"Plain",
		},
	} {
		got := tc.text.MarkdownWithOptions(MarkdownOptions{Dialect: DialectMDX})
		if got != tc.want {
			t.Errorf("%s = %q, want %q", name, got, tc.want)
		}
	}
}

// TestMarkdownDialectMDXForwardsOnlyColourClasses proves the class list is
// filtered rather than forwarded whole. Transform and truncate classes are
// already materialised into the string by ApplyStyle, and clicky's private
// arbitrary values (max-w-[tw-20ch]) mean nothing to Tailwind — emitting either
// would re-apply work already done or feed a consumer's JIT a class it cannot
// compile. Bold/italic keep arriving as markdown markers.
func TestMarkdownDialectMDXForwardsOnlyColourClasses(t *testing.T) {
	got := Text{
		Content: "Alert",
		Style:   "uppercase text-red-600 bg-red-100 font-bold underline",
	}.MarkdownWithOptions(MarkdownOptions{Dialect: DialectMDX})

	if !strings.Contains(got, `className="text-red-600 bg-red-100"`) {
		t.Errorf("colour classes missing or unfiltered: %q", got)
	}
	for _, unwanted := range []string{"uppercase", "underline", "font-bold"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("%q is already materialised and must not be forwarded: %q", unwanted, got)
		}
	}
	if !strings.Contains(got, "**ALERT**") {
		t.Errorf("bold must still arrive as markdown markers, and the text stays transformed: %q", got)
	}
}

// TestMarkdownDialectDefaultUnchanged pins that the dialect is opt-in: the zero
// value must render exactly what every existing consumer already receives.
func TestMarkdownDialectDefaultUnchanged(t *testing.T) {
	text := Text{Content: "Error", Style: "text-red-500"}

	want := `<span style="color: #ef4444">Error</span>`
	if got := text.Markdown(); got != want {
		t.Errorf("Markdown() = %q, want %q", got, want)
	}
	if got := text.MarkdownWithOptions(MarkdownOptions{}); got != want {
		t.Errorf("zero options = %q, want %q", got, want)
	}
	if got := text.MarkdownWithOptions(MarkdownOptions{Dialect: DialectGFM}); got != want {
		t.Errorf("explicit GFM = %q, want %q", got, want)
	}
}

// TestMarkdownOptionsReachNestedChildren is the propagation fix. RenderMarkdown
// falls back to the options-less Markdown() for any type that does not
// implement MarkdownWithOptions, so options were silently dropped at every
// nesting boundary — a styled Text inside a Link emitted a colour span even
// under NoColor. The dialect would have inherited the same hole.
func TestMarkdownOptionsReachNestedChildren(t *testing.T) {
	styled := Text{Content: "overdue", Style: "text-red-600"}

	for name, node := range map[string]Textable{
		"link":       Link{Content: styled, Href: "https://example.test"},
		"heading":    Heading{Level: 2, Content: styled},
		"blockquote": Blockquote{Content: TextList{styled}},
		"keyed":      Keyed{Key: "k", Value: styled},
		"list":       List{Items: []Textable{styled}},
	} {
		mdx := RenderMarkdown(node, MarkdownOptions{Dialect: DialectMDX})
		if strings.Contains(mdx, "style=") {
			t.Errorf("%s: MDX dialect did not reach the child: %q", name, mdx)
		}
		if !strings.Contains(mdx, "className=") {
			t.Errorf("%s: child lost its styling entirely: %q", name, mdx)
		}

		plain := RenderMarkdown(node, MarkdownOptions{NoColor: true})
		if strings.Contains(plain, "<span") {
			t.Errorf("%s: NoColor did not reach the child: %q", name, plain)
		}
	}
}
