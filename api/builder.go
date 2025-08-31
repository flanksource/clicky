// Package api provides fluent builders for creating styled Text objects.
// TextBuilder and StyleBuilder use the builder pattern to construct rich text
// with Tailwind CSS-compatible styling for consistent formatting across output types.
package api

import (
	"strings"
)

// TextBuilder constructs Text objects using a fluent interface with chained styling methods.
type TextBuilder struct {
	text Text
}

// StyleBuilder accumulates CSS classes into a space-separated style string.
type StyleBuilder struct {
	styles []string
}

func NewText(content string) *TextBuilder {
	return &TextBuilder{
		text: Text{
			Content:  content,
			Children: make([]Text, 0),
		},
	}
}

func NewStyle() *StyleBuilder {
	return &StyleBuilder{
		styles: make([]string, 0),
	}
}

func (tb *TextBuilder) Content(content string) *TextBuilder {
	tb.text.Content = content
	return tb
}

func (tb *TextBuilder) Style(style string) *TextBuilder {
	tb.text.Style = style
	return tb
}

func (tb *TextBuilder) Bold() *TextBuilder {
	tb.addStyle("font-bold")
	return tb
}

func (tb *TextBuilder) Italic() *TextBuilder {
	tb.addStyle("italic")
	return tb
}

func (tb *TextBuilder) Underline() *TextBuilder {
	tb.addStyle("underline")
	return tb
}

func (tb *TextBuilder) Strikethrough() *TextBuilder {
	tb.addStyle("line-through")
	return tb
}

// Color accepts both hex colors (#FF0000) and named colors (red-600).
func (tb *TextBuilder) Color(color string) *TextBuilder {
	if strings.HasPrefix(color, "#") {
		tb.addStyle("text-[" + color + "]")
	} else {
		tb.addStyle("text-" + color)
	}
	return tb
}

// Background accepts both hex colors (#FF0000) and named colors (red-600).
func (tb *TextBuilder) Background(color string) *TextBuilder {
	if strings.HasPrefix(color, "#") {
		tb.addStyle("bg-[" + color + "]")
	} else {
		tb.addStyle("bg-" + color)
	}
	return tb
}

func (tb *TextBuilder) Faint() *TextBuilder {
	tb.addStyle("opacity-60")
	return tb
}

func (tb *TextBuilder) Success() *TextBuilder {
	return tb.Color("green-600")
}

func (tb *TextBuilder) Error() *TextBuilder {
	return tb.Color("red-600")
}

func (tb *TextBuilder) Warning() *TextBuilder {
	return tb.Color("yellow-600")
}

func (tb *TextBuilder) Info() *TextBuilder {
	return tb.Color("blue-600")
}

func (tb *TextBuilder) Muted() *TextBuilder {
	return tb.Color("gray-500")
}

func (tb *TextBuilder) Uppercase() *TextBuilder {
	tb.addStyle("uppercase")
	return tb
}

func (tb *TextBuilder) Lowercase() *TextBuilder {
	tb.addStyle("lowercase")
	return tb
}

func (tb *TextBuilder) Capitalize() *TextBuilder {
	tb.addStyle("capitalize")
	return tb
}

func (tb *TextBuilder) Child(child Text) *TextBuilder {
	tb.text.Children = append(tb.text.Children, child)
	return tb
}

func (tb *TextBuilder) ChildBuilder(childBuilder *TextBuilder) *TextBuilder {
	tb.text.Children = append(tb.text.Children, childBuilder.Build())
	return tb
}

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

func (sb *StyleBuilder) Bold() *StyleBuilder {
	sb.styles = append(sb.styles, "font-bold")
	return sb
}

func (sb *StyleBuilder) Italic() *StyleBuilder {
	sb.styles = append(sb.styles, "italic")
	return sb
}

func (sb *StyleBuilder) Underline() *StyleBuilder {
	sb.styles = append(sb.styles, "underline")
	return sb
}

func (sb *StyleBuilder) Strikethrough() *StyleBuilder {
	sb.styles = append(sb.styles, "line-through")
	return sb
}

func (sb *StyleBuilder) Color(color string) *StyleBuilder {
	if strings.HasPrefix(color, "#") {
		sb.styles = append(sb.styles, "text-["+color+"]")
	} else {
		sb.styles = append(sb.styles, "text-"+color)
	}
	return sb
}

func (sb *StyleBuilder) Background(color string) *StyleBuilder {
	if strings.HasPrefix(color, "#") {
		sb.styles = append(sb.styles, "bg-["+color+"]")
	} else {
		sb.styles = append(sb.styles, "bg-"+color)
	}
	return sb
}

func (sb *StyleBuilder) Faint() *StyleBuilder {
	sb.styles = append(sb.styles, "opacity-60")
	return sb
}

func (sb *StyleBuilder) Success() *StyleBuilder {
	return sb.Color("green-600")
}

func (sb *StyleBuilder) Error() *StyleBuilder {
	return sb.Color("red-600")
}

func (sb *StyleBuilder) Warning() *StyleBuilder {
	return sb.Color("yellow-600")
}

func (sb *StyleBuilder) Info() *StyleBuilder {
	return sb.Color("blue-600")
}

func (sb *StyleBuilder) Muted() *StyleBuilder {
	return sb.Color("gray-500")
}

func (sb *StyleBuilder) Uppercase() *StyleBuilder {
	sb.styles = append(sb.styles, "uppercase")
	return sb
}

func (sb *StyleBuilder) Lowercase() *StyleBuilder {
	sb.styles = append(sb.styles, "lowercase")
	return sb
}

func (sb *StyleBuilder) Capitalize() *StyleBuilder {
	sb.styles = append(sb.styles, "capitalize")
	return sb
}

func (sb *StyleBuilder) Custom(style string) *StyleBuilder {
	sb.styles = append(sb.styles, style)
	return sb
}

func (sb *StyleBuilder) Build() string {
	return strings.Join(sb.styles, " ")
}

// SuccessText creates a green-styled text for positive states.
func SuccessText(content string) Text {
	return NewText(content).Success().Build()
}

// ErrorText creates a red-styled text for error states.
func ErrorText(content string) Text {
	return NewText(content).Error().Build()
}

// WarningText creates a yellow-styled text for warning states.
func WarningText(content string) Text {
	return NewText(content).Warning().Build()
}

// InfoText creates a blue-styled text for informational content.
func InfoText(content string) Text {
	return NewText(content).Info().Build()
}

// MutedText creates a gray-styled text for secondary content.
func MutedText(content string) Text {
	return NewText(content).Muted().Build()
}

func BoldText(content string) Text {
	return NewText(content).Bold().Build()
}

func ItalicText(content string) Text {
	return NewText(content).Italic().Build()
}

// StatusText applies semantic styling based on status keywords
// (PASS/SUCCESS/OK -> green, FAIL/ERROR -> red, WARN -> yellow, etc.).
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
