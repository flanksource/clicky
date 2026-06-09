package api

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api/icons"
)

// Collapsed represents a collapsible section with a clickable button to toggle visibility.
// In HTML output, it uses Alpine.js collapse plugin for smooth animations.
// For other formats (ANSI, Markdown, plain text), it provides appropriate fallbacks.
type Collapsed struct {
	Label   string   // Button label text
	Content Textable // Content to show/hide
	Style   string   // Tailwind CSS classes for button styling
	Icon    *icons.Icon
	// CollapseANSI keeps the block collapsed in terminal output: ANSI renders a
	// label-only "▶ <label>" header and hides the content (which terminals
	// cannot toggle), while HTML/Markdown stay interactively expandable. Use it
	// for bulky-but-secondary content (e.g. a raw-JSON dump) that should not
	// flood a failure trace on the console but must remain in the report/web.
	CollapseANSI bool
}

// String returns plain text representation with label and content
func (c Collapsed) String() string {
	var result strings.Builder
	result.WriteString("▶ ")
	result.WriteString(c.Label)
	result.WriteString("\n")
	if c.Content != nil {
		// Indent content slightly
		content := c.Content.String()
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if line != "" {
				result.WriteString("  ")
				result.WriteString(line)
				result.WriteString("\n")
			}
		}
	}
	return result.String()
}

// ANSI returns the content directly without the collapsed wrapper, since
// terminals don't support interactive expand/collapse. When CollapseANSI is set,
// the content is hidden and only a "▶ <label>" header is shown, keeping bulky
// secondary content out of the terminal while leaving it expandable in HTML.
func (c Collapsed) ANSI() string {
	if c.CollapseANSI {
		return "▶ " + c.Label
	}
	if c.Content != nil {
		return c.Content.ANSI()
	}
	return ""
}

// HTML returns HTML with Alpine.js collapse functionality
func (c Collapsed) HTML() string {
	// Default styling for the button
	buttonClass := "text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50  outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
	if c.Style != "" {
		buttonClass = c.Style
	}

	// Build the collapsible HTML using Alpine.js
	var html strings.Builder

	// Container with Alpine.js state
	html.WriteString(`<span x-data="{ open: false }" class="space-y-2">`)

	// Button with toggle functionality
	fmt.Fprintf(&html, `<button @click="open = !open" class="%s">`, buttonClass)

	// Chevron icon that rotates based on state
	html.WriteString(`<iconify-icon x-show="!open" icon="codicon:chevron-right"></iconify-icon>`)
	html.WriteString(`<iconify-icon x-show="open" style="display:none" icon="codicon:chevron-down"></iconify-icon>`)

	// Custom icon if provided
	if c.Icon != nil {
		html.WriteString(c.Icon.HTML())
	}

	// Button label
	fmt.Fprintf(&html, `<span>%s</span>`, c.Label)
	html.WriteString(`</button>`)

	// Collapsible content - use x-if with template to avoid adding heavy content to DOM until expanded.
	// Use StaticHTML when available since scripts inside <template x-if> won't execute.
	if c.Content != nil {
		html.WriteString(`<template x-if="open"><div class="ml-6 mt-2">`)
		if sp, ok := c.Content.(StaticHTMLProvider); ok {
			html.WriteString(sp.StaticHTML())
		} else {
			html.WriteString(c.Content.HTML())
		}
		html.WriteString(`</div></template>`)
	}

	html.WriteString(`</span>`)

	return html.String()
}

// Markdown returns Markdown with HTML details/summary fallback
func (c Collapsed) Markdown() string {
	var result strings.Builder

	// Use HTML details/summary element which works in many Markdown renderers
	result.WriteString("<details>\n")
	fmt.Fprintf(&result, "<summary>%s</summary>\n\n", c.Label)

	if c.Content != nil {
		result.WriteString(c.Content.Markdown())
		result.WriteString("\n")
	}

	result.WriteString("</details>")

	return result.String()
}

func (c Collapsed) MarkdownSlack() string {
	var result strings.Builder

	fmt.Fprintf(&result, "*▸ %s*\n", c.Label)

	if c.Content != nil {
		result.WriteString(markdownTextable(c.Content, true))
		result.WriteString("\n")
	}

	return strings.TrimRight(result.String(), "\n")
}
