package api

import (
	"strings"
)

// Code represents source code that can be syntax-highlighted across
// multiple output formats (ANSI terminal, HTML, Markdown). ANSI and HTML are
// chroma-highlighted natively (code_chroma.go) and plain on wasm
// (code_wasm.go).
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

func isPropertiesLanguage(language string) bool {
	return language == "properties" || language == "config" || language == "conf"
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
