package api

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/flanksource/clicky/api/tailwind"
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

// StaticHTMLProvider is implemented by types that can render pure HTML without JavaScript.
// Used by Collapsed to avoid embedding scripts (e.g. Grid.js) inside <template x-if>
// where they won't execute.
type StaticHTMLProvider interface {
	StaticHTML() string
}

// DetailLevel controls how much of an entity's content Details renders.
// Higher levels produce richer output; callers pick the level based on
// verbosity flags or request query params.
type DetailLevel int

const (
	// DetailSummary is the one-line identity view, equivalent to Pretty().
	DetailSummary DetailLevel = iota
	// DetailStandard is the default detail-page view: headline fields,
	// labels, tags, properties, locations, metadata — but no heavy bodies.
	DetailStandard
	// DetailFull includes everything, including raw config bodies and
	// other potentially large sections.
	DetailFull
)

// Detailable is implemented by types that render a multi-section detail view.
// The ctx carries DB / HTTP handles for lazy lookups (joined tables, counts);
// implementations type-assert it to their richer context type (e.g.
// duty/context.Context) and degrade gracefully when the assertion fails.
type Detailable interface {
	Details(ctx context.Context, level DetailLevel) Textable
}

func CompactList[T any](items []T) Textable {
	list := List{
		MaxInline: 3,
	}
	for _, item := range items {
		list.Items = append(list.Items, Human(item))
	}
	return list
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
	switch verb {
	case 's':
		_, _ = f.Write([]byte(t.String()))
	default:
		_, _ = f.Write([]byte(t.ANSI()))
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

func (l List) MarkdownSlack() string {
	if len(l.Items) == 0 {
		return ""
	}
	var parts []string
	for i, item := range l.Items {
		var bullet string
		if l.Numbered {
			bullet = fmt.Sprintf("%d. ", i+1)
		} else if l.Bullet != nil {
			bullet = markdownTextable(l.Bullet, true)
		}
		parts = append(parts, fmt.Sprintf("%s%s", bullet, markdownTextable(item, true)))
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
	return t.Add(HR).Indent(t.indent)
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

func uniqueStyles(existing string, styles ...string) string {
	styleSet := make(map[string]struct{})
	if existing != "" {
		for _, s := range strings.Split(existing, " ") {
			styleSet[s] = struct{}{}
		}
	}
	for _, style := range styles {
		for _, s := range strings.Split(style, " ") {
			if s != "" {
				styleSet[s] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(styleSet))
	for s := range styleSet {
		result = append(result, s)
	}
	sort.Strings(result)
	return strings.Join(result, " ")
}

func (t Text) Styles(classes ...string) Text {
	t.Style = uniqueStyles(t.Style, classes...)
	return t
}

// AppendStyle merges the given Tailwind class names into t.Style without
// duplicating classes that are already present.
func (t Text) AppendStyle(classes ...string) Text {
	t.Style = uniqueStyles(t.Style, classes...)
	return t
}

// WrapSpace adds a space before and after the content
func (t Text) WrapSpace() Text {
	return t.Wrap(" ", " ")
}

// Wrap adds a prefix and suffix to the content
// Wrap returns a new Text consisting of `prefix`, the receiver `t`, and
// `suffix`. When a style is supplied, the prefix and suffix are placed in
// dedicated child Text nodes carrying that style; the receiver `t` is left
// untouched. (The earlier implementation called `t.Add(t)` after appending the
// prefix, which doubled the receiver in the output.)
func (t Text) Wrap(prefix, suffix string, style ...string) Text {
	if len(style) > 0 {
		return Text{}.
			Add(Text{Content: prefix, Style: style[0]}).
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
	return t.Append(NBSP)
}

func (t Text) Tab() Text {
	return t.Append(TAB)
}

func (t Text) IsSpace() bool {
	return strings.TrimSpace(t.Content) == ""
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
		style := strings.Join(styles, " ")
		v = tailwind.TruncateForAppend(v, style)
		for i, line := range strings.Split(v, "\n") {
			if i > 0 {
				t.Children = append(t.Children, BR)
			}
			t.Children = append(t.Children, Text{Content: line, Style: style})
		}
	case time.Time, *time.Time, *time.Duration, time.Duration, float64, float32:
		t.Children = append(t.Children, Human(v, styles...))
	case map[string]any:
		t.Children = append(t.Children, Map(v, styles...))
	case map[string]string:
		t.Children = append(t.Children, Map(v, styles...))
	case []map[string]any:
		for _, item := range v {
			t.Children = append(t.Children, Map(item, styles...))
		}
	case []map[string]string:
		for _, item := range v {
			t.Children = append(t.Children, Map(item, styles...))
		}
	case []any:
		for _, item := range v {
			t.Children = append(t.Children, Human(item, styles...))
		}
	case []string:
		for _, item := range v {
			t.Children = append(t.Children, Text{Content: item, Style: strings.Join(styles, " ")})
		}

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

func (kv KeyValuePair) IsEmpty() bool {
	return kv.Value == nil || fmt.Sprintf("%v", kv.Value) == ""
}

func (kv KeyValuePair) String() string {
	if kv.IsEmpty() {
		return ""
	}
	return fmt.Sprintf("%s: %v", kv.Key, kv.Value)
}

func (kv KeyValuePair) ANSI() string {
	if kv.IsEmpty() {
		return ""
	}
	return Text{}.Append(kv.Key+": ", "text-muted").Add(Human(kv.Value, kv.Style)).ANSI()
}

func (kv KeyValuePair) Markdown() string {
	if kv.IsEmpty() {
		return ""
	}
	return fmt.Sprintf("**%s**: %v", kv.Key, kv.Value)
}

func (kv KeyValuePair) MarkdownSlack() string {
	if kv.IsEmpty() {
		return ""
	}
	return fmt.Sprintf("%s: %v", Text{}.boldMD(kv.Key, true), kv.Value)
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
	parts := make([]string, 0, len(dl.Items))
	for _, item := range dl.Items {
		parts = append(parts, item.String())
	}
	return joinDescriptionParts(parts)
}

func (dl DescriptionList) ANSI() string {
	if len(dl.Items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(dl.Items))
	for _, item := range dl.Items {
		parts = append(parts, item.ANSI())
	}
	return joinDescriptionParts(parts)
}

// joinDescriptionParts renders pre-formatted key/value parts inline (", "
// separated) for small lists that fit the terminal, and stacks them one per
// line otherwise. Inline joining is fine for a handful of pairs, but a large
// map (e.g. a 493-entry activity Math map) becomes a single multi-thousand-char
// line. When that line is the widest in a tree, lipgloss right-pads every
// sibling line to match — flooding the terminal with trailing-space "blank"
// rows. Stacking removes the giant line so nothing forces that padding.
func joinDescriptionParts(parts []string) string {
	inline := strings.Join(parts, ", ")
	if len(parts) <= 3 && ansi.StringWidth(inline) <= GetTerminalWidth() {
		return inline
	}
	return strings.Join(parts, "\n")
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

func (dl DescriptionList) MarkdownSlack() string {
	if len(dl.Items) == 0 {
		return ""
	}
	var parts []string
	for _, item := range dl.Items {
		parts = append(parts, markdownTextable(item, true))
	}
	return strings.Join(parts, ", ")
}

func KeyValue(key string, value any, styles ...string) KeyValuePair {
	if value == nil || fmt.Sprintf("%v", value) == "" {
		return KeyValuePair{}
	}
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

// mimeTypeToLanguage maps an HTTP-style MIME type ("application/json",
// "text/x-typescript", etc.) to the chroma lexer name. For inputs that look
// like plain language identifiers ("typescript", "go", "sql") it falls back
// to normalizeLanguage so callers can pass either form.
func mimeTypeToLanguage(mime string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	if !strings.Contains(mime, "/") {
		return normalizeLanguage(mime)
	}
	switch {
	case strings.Contains(mime, "json"):
		return "json"
	case strings.Contains(mime, "xml"):
		return "xml"
	case strings.Contains(mime, "yaml") || strings.Contains(mime, "yml"):
		return "yaml"
	case strings.Contains(mime, "html"):
		return "html"
	case strings.Contains(mime, "text/plain"):
		return "txt"
	case strings.Contains(mime, "typescript"):
		return "typescript"
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
func (tl TextList) JoinNewlines() Text {
	return tl.Join(BR)
}

func (tl TextList) Join(sep ...Textable) Text {
	if len(tl) == 0 {
		return Text{}
	}

	result := Text{}
	for i, item := range tl {
		if i > 0 && len(sep) > 0 {
			// Add separator between items
			result = result.Add(sep[0])
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

func (tl TextList) MarkdownSlack() string {
	return tl.JoinNewlines().MarkdownSlack()
}

// ExtractOrderValue extracts the Tailwind order-X class value from a style string.
// Returns 0 for columns without order-X (they appear first).
// Supports order-1 through order-12 (standard Tailwind range).
func ExtractOrderValue(style string) int {
	if style == "" {
		return 0
	}

	// Split style string into individual classes
	classes := strings.Fields(style)
	for _, class := range classes {
		// Check if this is an order-X class
		if strings.HasPrefix(class, "order-") {
			orderStr := strings.TrimPrefix(class, "order-")
			// Parse the order value
			var orderVal int
			if _, err := fmt.Sscanf(orderStr, "%d", &orderVal); err == nil {
				return orderVal
			}
		}
	}

	// No order class found, return 0 (appears first)
	return 0
}
