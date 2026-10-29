package api

import (
	"slices"
	"strings"

	"github.com/flanksource/clicky/api/tailwind"
)

type badgeElement struct {
	HtmlElement
}

func (b badgeElement) Markdown() string {
	return b.MarkdownWithOptions(MarkdownOptions{})
}

func (b badgeElement) MarkdownWithOptions(options MarkdownOptions) string {
	if options.Dialect == DialectGFM {
		if !options.NoColor {
			return b.HTML()
		}
		element := b.HtmlElement
		colors := tailwind.ColorClasses(element.Attributes["class"])
		classes := slices.DeleteFunc(strings.Fields(element.Attributes["class"]), func(class string) bool { return slices.Contains(colors, class) })
		class := strings.Join(append(classes, "border"), " ")
		element.Attributes = map[string]string{"class": class, "style": tailwindInlineCSS(class)}
		return element.HTML()
	}
	options.NoColor = options.NoColor || options.Dialect == DialectSlack
	return RenderMarkdown(b.Fallback, options)
}

func (b badgeElement) MarkdownSlack() string {
	return b.MarkdownWithOptions(MarkdownOptions{Dialect: DialectSlack})
}
