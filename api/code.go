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

// Code represents source code that can be syntax-highlighted across
// multiple output formats (ANSI terminal, HTML, Markdown).
type Code struct {
	Content  string `json:"content,omitempty"`  // The source code content
	Language string `json:"language,omitempty"` // Language identifier (sql, java, javascript, go, xml, xslt, conf, etc.)
	Style    string `json:"style,omitempty"`    // Optional Tailwind CSS styling for wrapper (HTML only)
}

// NewCode creates a new Code instance with the given content and language.
// If language is empty, it will attempt to detect it from common patterns.
func NewCode(content, language string) Code {
	if language == "" {
		language = detectLanguage(content)
	}
	return Code{
		Content:  content,
		Language: normalizeLanguage(language),
	}
}

// String returns the plain source code without any syntax highlighting.
func (c Code) String() string {
	return c.Content
}

func (c Code) Trim() Code {
	c.Content = strings.TrimSpace(c.Content)
	return c
}

// ANSI returns the source code with ANSI color codes for terminal display.
// Uses chroma with a terminal-compatible formatter.
func (c Code) ANSI() string {
	if c.Content == "" {
		return ""
	}
	if c.Language == "properties" || c.Language == "config" || c.Language == "conf" {
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

func formatProperties(content string) Text {
	lines := strings.Split(content, "\n")
	t := Text{}
	for _, line := range lines {
		line := strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			t = t.Append(line, "text-muted").NewLine()
		} else {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				t = t.Append(parts[0]).Append(" = ", "text-muted").Append(parts[1], "text-orange-500").NewLine()
			} else {
				t = t.Append(line).NewLine()
			}
		}
	}
	return t
}

// HTML returns the source code as syntax-highlighted HTML.
// The output includes inline styles and proper HTML escaping.
func (c Code) HTML() string {
	if c.Content == "" {
		return ""
	}

	if c.Language == "properties" || c.Language == "config" || c.Language == "conf" {
		return formatProperties(c.Content).HTML()
	}

	if strings.Trim(c.Language, ".") == "xml" {
		c.Content = xmlfmt.FormatXML(c.Content, "", "  ")
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

// Markdown returns the source code as a Markdown code block with language tag.
func (c Code) Markdown() string {
	if c.Content == "" {
		return "```\n```"
	}

	lang := c.Language
	if lang == "" {
		lang = "text"
	}

	// Ensure content doesn't break the fence
	content := strings.TrimRight(c.Content, "\n")

	return "```" + lang + "\n" + content + "\n```"
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

// normalizeLanguage converts common language names to chroma-compatible identifiers.
func normalizeLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))

	// Map common aliases to chroma lexer names
	langMap := map[string]string{
		"golang":     "go",
		"js":         "javascript",
		"typescript": "typescript",
		"ts":         "typescript",
		"python":     "python",
		"py":         "python",
		"sql":        "sql",
		"java":       "java",
		"xml":        "xml",
		"xslt":       "xslt",
		"html":       "html",
		"css":        "css",
		"json":       "json",
		"yaml":       "yaml",
		"yml":        "yaml",
		"markdown":   "markdown",
		"md":         "markdown",
		"bash":       "bash",
		"sh":         "bash",
		"shell":      "bash",
		"c":          "c",
		"cpp":        "cpp",
		"c++":        "cpp",
		"csharp":     "csharp",
		"c#":         "csharp",
		"rust":       "rust",
		"ruby":       "ruby",
		"rb":         "ruby",
		"php":        "php",
		"conf":       "properties",
		"config":     "properties",
		"properties": "properties",
		"ini":        "ini",
	}

	if normalized, ok := langMap[lang]; ok {
		return normalized
	}

	return lang
}

// detectLanguage attempts to detect the programming language from code content.
// Returns empty string if detection fails.
func detectLanguage(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	// Simple heuristics for common languages
	if strings.HasPrefix(content, "package ") || strings.Contains(content, "func ") {
		return "go"
	}
	if strings.HasPrefix(content, "SELECT ") || strings.HasPrefix(content, "select ") ||
		strings.Contains(strings.ToUpper(content), "FROM ") {
		return "sql"
	}
	if strings.HasPrefix(content, "<?xml") || strings.HasPrefix(content, "<xsl:") {
		return "xml"
	}
	if strings.Contains(content, "public class ") || strings.Contains(content, "public static void main") {
		return "java"
	}
	if strings.Contains(content, "function ") || strings.Contains(content, "const ") ||
		strings.Contains(content, "let ") || strings.Contains(content, "var ") {
		return "javascript"
	}

	// Default to empty if no detection
	return ""
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
