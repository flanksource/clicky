package api

import (
	"fmt"
	"strings"
)

func (t Text) HTML() string {
	content := t.Content
	for _, child := range t.Children {
		content += child.HTML()
	}

	// Get the effective style (Class takes precedence over Style string)
	var style TailwindStyle
	var transformedText string
	var originalStyle string

	if t.Class != (Class{}) {
		// Use Class if available
		transformedText = content
		style = classToTailwindStyle(t.Class)
		// Could convert Class back to style string if needed
		originalStyle = ""
	} else if t.Style != "" {
		// Fall back to Style string
		transformedText, style = ApplyTailwindStyle(content, t.Style)
		originalStyle = t.Style
	} else {
		// No style
		transformedText = content
	}

	html := formatHTML(transformedText, style, originalStyle)

	// Apply tooltip if present
	if t.Tooltip != nil && t.Tooltip.String() != "" {
		// HTML-escape the tooltip content using standard library
		escapedTooltip := htmlEscapeString(t.Tooltip.String())
		html = fmt.Sprintf(`<span title="%s">%s</span>`, escapedTooltip, html)
	}
	return html
}

func (kv KeyValuePair) HTML() string {
	// Determine style
	style := kv.Style
	if style == "" {
		style = "compact"
	}

	// HTML-escape the key and value
	escapedKey := htmlEscapeString(kv.Key)
	escapedValue := htmlEscapeString(fmt.Sprintf("%v", kv.Value))

	if strings.Contains(style, "badge") {
		// Badge style: pill-shaped badge
		return fmt.Sprintf(
			`<span class="inline-flex items-center gap-1 px-3 py-1 rounded-full bg-gray-100"><dt class="text-xs font-medium text-gray-600">%s:</dt><dd class="text-xs font-semibold text-gray-900">%s</dd></span>`,
			escapedKey,
			escapedValue,
		)
	}

	// Compact style (default): inline with minimal spacing
	return fmt.Sprintf(
		`<div class="inline-flex gap-1"><dt class="text-gray-500 font-medium">%s:</dt><dd class="text-gray-900">%s</dd></div>`,
		escapedKey,
		escapedValue,
	)
}

// htmlEscapeString escapes special HTML characters for use in attributes
func htmlEscapeString(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

type HtmlElement struct {
	Tag        string
	Attributes map[string]string
	Content    string
	Fallback   Textable
}

func Badge(label string, classes ...string) Textable {
	allClasses := append([]string{"badge", "p-0.5", "mr-1", "rounded-lg", "text-xs", "font-light bg-gray-200"}, classes...)
	return HtmlElement{
		Tag: "span",
		Attributes: map[string]string{
			"class": strings.Join(allClasses, " "),
		},
		Content: label,
		Fallback: Text{
			Content: label,
			Style:   strings.Join(classes, " "),
		},
	}
}

func (e HtmlElement) HTML() string {
	if e.Tag == "" {
		return e.Content
	}
	return fmt.Sprintf("<%s %s>%s</%s>", e.Tag, formatAttributes(e.Attributes), e.Content, e.Tag)
}

// formatHTML generates HTML with both semantic tags and CSS styling for maximum
// compatibility across different HTML renderers and Tailwind CSS environments.
func formatHTML(text string, style TailwindStyle, originalStyle string) string {
	if text == "" {
		return ""
	}

	result := text
	var tags []string
	var styles []string
	var classes []string

	// Apply semantic HTML tags first
	if style.Bold {
		tags = append(tags, "strong")
	}
	if style.Italic {
		tags = append(tags, "em")
	}
	if style.Underline {
		tags = append([]string{"u"}, tags...) // Underline goes innermost
	}
	if style.Strikethrough {
		tags = append(tags, "s")
	}

	// Apply CSS styles for fallback compatibility
	if style.Foreground != "" {
		styles = append(styles, fmt.Sprintf("color: %s", style.Foreground))
	}
	if style.Background != "" {
		styles = append(styles, fmt.Sprintf("background-color: %s", style.Background))
	}
	if style.Faint {
		styles = append(styles, "opacity: 0.6")
	}

	// Include original Tailwind classes if provided
	if originalStyle != "" {
		// Split and clean up classes
		tailwindClasses := strings.Fields(originalStyle)
		classes = append(classes, tailwindClasses...)
	}

	// Wrap in semantic tags
	for _, tag := range tags {
		result = fmt.Sprintf("<%s>%s</%s>", tag, result, tag)
	}

	// Add wrapper span with both classes and inline styles for maximum compatibility
	if len(styles) > 0 || len(classes) > 0 {
		var attributes []string

		// Add Tailwind classes if any
		if len(classes) > 0 {
			attributes = append(attributes, fmt.Sprintf("class=\"%s\"", strings.Join(classes, " ")))
		}

		// Add inline CSS as fallback
		if len(styles) > 0 {
			attributes = append(attributes, fmt.Sprintf("style=\"%s\"", strings.Join(styles, "; ")))
		}

		result = fmt.Sprintf("<span %s>%s</span>", strings.Join(attributes, " "), result)
	}

	return result
}

func formatAttributes(attrs map[string]string) string {
	var parts []string
	for k, v := range attrs {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, k, v))
	}
	return strings.Join(parts, " ")
}

func (e HtmlElement) String() string {
	return e.Fallback.String()
}

func (e HtmlElement) ANSI() string {
	return e.Fallback.ANSI()
}

func (e HtmlElement) Markdown() string {
	return e.Fallback.Markdown()
}

var NBSP = HtmlElement{
	Tag:      "",
	Content:  "&nbsp;",
	Fallback: Text{Content: " "},
}

var TAB = HtmlElement{
	Tag:      "",
	Content:  "&emsp;",
	Fallback: Text{Content: "\t"},
}

var BR = HtmlElement{
	Tag: "br",
	Attributes: map[string]string{
		"class": "clicky",
	},
	Content:  "",
	Fallback: Text{Content: "\n"},
}

var HR = HtmlElement{
	Tag:      "hr",
	Content:  "",
	Fallback: Text{Content: "\n--------------------------\n"},
}
