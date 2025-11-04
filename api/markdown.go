package api

import (
	"fmt"
	"strings"
)

func (t Text) Markdown() string {
	content := t.Content
	for _, child := range t.Children {
		content += child.Markdown()
	}

	// Get the effective style (Class takes precedence over Style string)
	var style TailwindStyle
	var transformedText string

	if t.Class != (Class{}) {
		// Use Class if available
		transformedText = content
		style = classToTailwindStyle(t.Class)
	} else if t.Style != "" {
		// Fall back to Style string
		transformedText, style = ApplyTailwindStyle(content, t.Style)
	} else {
		// No style
		return content
	}

	// Convert tailwind styles to markdown with HTML fallback for colors
	result := transformedText
	hasColors := style.Foreground != "" || style.Background != ""

	// If we have colors, use HTML span with inline CSS for better markdown renderer support
	if hasColors {
		var styles []string

		if style.Foreground != "" {
			styles = append(styles, fmt.Sprintf("color: %s", style.Foreground))
		}
		if style.Background != "" {
			styles = append(styles, fmt.Sprintf("background-color: %s", style.Background))
		}
		if style.Faint {
			styles = append(styles, "opacity: 0.6")
		}

		styleAttr := fmt.Sprintf("style=\"%s\"", strings.Join(styles, "; "))
		result = fmt.Sprintf("<span %s>%s</span>", styleAttr, result)
	}

	// Apply markdown formatting for text decorations
	if style.Bold {
		if hasColors {
			// Bold inside the span
			result = strings.Replace(result, transformedText, "**"+transformedText+"**", 1)
		} else {
			result = "**" + result + "**"
		}
	}
	if style.Italic {
		if hasColors {
			// Italic inside the span
			contentToReplace := transformedText
			if style.Bold {
				contentToReplace = "**" + transformedText + "**"
			}
			result = strings.Replace(result, contentToReplace, "*"+contentToReplace+"*", 1)
		} else {
			result = "*" + result + "*"
		}
	}
	if style.Strikethrough {
		if hasColors {
			// Find the text to strikethrough (may be wrapped in bold/italic)
			contentToReplace := transformedText
			if style.Bold && style.Italic {
				contentToReplace = "*" + "**" + transformedText + "**" + "*"
			} else if style.Bold {
				contentToReplace = "**" + transformedText + "**"
			} else if style.Italic {
				contentToReplace = "*" + transformedText + "*"
			}
			result = strings.Replace(result, contentToReplace, "~~"+contentToReplace+"~~", 1)
		} else {
			result = "~~" + result + "~~"
		}
	}

	// Note: Underline isn't supported in standard markdown, but will be handled by HTML span

	return result
}
