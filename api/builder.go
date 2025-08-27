package api

import (
	"strings"
)

// TextBuilder provides a fluent interface for building Text objects
type TextBuilder struct {
	text Text
}

// StyleBuilder provides a fluent interface for building styles
type StyleBuilder struct {
	styles []string
}

// NewText creates a new TextBuilder with the given content
func NewText(content string) *TextBuilder {
	return &TextBuilder{
		text: Text{
			Content:  content,
			Children: make([]Text, 0),
		},
	}
}

// NewStyle creates a new StyleBuilder
func NewStyle() *StyleBuilder {
	return &StyleBuilder{
		styles: make([]string, 0),
	}
}

// Content sets the text content
func (tb *TextBuilder) Content(content string) *TextBuilder {
	tb.text.Content = content
	return tb
}

// Style applies a custom style string
func (tb *TextBuilder) Style(style string) *TextBuilder {
	tb.text.Style = style
	return tb
}

// Bold makes the text bold
func (tb *TextBuilder) Bold() *TextBuilder {
	tb.addStyle("font-bold")
	return tb
}

// Italic makes the text italic
func (tb *TextBuilder) Italic() *TextBuilder {
	tb.addStyle("italic")
	return tb
}

// Underline makes the text underlined
func (tb *TextBuilder) Underline() *TextBuilder {
	tb.addStyle("underline")
	return tb
}

// Strikethrough adds strikethrough to the text
func (tb *TextBuilder) Strikethrough() *TextBuilder {
	tb.addStyle("line-through")
	return tb
}

// Color sets the text color
func (tb *TextBuilder) Color(color string) *TextBuilder {
	// Handle both hex colors and named colors
	if strings.HasPrefix(color, "#") {
		tb.addStyle("text-[" + color + "]")
	} else {
		tb.addStyle("text-" + color)
	}
	return tb
}

// Background sets the background color
func (tb *TextBuilder) Background(color string) *TextBuilder {
	// Handle both hex colors and named colors
	if strings.HasPrefix(color, "#") {
		tb.addStyle("bg-[" + color + "]")
	} else {
		tb.addStyle("bg-" + color)
	}
	return tb
}

// Faint makes the text faint/dim
func (tb *TextBuilder) Faint() *TextBuilder {
	tb.addStyle("opacity-60")
	return tb
}

// Success applies success styling (green)
func (tb *TextBuilder) Success() *TextBuilder {
	return tb.Color("green-600")
}

// Error applies error styling (red)
func (tb *TextBuilder) Error() *TextBuilder {
	return tb.Color("red-600")
}

// Warning applies warning styling (yellow)
func (tb *TextBuilder) Warning() *TextBuilder {
	return tb.Color("yellow-600")
}

// Info applies info styling (blue)
func (tb *TextBuilder) Info() *TextBuilder {
	return tb.Color("blue-600")
}

// Muted applies muted styling (gray)
func (tb *TextBuilder) Muted() *TextBuilder {
	return tb.Color("gray-500")
}

// Uppercase transforms text to uppercase
func (tb *TextBuilder) Uppercase() *TextBuilder {
	tb.addStyle("uppercase")
	return tb
}

// Lowercase transforms text to lowercase
func (tb *TextBuilder) Lowercase() *TextBuilder {
	tb.addStyle("lowercase")
	return tb
}

// Capitalize capitalizes the first letter of each word
func (tb *TextBuilder) Capitalize() *TextBuilder {
	tb.addStyle("capitalize")
	return tb
}

// Child adds a child Text element
func (tb *TextBuilder) Child(child Text) *TextBuilder {
	tb.text.Children = append(tb.text.Children, child)
	return tb
}

// ChildBuilder adds a child using a TextBuilder
func (tb *TextBuilder) ChildBuilder(childBuilder *TextBuilder) *TextBuilder {
	tb.text.Children = append(tb.text.Children, childBuilder.Build())
	return tb
}

// Build returns the constructed Text object
func (tb *TextBuilder) Build() Text {
	return tb.text
}

// addStyle adds a style to the current style string
func (tb *TextBuilder) addStyle(style string) {
	if tb.text.Style == "" {
		tb.text.Style = style
	} else {
		tb.text.Style += " " + style
	}
}

// StyleBuilder methods

