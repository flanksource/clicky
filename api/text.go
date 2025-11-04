package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"
)

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

func (t Text) MarshalJSON() ([]byte, error) {
	var s = t.Content

	for _, child := range t.Children {
		s += child.String()
	}
	return json.Marshal(s)

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
	Unstyled  bool     // Whether to render without any bullet or numbering
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

func (t Text) HR() Text {
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

	args = lo.Map(args, func(i any, _ int) any {
		return Human(i)
	})
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

func Map[T any](m map[string]T, styles ...string) DescriptionList {
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

// TextList is a list of Textable items that can be rendered to multiple formats.
// Use JoinNewlines() to create a single Textable that joins all items with newlines.
type TextList []Textable

func (tl TextList) Strings() []string {
	result := make([]string, len(tl))
	for i, item := range tl {
		result[i] = item.String()
	}
	return result
}

// JoinNewlines joins all items with newlines and returns a single Textable.
// This is the primary method for rendering a TextList - call .ANSI(), .HTML(), or .Markdown() on the result.
func (tl TextList) JoinNewlines() Textable {
	if len(tl) == 0 {
		return Text{}
	}

	result := Text{}
	for i, item := range tl {
		if i > 0 {
			// Add newline between items
			result = result.NewLine()
		}
		result = result.Add(item)
	}
	return result
}

// Indent returns a new TextList with all items indented by one level (one tab).
// The indentation is prepended to the beginning of each item when rendered.
func (tl TextList) Indent() TextList {
	indented := make(TextList, len(tl))
	for i, item := range tl {
		// Wrap item in a Text that prepends a tab
		indented[i] = Text{Content: "\t"}.Add(item)
	}
	return indented
}

func (tl TextList) String() string {
	return tl.JoinNewlines().String()
}
func (tl TextList) AsANSI() []string {
	result := make([]string, len(tl))
	for i, item := range tl {
		result[i] = item.ANSI()
	}
	return result
}

func (tl TextList) AsHTML() []string {
	result := make([]string, len(tl))
	for i, item := range tl {
		result[i] = item.HTML()
	}
	return result
}

func (tl TextList) AsMarkdown() []string {
	result := make([]string, len(tl))
	for i, item := range tl {
		result[i] = item.Markdown()
	}
	return result
}

func (tl TextList) AsString() []string {
	result := make([]string, len(tl))
	for i, item := range tl {
		result[i] = item.String()
	}
	return result
}

func (tl TextList) ANSI() string {
	return tl.JoinNewlines().ANSI()
}
func (tl TextList) HTML() string {
	return tl.JoinNewlines().HTML()
}
func (tl TextList) Markdown() string {
	return tl.JoinNewlines().Markdown()
}
