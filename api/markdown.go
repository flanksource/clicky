package api

import (
	"fmt"
	"strings"
)

func markdownTextable(t Textable, slack bool) string {
	return markdownTextableWithOptions(t, slack, MarkdownOptions{})
}

func markdownTextableWithOptions(t Textable, slack bool, options MarkdownOptions) string {
	if t == nil {
		return ""
	}
	if slack {
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
	return t.markdown(false, options)
}

func (t Text) MarkdownSlack() string {
	return t.markdown(true, MarkdownOptions{})
}

func (t Text) boldMD(text string, slack bool) string {
	if slack {
		return "*" + text + "*"
	}
	return "**" + text + "**"
}

func (t Text) markdown(slack bool, options MarkdownOptions) string {
	renderedContent := t.Content
	plainContent := t.Content
	for _, child := range t.Children {
		renderedContent += markdownTextableWithOptions(child, slack, options)
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

	if !options.NoColor && (style.Foreground != "" || style.Background != "") {
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

	return result
}