// Bold adds bold styling
func (sb *StyleBuilder) Bold() *StyleBuilder {
	sb.styles = append(sb.styles, "font-bold")
	return sb
}

// Italic adds italic styling
func (sb *StyleBuilder) Italic() *StyleBuilder {
	sb.styles = append(sb.styles, "italic")
	return sb
}

// Underline adds underline styling
func (sb *StyleBuilder) Underline() *StyleBuilder {
	sb.styles = append(sb.styles, "underline")
	return sb
}

// Strikethrough adds strikethrough styling
func (sb *StyleBuilder) Strikethrough() *StyleBuilder {
	sb.styles = append(sb.styles, "line-through")
	return sb
}

// Color adds text color
func (sb *StyleBuilder) Color(color string) *StyleBuilder {
	if strings.HasPrefix(color, "#") {
		sb.styles = append(sb.styles, "text-["+color+"]")
	} else {
		sb.styles = append(sb.styles, "text-"+color)
	}
	return sb
}

// Background adds background color
func (sb *StyleBuilder) Background(color string) *StyleBuilder {
	if strings.HasPrefix(color, "#") {
		sb.styles = append(sb.styles, "bg-["+color+"]")
	} else {
		sb.styles = append(sb.styles, "bg-"+color)
	}
	return sb
}

// Faint adds faint/dim styling
func (sb *StyleBuilder) Faint() *StyleBuilder {
	sb.styles = append(sb.styles, "opacity-60")
	return sb
}

// Success adds success styling
func (sb *StyleBuilder) Success() *StyleBuilder {
	return sb.Color("green-600")
}

// Error adds error styling
func (sb *StyleBuilder) Error() *StyleBuilder {
	return sb.Color("red-600")
}

// Warning adds warning styling
func (sb *StyleBuilder) Warning() *StyleBuilder {
	return sb.Color("yellow-600")
}

// Info adds info styling
func (sb *StyleBuilder) Info() *StyleBuilder {
	return sb.Color("blue-600")
}

// Muted adds muted styling
func (sb *StyleBuilder) Muted() *StyleBuilder {
	return sb.Color("gray-500")
}

// Uppercase adds uppercase text transform
func (sb *StyleBuilder) Uppercase() *StyleBuilder {
	sb.styles = append(sb.styles, "uppercase")
	return sb
}

// Lowercase adds lowercase text transform
func (sb *StyleBuilder) Lowercase() *StyleBuilder {
	sb.styles = append(sb.styles, "lowercase")
	return sb
}

// Capitalize adds capitalize text transform
func (sb *StyleBuilder) Capitalize() *StyleBuilder {
	sb.styles = append(sb.styles, "capitalize")
	return sb
}

// Custom adds a custom style class
func (sb *StyleBuilder) Custom(style string) *StyleBuilder {
	sb.styles = append(sb.styles, style)
	return sb
}

// Build returns the style string
func (sb *StyleBuilder) Build() string {
	return strings.Join(sb.styles, " ")
}

// Predefined style shortcuts

// SuccessText creates a success-styled text
func SuccessText(content string) Text {
	return NewText(content).Success().Build()
}

// ErrorText creates an error-styled text
func ErrorText(content string) Text {
	return NewText(content).Error().Build()
}

// WarningText creates a warning-styled text
func WarningText(content string) Text {
	return NewText(content).Warning().Build()
}

// InfoText creates an info-styled text
func InfoText(content string) Text {
	return NewText(content).Info().Build()
}

// MutedText creates a muted-styled text
func MutedText(content string) Text {
	return NewText(content).Muted().Build()
}

// BoldText creates bold text
func BoldText(content string) Text {
	return NewText(content).Bold().Build()
}

// ItalicText creates italic text
func ItalicText(content string) Text {
	return NewText(content).Italic().Build()
}

// Status creates status-styled text based on status string
func StatusText(status, content string) Text {
	switch strings.ToUpper(status) {
	case "PASS", "SUCCESS", "OK":
		return SuccessText(content)
	case "FAIL", "FAILED", "ERROR":
		return ErrorText(content)
	case "WARN", "WARNING":
		return WarningText(content)
	case "SKIP", "SKIPPED":
		return MutedText(content)
	case "INFO":
		return InfoText(content)
	default:
		return NewText(content).Build()
	}
}
