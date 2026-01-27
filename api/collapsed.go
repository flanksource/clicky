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

// ANSI returns terminal output with expandable indicator
func (c Collapsed) ANSI() string {
	var result strings.Builder

	// Use chevron icon for collapsed state
	result.WriteString(icons.ChevronRight.ANSI())
	result.WriteString(" ")
	result.WriteString(c.Label)
	result.WriteString("\n")

	if c.Content != nil {
		// Indent content
		content := c.Content.ANSI()
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
	html.WriteString(fmt.Sprintf(`<button @click="open = !open" class="%s">`, buttonClass))

	// Chevron icon that rotates based on state
	html.WriteString(`<iconify-icon x-show="!open" icon="codicon:chevron-right"></iconify-icon>`)
	html.WriteString(`<iconify-icon x-show="open" icon="codicon:chevron-down"></iconify-icon>`)

	// Custom icon if provided
	if c.Icon != nil {
		html.WriteString(c.Icon.HTML())
	}

	// Button label
	html.WriteString(fmt.Sprintf(`<span>%s</span>`, c.Label))
	html.WriteString(`</button>`)

	// Collapsible content
	if c.Content != nil {
		html.WriteString(`<div x-show="open" x-collapse class="ml-6 mt-2">`)
		html.WriteString(c.Content.HTML())
		html.WriteString(`</div>`)
	}

	html.WriteString(`</span>`)

	return html.String()
}

// Markdown returns Markdown with HTML details/summary fallback
func (c Collapsed) Markdown() string {
	var result strings.Builder

	// Use HTML details/summary element which works in many Markdown renderers
	result.WriteString("<details>\n")
	result.WriteString(fmt.Sprintf("<summary>%s</summary>\n\n", c.Label))

	if c.Content != nil {
		result.WriteString(c.Content.Markdown())
		result.WriteString("\n")
	}

	result.WriteString("</details>")

	return result.String()
}

func (c Collapsed) MarkdownSlack() string {
	var result strings.Builder

	result.WriteString("<details>\n")
	result.WriteString(fmt.Sprintf("<summary>%s</summary>\n\n", c.Label))

	if c.Content != nil {
		result.WriteString(markdownTextable(c.Content, true))
		result.WriteString("\n")
	}

	result.WriteString("</details>")

	return result.String()
}
