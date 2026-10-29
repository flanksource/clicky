package api

// Dialect selects the markdown flavour to emit. It exists because "markdown"
// is not one target: the same document is consumed by GitHub-flavoured
// renderers, by Slack, and by MDX compilers, and they disagree about how
// styling and emphasis are expressed.
type Dialect int

const (
	// DialectGFM is GitHub-Flavored Markdown, the zero value. Styling is
	// emitted as inline-CSS HTML spans.
	DialectGFM Dialect = iota
	// DialectMDX emits JSX: styling becomes `className` carrying the source
	// Tailwind classes, because MDX parses raw HTML as JSX, where a string
	// `style` attribute is a compile error and the attribute is `className`.
	DialectMDX
	// DialectSlack emits Slack's mrkdwn, whose emphasis markers differ.
	DialectSlack
)

type MarkdownOptions struct {
	NoColor   bool
	Dialect   Dialect
	Unbounded bool
}

type MarkdownWithOptions interface {
	MarkdownWithOptions(MarkdownOptions) string
}

func RenderMarkdown(value Textable, options MarkdownOptions) string {
	if value == nil {
		return ""
	}
	if renderer, ok := value.(MarkdownWithOptions); ok {
		return renderer.MarkdownWithOptions(options)
	}
	return value.Markdown()
}

func (tv TypedValue) MarkdownWithOptions(options MarkdownOptions) string {
	return RenderMarkdown(tv.Value(), options)
}

func (tl TypedList) MarkdownWithOptions(options MarkdownOptions) string {
	return RenderMarkdown(tl.Value(), options)
}

func (tm TypedMap) MarkdownWithOptions(options MarkdownOptions) string {
	return RenderMarkdown(tm.Value(), options)
}

func (tm TextMap) MarkdownWithOptions(options MarkdownOptions) string {
	return RenderMarkdown(tm.Value(), options)
}

func (tl TextList) MarkdownWithOptions(options MarkdownOptions) string {
	return RenderMarkdown(tl.JoinNewlines(), options)
}

func (l List) MarkdownWithOptions(options MarkdownOptions) string {
	return l.renderList(func(value Textable) string { return RenderMarkdown(value, options) }, blockTextableString(l.Bullet))
}
