package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/clicky/api/tailwind"
	commonsText "github.com/flanksource/commons/text"
	"github.com/muesli/termenv"
)

type Text struct {
	Content  string
	Class    Class
	Style    string
	Children []Text
}

func (t Text) Add(child Text) Text {
	t.Children = append(t.Children, child)
	return t
}

func (t Text) Prefix(prefix string) Text {
	t.Content = prefix + t.Content
	return t
}

func (t Text) Suffix(suffix string) Text {
	t.Content = t.Content + suffix
	return t
}

func (t Text) Text(text string, styles ...string) Text {
	return t.Add(Text{Content: text, Style: strings.Join(styles, " ")})
}

func (t Text) Styles(classes ...string) Text {
	if t.Style != "" {
		// Append new classes to existing style
		t.Style = t.Style + " " + strings.Join(classes, " ")
	} else {
		t.Style = strings.Join(classes, " ")
	}
	return t
}

func (t Text) WrapSpace() Text {
	return t.Wrap(" ", " ")
}

func (t Text) Wrap(prefix, suffix string) Text {
	t.Content = prefix + t.Content + suffix
	return t
}

func (t Text) Append(text string, styles ...string) Text {
	t.Children = append(t.Children, Text{Content: text, Style: strings.Join(styles, " ")})
	return t
}

// Indent add spaces before every line in content, apply recursively to children
func (t Text) Indent(spaces int) Text {
	indentation := strings.Repeat(" ", spaces)
	t.Content = indentation + strings.ReplaceAll(t.Content, "\n", "\n"+indentation)
	for i := range t.Children {
		t.Children[i] = t.Children[i].Indent(spaces + 2)
	}
	return t
}

// Printf is like fmt.Printf, but prints floats and durations to 2 decimal places
func (t Text) PrintfWithStyle(format string, style string, args ...interface{}) Text {
	for i := range args {
		switch v := args[i].(type) {
		case float64:
			args[i] = fmt.Sprintf("%.2f", v)
		case time.Duration:
			args[i] = commonsText.HumanizeDuration(v)
		}
	}
	t.Children = append(t.Children, Text{Content: fmt.Sprintf(format, args...), Style: style})
	return t
}
func (t Text) Printf(format string, args ...interface{}) Text {
	return t.PrintfWithStyle(format, "", args...)
}

func (t Text) IsEmpty() bool {
	if t.Content != "" {
		return false
	}
	for _, child := range t.Children {
		if !child.IsEmpty() {
			return false
		}
	}
	return true
}

func (t Text) String() string {
	content := t.Content
	for _, child := range t.Children {
		content += child.String()
	}

	// Check if we have any style to apply
	if t.Class != (Class{}) {
		// Class doesn't have text transform, so just return content
		return content
	} else if t.Style != "" {
		// Apply text transforms only (no styling for plain text)
		transformedText, _ := ApplyTailwindStyle(content, t.Style)
		return transformedText
	}

	return content
}

func (t Text) ANSI() string {
	// Get the effective style (Class takes precedence over Style string)
	var style TailwindStyle
	var transformedText string

	if t.Class != (Class{}) {
		// Use Class if available
		transformedText = t.Content
		style = classToTailwindStyle(t.Class)
	} else if t.Style != "" {
		// Fall back to Style string
		transformedText, style = ApplyTailwindStyle(t.Content, t.Style)
	} else {
		// No style, just return content with children
		result := t.Content
		for _, child := range t.Children {
			result += child.ANSI()
		}
		return result
	}

	// Apply tailwind styles using ANSI escape codes
	content := transformedText
	for _, child := range t.Children {
		content += child.ANSI()
	}

	return formatANSI(content, style)
}

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
		return content
	}

	return formatHTML(transformedText, style, originalStyle)
}

