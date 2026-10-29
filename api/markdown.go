package api

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api/tailwind"
	"github.com/flanksource/clicky/internal/markdownutil"
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
	result := t.markdownContent(options)
	for _, child := range t.Children {
		result += markdownTextableWithOptions(child, options)
	}
	return result
}

func (t Text) markdownContent(options MarkdownOptions) string {
	slack := options.Dialect == DialectSlack
	plainContent := t.Content

	var style TailwindStyle
	content := plainContent

	if t.Class != (Class{}) {
		style = classToTailwindStyle(t.Class)
	} else if t.Style != "" {
		transformedText, parsedStyle := ApplyTailwindStyle(plainContent, t.Style)
		style = parsedStyle
		if options.Unbounded {
			transformedText = tailwind.TransformText(plainContent, style.TextTransform)
		}
		if transformedText != plainContent {
			content = transformedText
		}
	} else {
		return markdownutil.Text(content)
	}

	result := markdownutil.Text(content)
	if result == "" {
		return ""
	}
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
