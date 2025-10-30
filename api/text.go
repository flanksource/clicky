package api

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	commonsText "github.com/flanksource/commons/text"
	"github.com/muesli/termenv"

	"github.com/flanksource/clicky/api/icons"
	"github.com/flanksource/clicky/api/tailwind"
)

func Human(content any, styles ...string) Text {
	switch t := content.(type) {
	case Text:
		return t
	case Textable:
		return Text{}.Add(t)
	case time.Time, *time.Time:
		return Text{
			Content: t.(time.Time).Format(time.RFC3339),
			Style:   strings.Join(append(styles, "date"), " "),
		}
	case time.Duration:
		var v string
		if time.Duration(t.Seconds()) < 5*time.Second {
			v = fmt.Sprintf("%dms", t.Milliseconds())
		} else if time.Duration(t.Seconds()) < 1*time.Minute {
			v = fmt.Sprintf("%.2fs", t.Seconds())
		} else if time.Duration(t.Seconds()) < 1*time.Hour {
			v = fmt.Sprintf("%.1fm", t.Minutes())
		} else if time.Duration(t.Seconds()) < 24*time.Hour {
			v = fmt.Sprintf("%.1fh", t.Hours())
		} else {
			v = commonsText.HumanizeDuration(t)
		}
		return Text{
			Content: v,
			Style:   strings.Join(append(styles, "duration"), " "),
		}
	case *time.Duration:
		return Human(*t, styles...)
	case float32, float64:
		return Text{
			Content: fmt.Sprintf("%.2f", t),
			Style:   strings.Join(append(styles, "number"), " ")}

	case bool:
		if t {
			return Text{}.Add(icons.Success)
		} else {
			return Text{}.Add(icons.Fail)
		}
	}

	return Text{Content: fmt.Sprintf("%v", content), Style: strings.Join(styles, " ")}
}

type HtmlElement struct {
	Tag        string
	Attributes map[string]string
	Content    string
	Fallback   Textable
}

