//go:build !wasm

package api

import (
	"strings"
	"testing"
)

func TestCode_ANSI(t *testing.T) {
	tests := []struct {
		name         string
		code         Code
		wantContains string
	}{
		{
			name:         "sql_with_ansi",
			code:         Code{Content: "SELECT * FROM users", Language: "sql"},
			wantContains: "SELECT",
		},
		{
			name:         "go_with_ansi",
			code:         Code{Content: "package main\n\nfunc main() {}", Language: "go"},
			wantContains: "package",
		},
		{
			name:         "javascript_with_ansi",
			code:         Code{Content: "const x = 1;", Language: "javascript"},
			wantContains: "const",
		},
		{
			name:         "empty_code",
			code:         Code{Content: "", Language: "sql"},
			wantContains: "",
		},
		{
			name:         "unknown_language",
			code:         Code{Content: "some code", Language: "unknownlang"},
			wantContains: "some code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.code.ANSI()
			if tt.wantContains != "" && !strings.Contains(got, tt.wantContains) {
				t.Errorf("Code.ANSI() should contain %q, got %v", tt.wantContains, got)
			}
			// For non-empty content with known language, verify ANSI codes are present
			if tt.code.Content != "" && tt.code.Language != "unknownlang" && tt.name != "empty_code" {
				// ANSI escape codes start with \x1b or \033
				if !strings.Contains(got, "\x1b[") && !strings.Contains(got, "\033[") {
					t.Errorf("Code.ANSI() should contain ANSI escape codes for %s", tt.name)
				}
			}
		})
	}
}

// TestCodeBlock_HTMLWrapsInPreBlock ensures that CodeBlock("typescript", ...)
// produces a chroma-wrapped <pre class="chroma"><code>...</code></pre>
// element. Without language normalization the output collapses to escaped
// inline text, which renders as a single line in HTML disclosures.
func TestCodeBlock_HTMLWrapsInPreBlock(t *testing.T) {
	pseudo := "if cond {\n  doThing()\n}\nfollowUp()"
	html := CodeBlock("typescript", pseudo).HTML()
	if !strings.Contains(html, `<pre class="chroma">`) {
		t.Errorf("CodeBlock(typescript).HTML() missing <pre class=\"chroma\"> wrapper, got: %s", html)
	}
	if !strings.Contains(html, "<code>") {
		t.Errorf("CodeBlock(typescript).HTML() missing <code> wrapper, got: %s", html)
	}
	// Token-level highlighting should be present (chroma .k for keywords).
	if !strings.Contains(html, `class="k"`) {
		t.Errorf("CodeBlock(typescript).HTML() missing keyword highlighting span, got: %s", html)
	}
}

func TestCode_HTML(t *testing.T) {
	tests := []struct {
		name         string
		code         Code
		wantContains []string
	}{
		{
			name:         "sql_html",
			code:         Code{Content: "SELECT * FROM users", Language: "sql"},
			wantContains: []string{"SELECT", "FROM", "users"},
		},
		{
			name:         "go_html",
			code:         Code{Content: "package main", Language: "go"},
			wantContains: []string{"package", "main"},
		},
		{
			name:         "java_html",
			code:         Code{Content: "public class Test {}", Language: "java"},
			wantContains: []string{"public", "class", "Test"},
		},
		{
			name:         "javascript_html",
			code:         Code{Content: "const x = 1;", Language: "javascript"},
			wantContains: []string{"const", "x"},
		},
		{
			name:         "xml_html",
			code:         Code{Content: "<?xml version=\"1.0\"?>\n<root></root>", Language: "xml"},
			wantContains: []string{"xml", "root"},
		},
		{
			name:         "empty_code",
			code:         Code{Content: "", Language: "sql"},
			wantContains: []string{},
		},
		{
			name:         "unknown_language_escapes_html",
			code:         Code{Content: "<script>alert('xss')</script>", Language: "unknownlang"},
			wantContains: []string{"&lt;script&gt;", "&lt;/script&gt;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.code.HTML()
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("Code.HTML() should contain %q, got %v", want, got)
				}
			}
			// Verify HTML output contains style tags for syntax highlighting
			if tt.code.Content != "" && tt.code.Language != "unknownlang" && tt.name != "empty_code" && tt.name != "unknown_language_escapes_html" {
				if !strings.Contains(got, "style=") && !strings.Contains(got, "class=") {
					t.Logf("Warning: Code.HTML() for %s might not have syntax highlighting", tt.name)
				}
			}
		})
	}
}

func TestCodeHTML_XMLFormatterPanicFallsBackToHighlightedContent(t *testing.T) {
	code := Code{
		Content:  `<AddressScreen><Events><Event TYPE="ONLOAD"><ActionSet><Condition IF="1=1"></Condition></ActionSet></Event></Events></AddressScreen>`,
		Language: "xml",
	}

	got := code.HTML()
	if !strings.Contains(got, "AddressScreen") {
		t.Fatalf("Code.HTML() should preserve original XML after formatter failure, got %s", got)
	}
	if !strings.Contains(got, `class=`) {
		t.Fatalf("Code.HTML() should still use Chroma highlighting after formatter failure, got %s", got)
	}
}
