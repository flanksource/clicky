//go:build !wasm

package api

import (
	"bytes"
	"html"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/go-xmlfmt/xmlfmt"
)

// ANSI returns the source code with ANSI color codes for terminal display.
// Uses chroma with a terminal-compatible formatter.
func (c Code) ANSI() string {
	if c.Content == "" {
		return ""
	}
	if isPropertiesLanguage(c.Language) {
		return formatProperties(c.Content).ANSI()
	}

	lexer := getLexer(c.Language)
	if lexer == nil {
		return c.Content
	}

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal")
	if formatter == nil {
		return c.Content
	}

	iterator, err := lexer.Tokenise(nil, c.Content)
	if err != nil {
		return c.Content
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return c.Content
	}

	return buf.String()
}

// HTML returns the source code as syntax-highlighted HTML.
// The output includes inline styles and proper HTML escaping.
func (c Code) HTML() string {
	if c.Content == "" {
		return ""
	}

	if isPropertiesLanguage(c.Language) {
		return formatProperties(c.Content).HTML()
	}

	if strings.Trim(c.Language, ".") == "xml" {
		c.Content = formatXMLBestEffort(c.Content)
	}

	lexer := getLexer(c.Language)
	if lexer == nil {
		// Fallback to plain text with HTML escaping
		return html.EscapeString(c.Content)
	}

	style := styles.Get("github")
	if style == nil {
		style = styles.Fallback
	}

	// Use CSS classes for cleaner HTML
	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.WithLineNumbers(false),
		chromahtml.TabWidth(4),
	)

	iterator, err := lexer.Tokenise(nil, c.Content)
	if err != nil {
		return html.EscapeString(c.Content)
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return html.EscapeString(c.Content)
	}

	return buf.String()
}

func formatXMLBestEffort(content string) (formatted string) {
	defer func() {
		if recover() != nil || strings.TrimSpace(formatted) == "" {
			formatted = content
		}
	}()

	formatted = xmlfmt.FormatXML(content, "", "  ")
	return formatted
}

// getLexer returns the appropriate chroma lexer for the given language.
func getLexer(language string) chroma.Lexer {
	if language == "" {
		return nil
	}

	lexer := lexers.Get(language)
	if lexer == nil {
		return nil
	}

	return chroma.Coalesce(lexer)
}

// highlightCodeLineHTML returns the chroma-highlighted inner HTML of a single
// source line (no <pre>/<code> wrapper) so it can sit inline in a stack-trace
// row. Each chroma-tokenised <span> is preserved so multi-line highlighting
// stays consistent across rows.
func highlightCodeLineHTML(line, language string) string {
	highlighted := strings.TrimRight(NewCode(line, language).HTML(), "\n")
	// chroma's HTML formatter wraps output in `<pre class="chroma">…</pre>`
	// (sometimes with a `<code>` inside); strip those so the line sits inline.
	highlighted = strings.TrimPrefix(highlighted, `<pre class="chroma">`)
	highlighted = strings.TrimPrefix(highlighted, `<code>`)
	highlighted = strings.TrimSuffix(highlighted, `</pre>`)
	highlighted = strings.TrimSuffix(highlighted, `</code>`)
	if strings.TrimSpace(highlighted) == "" {
		return htmlEscapeString(line)
	}
	return highlighted
}

// GetChromaCSS returns the CSS stylesheet for chroma syntax highlighting
// with line wrapping support. This should be included in HTML documents
// that use Code.HTML() output.
func GetChromaCSS() string {
	style := styles.Get("github")
	if style == nil {
		style = styles.Fallback
	}

	formatter := chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.TabWidth(4),
	)

	var buf bytes.Buffer
	if err := formatter.WriteCSS(&buf, style); err != nil {
		return ""
	}

	// Custom CSS for chroma blocks: keep them as block-level <pre> elements
	// (the chroma default), but allow long lines to wrap so wide code doesn't
	// blow out tree views.
	buf.WriteString("\n/* Custom line wrapping for chroma code blocks */\n")
	buf.WriteString("pre.chroma {\n")
	buf.WriteString("    white-space: pre-wrap;\n")
	buf.WriteString("    word-break: break-word;\n")
	buf.WriteString("    overflow-wrap: anywhere;\n")
	buf.WriteString("    margin: 0.25rem 0;\n")
	buf.WriteString("    padding: 0.5rem 0.75rem;\n")
	buf.WriteString("    border-radius: 0.25rem;\n")
	buf.WriteString("}\n")

	return buf.String()
}
