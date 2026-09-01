package api

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api/tailwind"
)

func markdownTextable(t Textable, slack bool) string {
	options := MarkdownOptions{}
	if slack {
		options.Dialect = DialectSlack
	}
	return markdownTextableWithOptions(t, options)
}

func markdownTextableWithOptions(t Textable, options MarkdownOptions) string {
	if t == nil {
		return ""
	}
	if options.Dialect == DialectSlack {
		if v, ok := t.(interface{ MarkdownSlack() string }); ok {
			return v.MarkdownSlack()
		}
	}
	return RenderMarkdown(t, options)
}

func (t Text) Markdown() string {
	return t.MarkdownWithOptions(MarkdownOptions{})
}

func (t Text) MarkdownWithOptions(options MarkdownOptions) string {
	return t.markdown(options)
}

func (t Text) MarkdownSlack() string {
	return t.markdown(MarkdownOptions{Dialect: DialectSlack})
}

func (t Text) boldMD(text string, slack bool) string {
	if slack {
		return "*" + text + "*"
	}
	return "**" + text + "**"
}

func (t Text) markdown(options MarkdownOptions) string {
	slack := options.Dialect == DialectSlack
	renderedContent := t.Content
	plainContent := t.Content
	for _, child := range t.Children {
		renderedContent += markdownTextableWithOptions(child, options)
		plainContent += child.String()
	}

	var style TailwindStyle
	content := renderedContent

	if t.Class != (Class{}) {
		style = classToTailwindStyle(t.Class)
	} else if t.Style != "" {
		transformedText, parsedStyle := ApplyTailwindStyle(plainContent, t.Style)
		style = parsedStyle
		if transformedText != plainContent {
			content = transformedText
		}
	} else {
		return renderedContent
	}

	result := content
	if style.Bold {
		result = t.boldMD(result, slack)
	}
	if style.Italic {
		result = "*" + result + "*"
	}
	if style.Strikethrough {
		result = "~~" + result + "~~"
	}

	if options.NoColor || (style.Foreground == "" && style.Background == "") {
		return result
	}

	// MDX parses raw HTML as JSX, where a string `style` attribute is a compile
	// error and the attribute is `className`. Forward the source classes rather
	// than the resolved hex — there is no hex-to-class reverse mapping, and the
	// class is what a downstream Tailwind build needs. Only the colour classes
	// go: the rest of the style string is already spent (see ColorClasses).
	if options.Dialect == DialectMDX {
		if classes := tailwind.ColorClasses(t.Style); len(classes) > 0 {
			return fmt.Sprintf(`<span className="%s">%s</span>`, strings.Join(classes, " "), result)
		}
		// A Class-styled Text carries resolved colours and no source class, so
		// there is nothing MDX-safe to emit; the text keeps its emphasis.
		return result
	}

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
	return fmt.Sprintf(`<span style="%s">%s</span>`, strings.Join(styles, "; "), result)
}
