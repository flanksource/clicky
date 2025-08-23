package pdf

import (
	"fmt"
	"regexp"
	"strings"
	
	"github.com/flanksource/clicky/api"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/breakline"
)

// Text widget for rendering text in PDF
type Text struct {
	Text       api.Text `json:"text,omitempty"`
	EnableHTML bool     `json:"enable_html,omitempty"`     // Enable HTML parsing
	EnableMD   bool     `json:"enable_markdown,omitempty"` // Enable Markdown parsing
}

// Draw implements the Widget interface
func (t Text) Draw(b *Builder) error {
	// Process content based on flags
	processedText := t.Text
	
	if t.EnableMD && processedText.Content != "" {
		processedText.Content = t.parseMarkdown(processedText.Content)
	}
	
	if t.EnableHTML && processedText.Content != "" {
		processedText.Content = t.parseHTML(processedText.Content)
	}
	
	// Draw the text and all its children
	t.drawTextWithChildren(b, processedText)
	return nil
}

// drawTextWithChildren recursively draws text and its children
func (t Text) drawTextWithChildren(b *Builder, apiText api.Text) {
	// Calculate height for this text
	height := b.style.CalculateTextHeight(apiText.Class)
	
	// Apply padding if specified
	var topPadding, bottomPadding float64
	if apiText.Class.Padding != nil {
		_, topPadding, _, bottomPadding = b.style.CalculatePadding(apiText.Class.Padding)
		
		// Add top padding as empty row
		if topPadding > 0 {
			b.maroto.AddRows(row.New(topPadding))
		}
	}
	
	// Draw main content if present
	if apiText.Content != "" {
		// Convert style to text properties
		textProps := b.style.ConvertToTextProps(apiText.Class)
		
		// Apply alignment from Style field (Tailwind classes)
		if apiText.Style != "" {
			if strings.Contains(apiText.Style, "text-center") {
				textProps.Align = align.Center
			} else if strings.Contains(apiText.Style, "text-right") {
				textProps.Align = align.Right
			} else if strings.Contains(apiText.Style, "text-justify") {
				textProps.Align = align.Justify
			}
		}
		
		// Create text component
		textComponent := text.New(apiText.Content, *textProps)
		
		// Create column with text
		textCol := col.New(12).Add(textComponent)
		
		// Add row with text
		b.maroto.AddRow(height, textCol)
	}
	
	// Add bottom padding if specified
	if bottomPadding > 0 {
		b.maroto.AddRows(row.New(bottomPadding))
	}
	
	// Draw children recursively
	for _, child := range apiText.Children {
		t.drawTextWithChildren(b, child)
	}
}

// parseMarkdown converts markdown syntax to formatted text
// Note: This returns processed text that will be rendered with appropriate styles
func (t Text) parseMarkdown(content string) string {
	// For now, we'll strip markdown syntax and return plain text
	// In a full implementation, this would parse and apply styles
	
	// Headers - convert to plain text with emphasis
	content = regexp.MustCompile(`^#{1,6}\s+(.+)$`).ReplaceAllString(content, "$1")
	
	// Bold
	content = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(content, "$1")
	
	// Italic
	content = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`_(.+?)_`).ReplaceAllString(content, "$1")
	
	// Strikethrough
	content = regexp.MustCompile(`~~(.+?)~~`).ReplaceAllString(content, "$1")
	
	// Code blocks
	content = regexp.MustCompile("```[^`]*```").ReplaceAllString(content, "[code block]")
	content = regexp.MustCompile("`([^`]+)`").ReplaceAllString(content, "$1")
	
	// Links
	content = regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`).ReplaceAllString(content, "$1")
	
	// Lists - convert to plain text
	content = regexp.MustCompile(`^\s*[-*+]\s+(.+)$`).ReplaceAllString(content, "• $1")
	content = regexp.MustCompile(`^\s*\d+\.\s+(.+)$`).ReplaceAllString(content, "$1")
	
	return content
}

// parseHTML converts basic HTML tags to formatted text
func (t Text) parseHTML(content string) string {
	// Strip HTML tags but preserve content
	// In a full implementation, this would parse and apply styles
	
	// Bold tags
	content = regexp.MustCompile(`<b>(.+?)</b>`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`<strong>(.+?)</strong>`).ReplaceAllString(content, "$1")
	
	// Italic tags
	content = regexp.MustCompile(`<i>(.+?)</i>`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`<em>(.+?)</em>`).ReplaceAllString(content, "$1")
	
	// Underline and strikethrough
	content = regexp.MustCompile(`<u>(.+?)</u>`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`<s>(.+?)</s>`).ReplaceAllString(content, "$1")
	content = regexp.MustCompile(`<strike>(.+?)</strike>`).ReplaceAllString(content, "$1")
	
	// Headers
	for i := 6; i >= 1; i-- {
		pattern := fmt.Sprintf(`<h%d[^>]*>(.+?)</h%d>`, i, i)
		content = regexp.MustCompile(pattern).ReplaceAllString(content, "$1")
	}
	
	// Paragraphs and breaks
	content = strings.ReplaceAll(content, "<br/>", "\n")
	content = strings.ReplaceAll(content, "<br>", "\n")
	content = strings.ReplaceAll(content, "</p>", "\n")
	content = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(content, "")
	
	// Links
	content = regexp.MustCompile(`<a[^>]+>(.+?)</a>`).ReplaceAllString(content, "$1")
	
	// Lists
	content = strings.ReplaceAll(content, "<li>", "• ")
	content = strings.ReplaceAll(content, "</li>", "\n")
	content = regexp.MustCompile(`<[ou]l[^>]*>`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`</[ou]l>`).ReplaceAllString(content, "")
	
	// Remove any remaining tags
	content = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(content, "")
	
	// Clean up extra whitespace
	content = strings.TrimSpace(content)
	
	return content
}

// Enhanced text drawing with more options
func (t Text) drawEnhancedText(b *Builder, apiText api.Text) {
	// Apply Tailwind classes if specified
	if apiText.Style != "" {
		resolvedClass := api.ResolveStyles(apiText.Style)
		apiText.Class = mergeClasses(apiText.Class, resolvedClass)
	}
	
	// Convert to props
	textProps := b.style.ConvertToTextProps(apiText.Class)
	
	// Add alignment support
	if strings.Contains(apiText.Style, "text-center") {
		textProps.Align = align.Center
	} else if strings.Contains(apiText.Style, "text-right") {
		textProps.Align = align.Right
	} else if strings.Contains(apiText.Style, "text-justify") {
		textProps.Align = align.Justify
	}
	
	// Add break line strategy
	if strings.Contains(apiText.Style, "break-words") {
		textProps.BreakLineStrategy = breakline.DashStrategy
	}
	
	// TODO: Add hyperlink support if api.Text gets an Href field
	
	// Calculate height
	height := b.style.CalculateTextHeight(apiText.Class)
	
	// Create and add text component
	textComponent := text.New(apiText.Content, *textProps)
	b.maroto.AddRow(height, col.New(12).Add(textComponent))
}

// mergeClasses merges two api.Class instances
func mergeClasses(base, override api.Class) api.Class {
	result := base
	
	if override.Font != nil {
		result.Font = override.Font
	}
	if override.Foreground != nil {
		result.Foreground = override.Foreground
	}
	if override.Background != nil {
		result.Background = override.Background
	}
	if override.Padding != nil {
		result.Padding = override.Padding
	}
	if override.Border != nil {
		result.Border = override.Border
	}
	if override.Name != "" {
		result.Name = override.Name
	}
	
	return result
}