func ResolveStyles(styles ...string) Class {
	var resolved Class

	// Process each style string
	for _, styleStr := range styles {
		if styleStr == "" {
			continue
		}

		// Split into individual classes
		classes := strings.Fields(styleStr)

		for _, class := range classes {
			// Parse colors
			if strings.HasPrefix(class, "text-") && !tailwind.IsTextUtilityClass(class) {
				color := tailwind.Color(class)
				if color != "" {
					resolved.Foreground = &Color{Hex: color}
				}
			} else if strings.HasPrefix(class, "bg-") {
				color := tailwind.Color(class)
				if color != "" {
					resolved.Background = &Color{Hex: color}
				}
			}

			// Parse font properties
			parsedStyle := tailwind.ParseStyle(class)

			// Initialize Font if needed
			if resolved.Font == nil {
				resolved.Font = &Font{}
			}

			// Apply font weight
			if class == "bold" || class == "font-bold" || class == "font-semibold" || class == "font-medium" {
				resolved.Font.Bold = true
			} else if class == "font-normal" {
				resolved.Font.Bold = false
			}

			// Apply font style
			if class == "italic" || class == "font-italic" {
				resolved.Font.Italic = true
			} else if class == "not-italic" {
				resolved.Font.Italic = false
			}

			// Apply text decoration
			if class == "underline" {
				resolved.Font.Underline = true
			} else if class == "no-underline" {
				resolved.Font.Underline = false
			}

			if class == "line-through" || class == "strikethrough" {
				resolved.Font.Strikethrough = true
			}

			// Apply faint/opacity
			if class == "font-light" || class == "font-thin" || class == "font-extralight" ||
				class == "opacity-50" || class == "opacity-75" || class == "opacity-25" {
				resolved.Font.Faint = true
			} else if class == "opacity-100" {
				resolved.Font.Faint = false
			}

			// Parse font size
			if fontSize := tailwind.ParseFontSize(class); fontSize > 0 {
				resolved.Font.Size = fontSize
			}

			// Parse padding
			top, right, bottom, left := tailwind.ParsePadding(class)
			if top != nil || right != nil || bottom != nil || left != nil {
				if resolved.Padding == nil {
					resolved.Padding = &Padding{}
				}

				// Apply non-nil values
				if top != nil {
					resolved.Padding.Top = *top
				}
				if right != nil {
					resolved.Padding.Right = *right
				}
				if bottom != nil {
					resolved.Padding.Bottom = *bottom
				}
				if left != nil {
					resolved.Padding.Left = *left
				}
			}

			// Apply colors from parsed style (as fallback)
			if parsedStyle.Foreground != "" && resolved.Foreground == nil {
				resolved.Foreground = &Color{Hex: parsedStyle.Foreground}
			}
			if parsedStyle.Background != "" && resolved.Background == nil {
				resolved.Background = &Color{Hex: parsedStyle.Background}
			}
		}
	}

	return resolved
}

// ApplyTailwindStyle applies tailwind styles to text - wrapper around tailwind.ApplyStyle
func ApplyTailwindStyle(text string, styleStr string) (string, TailwindStyle) {
	// Import the tailwind package functions
	transformedText, twStyle := tailwind.ApplyStyle(text, styleStr)

	// Convert to our TailwindStyle struct
	style := TailwindStyle{
		Foreground:    twStyle.Foreground,
		Background:    twStyle.Background,
		Bold:          twStyle.Bold,
		Faint:         twStyle.Faint,
		Italic:        twStyle.Italic,
		Underline:     twStyle.Underline,
		Strikethrough: twStyle.Strikethrough,
		TextTransform: twStyle.TextTransform,
	}

	return transformedText, style
}

// classToTailwindStyle converts a Class to TailwindStyle for rendering
func classToTailwindStyle(class Class) TailwindStyle {
	style := TailwindStyle{}

	// Apply colors
	if class.Foreground != nil {
		style.Foreground = class.Foreground.Hex
	}
	if class.Background != nil {
		style.Background = class.Background.Hex
	}

	// Apply font properties
	if class.Font != nil {
		style.Bold = class.Font.Bold
		style.Faint = class.Font.Faint
		style.Italic = class.Font.Italic
		style.Underline = class.Font.Underline
		style.Strikethrough = class.Font.Strikethrough
	}

	return style
}

// TailwindStyle represents parsed tailwind styles
type TailwindStyle struct {
	Foreground    string
	Background    string
	Font          Font
	Bold          bool
	Faint         bool
	Italic        bool
	Underline     bool
	Strikethrough bool
	TextTransform string
}

// formatANSI formats text with ANSI escape codes
func formatANSI(text string, style TailwindStyle) string {
	if text == "" {
		return ""
	}

	// Force termenv to use ANSI mode for consistent output in tests
	output := termenv.NewOutput(termenv.DefaultOutput().Writer(), termenv.WithProfile(termenv.ANSI))
	termStyle := output.String(text)

	// Apply text decorations
	if style.Bold {
		termStyle = termStyle.Bold()
	}
	if style.Faint {
		termStyle = termStyle.Faint()
	}
	if style.Italic {
		termStyle = termStyle.Italic()
	}
	if style.Underline {
		termStyle = termStyle.Underline()
	}

	// Apply foreground color using termenv
	if style.Foreground != "" {
		if color := hexToTermenvColor(style.Foreground); color != nil {
			termStyle = termStyle.Foreground(color)
		}
	}

	// Apply background color using termenv
	if style.Background != "" {
		if color := hexToTermenvColor(style.Background); color != nil {
			termStyle = termStyle.Background(color)
		}
	}

	// Handle strikethrough manually since termenv doesn't support it
	result := termStyle.String()
	if style.Strikethrough {
		// Remove any existing reset codes and add strikethrough
		if strings.HasSuffix(result, "\x1b[0m") {
			result = strings.TrimSuffix(result, "\x1b[0m")
			result = "\x1b[9m" + result + "\x1b[0m"
		} else {
			result = "\x1b[9m" + result + "\x1b[29m"
		}
	}

	return result
}

// hexToTermenvColor converts hex color to termenv Color
func hexToTermenvColor(hex string) termenv.Color {
	if hex == "" {
		return nil
	}

	// Handle special colors
	switch hex {
	case "transparent":
		return nil
	case "currentColor":
		return termenv.ANSIColor(termenv.ANSIBrightWhite)
	}

	// Convert hex to termenv color
	if strings.HasPrefix(hex, "#") {
		return termenv.RGBColor(hex)
	}

	return nil
}

// formatHTML formats text with HTML tags and styles
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