func (e HtmlElement) HTML() string {
	return fmt.Sprintf("<%s %s>%s</%s>", e.Tag, formatAttributes(e.Attributes), e.Content, e.Tag)
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

var BR = HtmlElement{
	Tag:      "br",
	Content:  "",
	Fallback: Text{Content: "\n"},
}

type Comment string

func (c Comment) String() string {
	return ""
}
func (c Comment) ANSI() string {
	return ""
}
func (c Comment) HTML() string {
	return fmt.Sprintf("<!--\n %s \n//-->", string(c))
}
func (c Comment) Markdown() string {
	return fmt.Sprintf("<!--\n %s \n//-->", string(c))
}

// Global cache for ResolveStyles to avoid repeated parsing
var (
	styleCache     = make(map[string]Class)
	styleCacheLock sync.RWMutex
)

// Textable interface defines the standard text rendering methods
// for any type that can be rendered to multiple output formats
type Textable interface {
	String() string   // Plain text representation
	ANSI() string     // ANSI colored terminal output
	HTML() string     // HTML formatted output
	Markdown() string // Markdown formatted output
}

// Text represents styled content that can be rendered to multiple output formats.
// It supports hierarchical structure through Children, CSS-compatible styling,
// and format-specific rendering (ANSI, HTML, Markdown).
type Text struct {
	Content  string
	Class    Class
	Style    string
	Children []Textable
	Tooltip  Textable
	indent   int // internal use for tracking indentation level
}

// Format implements fmt.Formatter to ensure sensitive values are redacted in all format verbs
func (t Text) Format(f fmt.State, verb rune) {
	f.Write([]byte(t.ANSI()))
	switch verb {
	case 's':
		f.Write([]byte(t.String()))
	default:
		f.Write([]byte(t.ANSI()))
	}
}

func (t Text) Human(o any, styles ...string) Text {
	return t.Add(Text{Content: fmt.Sprintf("%v", o), Style: strings.Join(styles, " ")})
}

func (t Text) WithStyles(styles ...string) Text {
	return t.Styles(styles...)
}

func (t Text) WithTooltip(tooltip Textable) Text {
	t.Tooltip = tooltip
	return t
}

func (t Text) Add(child Textable) Text {
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

type List struct {
	Items     []Textable
	Bullet    Textable // Bullet character or icon
	Numbered  bool     // Whether to use numbered list
	Ordered   bool     // Alias for Numbered
	Style     string   // Additional styles for the list container
	Spacing   int      // Spaces between bullet and content
	Indent    int
	MaxInline int // Max items to render inline, else vertical
}

func (l List) String() string {
	if len(l.Items) == 0 {
		return ""
	}
	var parts []string
	for _, item := range l.Items {

		parts = append(parts, item.String())
	}
	return strings.Join(parts, "")
}

func (l List) Markdown() string {
	if len(l.Items) == 0 {
		return ""
	}
	var parts []string
	for i, item := range l.Items {
		var bullet string
		if l.Numbered {
			bullet = fmt.Sprintf("%d. ", i+1)
		} else if l.Bullet != nil {
			bullet = l.Bullet.Markdown()
		}
		parts = append(parts, fmt.Sprintf("%s%s", bullet, item.Markdown()))
	}
	return strings.Join(parts, "\n")
}

func (l List) ANSI() string {
	if len(l.Items) == 0 {
		return ""
	}
	var parts []string
	for i, item := range l.Items {
		var bullet string
		if l.Numbered {
			bullet = fmt.Sprintf("%d. ", i+1)
		} else if l.Bullet != nil {
			bullet = l.Bullet.ANSI()
		}
		parts = append(parts, fmt.Sprintf("%s%s", bullet, item.ANSI()))
	}
	if len(parts) > l.MaxInline && l.MaxInline > 0 || len(parts) > 3 {
		return strings.Join(parts, "\n")
	}
	return strings.Join(parts, ", ")
}

func (l List) HTML() string {
	if len(l.Items) == 0 {
		return ""
	}
	var parts []string
	for _, item := range l.Items {
		parts = append(parts, item.HTML())
	}
	if len(parts) <= l.MaxInline {
		return strings.Join(parts, ",")
	}

	var tag string
	if l.Numbered {
		tag = "ol"
	} else {
		tag = "ul"
	}

	parts = []string{}
	for _, item := range l.Items {
		parts = append(parts, fmt.Sprintf("<li>%s</li>", item.HTML()))
	}
	return fmt.Sprintf("<%s>%s</%s>", tag, strings.Join(parts, ""), tag)
}

func (t Text) NewLine() Text {
	return t.Add(BR).Indent(t.indent)
}

// Text adds a new child Text with the specified content and styles.
func (t Text) Text(text string, styles ...string) Text {
	return t.Add(Text{Content: text, Style: strings.Join(styles, " ")})
}

// AddText convenience method for adding Text content as a child
func (t Text) AddText(content string, styles ...string) Text {
	return t.Add(Text{Content: content, Style: strings.Join(styles, " ")})
}

// AddIcon convenience method for adding icons as children
func (t Text) AddIcon(icon Textable, styles ...string) Text {
	return t.Add(icon)
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

// WrapSpace adds a space before and after the content
func (t Text) WrapSpace() Text {
	return t.Wrap(" ", " ")
}

// Wrap adds a prefix and suffix to the content
func (t Text) Wrap(prefix, suffix string, style ...string) Text {
	if len(style) > 0 {
		return t.Add(Text{Content: prefix, Style: style[0]}).
			Add(t).
			Add(Text{Content: suffix, Style: style[0]})
	}

	t.Content = prefix + t.Content + suffix
	return t
}

// Append adds a new child Text with the specified content and styles.
func (t Text) Appendf(format string, args ...interface{}) Text {
	return t.Append(fmt.Sprintf(format, args...))
}

func (t Text) Space() Text {
	return t.Append(" ")
}

// Append adds a new child Text with the specified content and styles.
func (t Text) Append(text any, styles ...string) Text {

	if text == nil {
		return t
	}

	switch v := text.(type) {
	case Pretty:
		t.Children = append(t.Children, v.Pretty())
	case Textable:
		t.Children = append(t.Children, v)
	case string:
		t.Children = append(t.Children, Text{Content: v, Style: strings.Join(styles, " ")})
	case time.Time, *time.Time, *time.Duration, time.Duration, float64, float32:
		t.Children = append(t.Children, Human(v, styles...))
	case map[string]any:
		t.Children = append(t.Children, Map(v, styles...))
	default:
		t.Children = append(t.Children, Text{Content: fmt.Sprintf("%v", text), Style: strings.Join(styles, " ")})
	}

	return t
}

// Indent adds spaces before every line in content and recursively indents children
// with additional spacing, creating proper hierarchical indentation.
func (t Text) Indent(spaces int) Text {
	if spaces <= 0 {
		return t
	}
	t.indent = spaces
	indentation := strings.Repeat(" ", spaces)

	// if strings.HasPrefix(t.Content, "\n") {
	// t.Content = indentation + t.Content
	// }
	t.Content = strings.ReplaceAll(t.Content, "\n", "\n"+indentation)
	for i := range t.Children {
		// Only indent if the child is a Text type that supports Indent
		if textChild, ok := t.Children[i].(Text); ok {
			t.Children[i] = textChild.Indent(spaces + 2)
		}
		// Icons and other Textable types don't need indentation
	}
	return t
}

// PrintfWithStyle formats arguments with special handling for float64 (2 decimal places)
// and time.Duration (human-readable format), appending the result as a styled child.
func (t Text) PrintfWithStyle(format, style string, args ...interface{}) Text {
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
		// Check if child is Text type and not empty
		if textChild, ok := child.(Text); ok {
			if !textChild.IsEmpty() {
				return false
			}
		} else {
			// For non-Text Textable types (like icons), check if they have content
			if child.String() != "" {
				return false
			}
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

// htmlEscapeString escapes special HTML characters for use in attributes
func htmlEscapeString(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

func ResolveStyles(styles ...string) Class {
	// Create cache key from all style strings
	cacheKey := strings.Join(styles, "|")

	// Check cache first
	styleCacheLock.RLock()
	if cached, ok := styleCache[cacheKey]; ok {
		styleCacheLock.RUnlock()
		return cached
	}
	styleCacheLock.RUnlock()

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

			// Apply font family
			if strings.HasPrefix(class, "font-family-") {
				fontName := strings.TrimPrefix(class, "font-family-")
				switch strings.ToLower(fontName) {
				case "arial":
					resolved.Font.Name = "Arial"
				case "times":
					resolved.Font.Name = "Times"
				case "helvetica":
					resolved.Font.Name = "Helvetica"
				case "courier":
					resolved.Font.Name = "Courier"
				case "georgia":
					resolved.Font.Name = "Georgia"
				case "verdana":
					resolved.Font.Name = "Verdana"
				default:
					// Allow custom font names (case-sensitive for exact match)
					resolved.Font.Name = fontName
				}
			}

			// Apply font weight
			switch class {
			case "bold", "font-bold", "font-semibold", "font-medium":
				resolved.Font.Bold = true
			case "font-normal":
				resolved.Font.Bold = false
			}

			// Apply font style
			switch class {
			case "italic", "font-italic":
				resolved.Font.Italic = true
			case "not-italic":
				resolved.Font.Italic = false
			}

			// Apply text decoration
			switch class {
			case "underline":
				resolved.Font.Underline = true
			case "no-underline":
				resolved.Font.Underline = false
			}

			if class == "line-through" || class == "strikethrough" {
				resolved.Font.Strikethrough = true
			}

			// Apply faint/opacity
			switch class {
			case "font-light", "font-thin", "font-extralight", "opacity-50", "opacity-75", "opacity-25":
				resolved.Font.Faint = true
			case "opacity-100":
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

				// Apply non-nil values, converting to Point type
				if top != nil {
					resolved.Padding.Top = NewPoint(*top)
				}
				if right != nil {
					resolved.Padding.Right = NewPoint(*right)
				}
				if bottom != nil {
					resolved.Padding.Bottom = NewPoint(*bottom)
				}
				if left != nil {
					resolved.Padding.Left = NewPoint(*left)
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

	// Store in cache before returning
	styleCacheLock.Lock()
	styleCache[cacheKey] = resolved
	styleCacheLock.Unlock()

	return resolved
}

// ApplyTailwindStyle processes Tailwind CSS classes and applies text transformations,
// returning both the transformed text and parsed style information.
func ApplyTailwindStyle(text, styleStr string) (string, TailwindStyle) {
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

// TailwindStyle contains parsed CSS styling information extracted from Tailwind classes.
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

func formatANSI(text string, style TailwindStyle) string {
	if text == "" {
		return ""
	}
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

// KeyValuePair represents a single key-value pair that can be rendered to multiple output formats.
// It supports two styles: "compact" (default, inline with minimal spacing) and "badge" (pill-shaped badges).
type KeyValuePair struct {
	Key   string
	Value any
	Style string // "compact" (default) or "badge"
}

func (kv KeyValuePair) String() string {
	return fmt.Sprintf("%s: %v", kv.Key, kv.Value)
}

func (kv KeyValuePair) ANSI() string {
	return Text{}.Append(kv.Key+": ", "text-muted").Add(Human(kv.Value, kv.Style)).ANSI()
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

func (kv KeyValuePair) Markdown() string {
	return fmt.Sprintf("**%s**: %v", kv.Key, kv.Value)
}

// DescriptionList represents a collection of key-value pairs rendered as an HTML description list.
// It supports two styles: "compact" (default, inline flex layout) and "badge" (pill-shaped badges).
type DescriptionList struct {
	Items []KeyValuePair
	Style string // "compact" (default) or "badge"
}

func (dl DescriptionList) String() string {
	if len(dl.Items) == 0 {
		return ""
	}
	var parts []string
	for _, item := range dl.Items {
		parts = append(parts, item.String())
	}
	return strings.Join(parts, ", ")
}

func (dl DescriptionList) ANSI() string {
	if len(dl.Items) == 0 {
		return ""
	}
	var parts []string
	for _, item := range dl.Items {
		parts = append(parts, item.ANSI())
	}
	return strings.Join(parts, ", ")
}

func (dl DescriptionList) HTML() string {
	if len(dl.Items) == 0 {
		return `<span class="text-gray-400">{}</span>`
	}

	// Determine style
	style := dl.Style
	if style == "" {
		style = "compact"
	}

	if strings.Contains(style, "badge") {
		// Badge style: wrap in flex container
		var parts []string
		for _, item := range dl.Items {
			// Ensure each item uses badge style
			item.Style = "badge"
			parts = append(parts, item.HTML())
		}
		return fmt.Sprintf(`<div class="inline-flex flex-wrap gap-2">%s</div>`, strings.Join(parts, ""))
	}

	// Compact style (default): description list with inline-flex
	var parts []string
	for _, item := range dl.Items {
		// Ensure each item uses compact style
		item.Style = "compact"
		parts = append(parts, item.HTML())
	}
	return fmt.Sprintf(`<dl class="inline-flex flex-wrap gap-x-4 gap-y-1">%s</dl>`, strings.Join(parts, ""))
}

func (dl DescriptionList) Markdown() string {
	if len(dl.Items) == 0 {
		return ""
	}
	var parts []string
	for _, item := range dl.Items {
		parts = append(parts, item.Markdown())
	}
	return strings.Join(parts, ", ")
}

func KeyValue(key string, value any, styles ...string) KeyValuePair {
	style := "compact"
	if len(styles) > 0 {
		style = strings.Join(styles, " ")
	}
	return KeyValuePair{
		Key:   key,
		Value: value,
		Style: style,
	}
}

func Map(m map[string]any, styles ...string) DescriptionList {
	style := "compact"
	if len(styles) > 0 {
		style = strings.Join(styles, " ")
	}

	// Sort keys for consistent ordering
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	items := make([]KeyValuePair, 0, len(m))
	for _, k := range keys {
		items = append(items, KeyValuePair{
			Key:   k,
			Value: m[k],
			Style: style,
		})
	}

	return DescriptionList{
		Items: items,
		Style: style,
	}
}
func CodeBlock(language, content string, styles ...string) Code {
	return Code{
		Content:  content,
		Language: mimeTypeToLanguage(language),
		Style:    strings.Join(styles, " "),
	}
}

func Clz(v bool, clz string, elseClz ...string) string {
	if v {
		return clz
	}
	if len(elseClz) > 0 {
		return strings.Join(elseClz, " ")
	}
	return ""
}

func HumanizeBytes(bytes int64) Text {
	return Text{
		Content: commonsText.HumanizeBytes(bytes),
	}
}

func mimeTypeToLanguage(mime string) string {
	switch {
	case strings.Contains(mime, "json"):
		return "json"
	case strings.Contains(mime, "xml"):
		return "xml"
	case strings.Contains(mime, "yaml") || strings.Contains(mime, "yml"):
		return "yaml"
	case strings.Contains(mime, "html"):
		return "html"
	case strings.Contains(mime, "text/html"):
		return "html"
	case strings.Contains(mime, "text/plain"):
		return "txt"
	case strings.Contains(mime, "javascript"):
		return "javascript"
	case strings.Contains(mime, "css"):
		return "css"
	case strings.Contains(mime, "csv"):
		return "csv"
	case strings.Contains(mime, "markdown") || strings.Contains(mime, "md"):
		return "markdown"
	case strings.Contains(mime, "sql"):
		return "sql"
	case strings.Contains(mime, "graphql"):
		return "graphql"
	case strings.Contains(mime, "python"):
		return "python"
	case strings.Contains(mime, "java"):
		return "java"
	}
	return ""

}
