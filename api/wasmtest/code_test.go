//go:build wasm

package wasmtest

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
)

func TestCodeHTMLIsEscapedPreCodeBlockWithLanguageClass(t *testing.T) {
	code := api.NewCode("if a < b && c > \"d\" {\n\treturn 'e'\n}\n", "golang")

	want := "<pre><code class=\"language-go\">" +
		"if a &lt; b &amp;&amp; c &gt; &#34;d&#34; {\n\treturn &#39;e&#39;\n}\n" +
		"</code></pre>"
	if got := code.HTML(); got != want {
		t.Fatalf("HTML() =\n%q\nwant\n%q", got, want)
	}
	if got := code.String(); got != code.Content {
		t.Fatalf("String() = %q, want the unmodified content %q", got, code.Content)
	}
	if got := code.ANSI(); got != code.Content {
		t.Fatalf("ANSI() = %q, want the unhighlighted content %q", got, code.Content)
	}
}

func TestCodeHTMLWithoutLanguageOmitsClass(t *testing.T) {
	if got, want := (api.Code{Content: "<x>"}).HTML(), "<pre><code>&lt;x&gt;</code></pre>"; got != want {
		t.Fatalf("HTML() = %q, want %q", got, want)
	}
}

func TestCodeHTMLLeavesXMLUnformatted(t *testing.T) {
	xml := `<a><b x="1">t</b></a>`
	want := `<pre><code class="language-xml">&lt;a&gt;&lt;b x=&#34;1&#34;&gt;t&lt;/b&gt;&lt;/a&gt;</code></pre>`
	if got := api.NewCode(xml, "xml").HTML(); got != want {
		t.Fatalf("HTML() = %q, want %q", got, want)
	}
}

func TestGetChromaCSSIsEmpty(t *testing.T) {
	if css := api.GetChromaCSS(); css != "" {
		t.Fatalf("GetChromaCSS() = %q, want empty", css)
	}
}

func TestStackTraceHTMLRendersEscapedSourceLinesWithoutCodeWrappers(t *testing.T) {
	trace := api.NewStackTrace()
	trace.Frames = []api.StackFrame{{
		Class:           "com.example.App",
		Method:          "run",
		File:            "App.java",
		Line:            42,
		SourceLines:     []string{`String s = "<a & b>";`, "run();"},
		SourceStartLine: 41,
		SourceLanguage:  "java",
	}}

	rendered := trace.HTML()
	wantLine := `<span class="stack-source-code px-2 whitespace-pre overflow-x-auto flex-1">` +
		`String s = &quot;&lt;a &amp; b&gt;&quot;;</span>`
	if !strings.Contains(rendered, wantLine) {
		t.Fatalf("HTML() missing escaped source line %q in:\n%s", wantLine, rendered)
	}
	for _, markup := range []string{"<pre", "<code", "chroma", `class="line"`} {
		if strings.Contains(rendered, markup) {
			t.Fatalf("HTML() contains highlighter markup %q:\n%s", markup, rendered)
		}
	}
}
