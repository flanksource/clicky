package markdown

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
	"gopkg.in/yaml.v3"
)

const DocumentVersion = 1

// Document is a parsed markdown document that can render through Clicky's
// Textable formats and export as structured Clicky document JSON.
type Document struct {
	Version  int            `json:"version"`
	Filename string         `json:"filename,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Root     Node           `json:"root"`
}

// Node is the intermediate semantic tree produced from Goldmark's AST.
type Node struct {
	Kind       string            `json:"kind"`
	Text       string            `json:"text,omitempty"`
	Children   []Node            `json:"children,omitempty"`
	Items      []Node            `json:"items,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Level      int               `json:"level,omitempty"`
	Language   string            `json:"language,omitempty"`
	Source     string            `json:"source,omitempty"`
	RawHTML    string            `json:"html,omitempty"`
	Href       string            `json:"href,omitempty"`
	Title      string            `json:"title,omitempty"`
	ID         string            `json:"id,omitempty"`
	Severity   string            `json:"severity,omitempty"`
	Checked    *bool             `json:"checked,omitempty"`
	Ordered    bool              `json:"ordered,omitempty"`
	Align      string            `json:"align,omitempty"`
	LineStart  int               `json:"lineStart,omitempty"`
	LineEnd    int               `json:"lineEnd,omitempty"`
}

func (d *Document) HTML() string {
	if d == nil {
		return ""
	}
	return d.Root.HTML()
}

func (d *Document) Markdown() string {
	if d == nil {
		return ""
	}
	body := d.Root.Markdown()
	frontmatter := documentFrontmatter(d.Metadata, d.Filename)
	if frontmatter == "" {
		return body
	}
	if body == "" {
		return frontmatter
	}
	return frontmatter + "\n\n" + body
}

func (n Node) String() string {
	switch n.Kind {
	case "document":
		return strings.TrimSpace(joinNodeStrings(n.Children, "\n"))
	case "paragraph", "heading", "emphasis", "strong", "strike", "link", "image", "table_cell":
		return strings.TrimSpace(joinNodeStrings(n.Children, ""))
	case "text", "code", "raw-html", "html":
		if n.Text != "" {
			return n.Text
		}
		if n.Source != "" {
			return n.Source
		}
		return n.RawHTML
	case "linebreak":
		return "\n"
	case "code_block":
		return n.Source
	case "list":
		return strings.TrimSpace(joinNodeStrings(n.Items, "\n"))
	case "list_item":
		return strings.TrimSpace(joinNodeStrings(n.Children, ""))
	case "blockquote":
		return strings.TrimSpace(joinNodeStrings(n.Children, "\n"))
	case "admonition":
		parts := []string{strings.TrimSpace(n.Title)}
		if body := strings.TrimSpace(joinNodeStrings(n.Children, "\n")); body != "" {
			parts = append(parts, body)
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case "collapsed":
		return strings.TrimSpace(n.Title + "\n" + joinNodeStrings(n.Children, "\n"))
	case "thematic_break":
		return ""
	case "table":
		return tableString(n, false)
	case "table_row":
		return strings.TrimSpace(joinNodeStrings(n.Children, " "))
	case "footnote_ref":
		return n.ID
	case "footnote":
		return strings.TrimSpace(joinNodeStrings(n.Children, "\n"))
	case "footnotes":
		return strings.TrimSpace(joinNodeStrings(n.Items, "\n"))
	default:
		if len(n.Children) > 0 {
			return strings.TrimSpace(joinNodeStrings(n.Children, ""))
		}
		return n.Text
	}
}

func (n Node) HTML() string {
	switch n.Kind {
	case "document":
		return joinNodeHTML(n.Children, "\n")
	case "paragraph":
		return "<p>" + joinNodeHTML(n.Children, "") + "</p>"
	case "heading":
		level := clampHeading(n.Level)
		return fmt.Sprintf("<h%d>%s</h%d>", level, joinNodeHTML(n.Children, ""), level)
	case "text":
		return html.EscapeString(n.Text)
	case "linebreak":
		return "<br />"
	case "emphasis":
		return "<em>" + joinNodeHTML(n.Children, "") + "</em>"
	case "strong":
		return "<strong>" + joinNodeHTML(n.Children, "") + "</strong>"
	case "strike":
		return "<s>" + joinNodeHTML(n.Children, "") + "</s>"
	case "code":
		return "<code>" + html.EscapeString(n.Text) + "</code>"
	case "code_block":
		code := api.NewCode(n.Source, n.Language)
		rendered := code.HTML()
		if strings.Contains(rendered, "<pre") {
			return rendered
		}
		langClass := ""
		if n.Language != "" {
			langClass = ` class="language-` + html.EscapeString(n.Language) + `"`
		}
		return "<pre><code" + langClass + ">" + rendered + "</code></pre>"
	case "link":
		attrs := ` href="` + html.EscapeString(n.Href) + `"`
		if n.Title != "" {
			attrs += ` title="` + html.EscapeString(n.Title) + `"`
		}
		return "<a" + attrs + ">" + joinNodeHTML(n.Children, "") + "</a>"
	case "image":
		alt := html.EscapeString(n.String())
		attrs := ` src="` + html.EscapeString(n.Href) + `" alt="` + alt + `"`
		if n.Title != "" {
			attrs += ` title="` + html.EscapeString(n.Title) + `"`
		}
		return "<img" + attrs + " />"
	case "list":
		tag := "ul"
		if n.Ordered {
			tag = "ol"
		}
		return "<" + tag + ">" + joinNodeHTML(n.Items, "") + "</" + tag + ">"
	case "list_item":
		prefix := ""
		if n.Checked != nil {
			checked := ""
			if *n.Checked {
				checked = " checked"
			}
			prefix = `<input type="checkbox" disabled` + checked + ` /> `
		}
		return "<li>" + prefix + joinNodeHTML(n.Children, "") + "</li>"
	case "blockquote":
		return "<blockquote>" + joinNodeHTML(n.Children, "\n") + "</blockquote>"
	case "admonition":
		title := html.EscapeString(n.Title)
		if title == "" {
			title = html.EscapeString(n.Severity)
		}
		return fmt.Sprintf(`<div class="admonition admonition-%s"><p>%s</p>%s</div>`,
			html.EscapeString(n.Severity), title, joinNodeHTML(n.Children, "\n"))
	case "collapsed":
		return "<details><summary>" + html.EscapeString(n.Title) + "</summary>\n" + joinNodeHTML(n.Children, "\n") + "\n</details>"
	case "thematic_break":
		return "<hr />"
	case "table":
		return tableHTML(n)
	case "raw-html", "html":
		return n.RawHTML
	case "footnote_ref":
		id := html.EscapeString(n.ID)
		return `<sup id="fnref:` + id + `"><a href="#fn:` + id + `">` + id + `</a></sup>`
	case "footnote":
		id := html.EscapeString(n.ID)
		return `<li id="fn:` + id + `">` + joinNodeHTML(n.Children, "\n") + `</li>`
	case "footnotes":
		return `<section class="footnotes"><ol>` + joinNodeHTML(n.Items, "") + `</ol></section>`
	default:
		return joinNodeHTML(n.Children, "")
	}
}

func (n Node) Markdown() string {
	switch n.Kind {
	case "document":
		return strings.TrimSpace(joinNodeMarkdown(n.Children, "\n\n"))
	case "paragraph":
		return joinNodeMarkdown(n.Children, "")
	case "heading":
		level := clampHeading(n.Level)
		return strings.Repeat("#", level) + " " + joinNodeMarkdown(n.Children, "")
	case "text":
		return n.Text
	case "linebreak":
		return "  \n"
	case "emphasis":
		return "*" + joinNodeMarkdown(n.Children, "") + "*"
	case "strong":
		return "**" + joinNodeMarkdown(n.Children, "") + "**"
	case "strike":
		return "~~" + joinNodeMarkdown(n.Children, "") + "~~"
	case "code":
		return "`" + strings.ReplaceAll(n.Text, "`", "\\`") + "`"
	case "code_block":
		lang := n.Language
		return "```" + lang + "\n" + strings.TrimRight(n.Source, "\n") + "\n```"
	case "link":
		title := ""
		if n.Title != "" {
			title = ` "` + strings.ReplaceAll(n.Title, `"`, `\"`) + `"`
		}
		return "[" + joinNodeMarkdown(n.Children, "") + "](" + n.Href + title + ")"
	case "image":
		title := ""
		if n.Title != "" {
			title = ` "` + strings.ReplaceAll(n.Title, `"`, `\"`) + `"`
		}
		return "![" + joinNodeMarkdown(n.Children, "") + "](" + n.Href + title + ")"
	case "list":
		lines := make([]string, 0, len(n.Items))
		for i, item := range n.Items {
			bullet := "- "
			if n.Ordered {
				bullet = fmt.Sprintf("%d. ", i+1)
			}
			lines = append(lines, bullet+indentContinuation(item.Markdown(), len(bullet)))
		}
		return strings.Join(lines, "\n")
	case "list_item":
		prefix := ""
		if n.Checked != nil {
			if *n.Checked {
				prefix = "[x] "
			} else {
				prefix = "[ ] "
			}
		}
		return prefix + strings.TrimSpace(joinNodeMarkdown(n.Children, "\n"))
	case "blockquote":
		return quoteLines(joinNodeMarkdown(n.Children, "\n\n"), "> ")
	case "admonition":
		head := "!!! " + n.Severity
		if n.Title != "" {
			head += " " + n.Title
		}
		body := strings.TrimSpace(joinNodeMarkdown(n.Children, "\n\n"))
		if body == "" {
			return head
		}
		return head + "\n" + quoteLines(body, "    ")
	case "collapsed":
		content := strings.TrimSpace(joinNodeMarkdown(n.Children, "\n\n"))
		if content == "" {
			return "<details>\n<summary>" + n.Title + "</summary>\n</details>"
		}
		return "<details>\n<summary>" + n.Title + "</summary>\n" + content + "\n</details>"
	case "thematic_break":
		return "---"
	case "table":
		return tableMarkdown(n)
	case "raw-html", "html":
		if n.Source != "" {
			return strings.TrimSpace(n.Source)
		}
		return strings.TrimSpace(n.RawHTML)
	case "footnote_ref":
		return "[^" + n.ID + "]"
	case "footnote":
		return "[^" + n.ID + "]: " + indentContinuation(strings.TrimSpace(joinNodeMarkdown(n.Children, "\n")), len("[^"+n.ID+"]: "))
	case "footnotes":
		return joinNodeMarkdown(n.Items, "\n")
	default:
		return joinNodeMarkdown(n.Children, "")
	}
}

func joinNodeStrings(nodes []Node, sep string) string {
	parts := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if s := node.String(); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, sep)
}

func joinNodeHTML(nodes []Node, sep string) string {
	parts := make([]string, 0, len(nodes))
	for _, node := range nodes {
		parts = append(parts, node.HTML())
	}
	return strings.Join(parts, sep)
}

func joinNodeMarkdown(nodes []Node, sep string) string {
	parts := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if s := node.Markdown(); strings.TrimSpace(s) != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, sep)
}

func clampHeading(level int) int {
	if level < 1 {
		return 1
	}
	if level > 6 {
		return 6
	}
	return level
}

func quoteLines(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		if lines[i] != "" {
			lines[i] = prefix + lines[i]
		} else {
			lines[i] = strings.TrimRight(prefix, " ")
		}
	}
	return strings.Join(lines, "\n")
}

func indentContinuation(s string, indent int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= 1 {
		return strings.TrimSpace(s)
	}
	prefix := strings.Repeat(" ", indent)
	for i := 1; i < len(lines); i++ {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func cloneMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func documentFrontmatter(metadata map[string]any, filename string) string {
	filtered := frontmatterMetadata(metadata, filename)
	if len(filtered) == 0 {
		return ""
	}

	keys := make([]string, 0, len(filtered))
	for key := range filtered {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := []string{"---"}
	for _, key := range keys {
		lines = append(lines, frontmatterEntry(key, filtered[key])...)
	}
	lines = append(lines, "---")
	return strings.Join(lines, "\n")
}

func frontmatterMetadata(metadata map[string]any, filename string) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if key == "filename" && filename != "" && fmt.Sprint(value) == filename {
			continue
		}
		out[key] = value
	}
	return out
}

func frontmatterEntry(key string, value any) []string {
	raw, err := yaml.Marshal(map[string]any{key: value})
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", key, value)}
	}
	rendered := strings.TrimRight(string(raw), "\n")
	if rendered == "" {
		return []string{key + ":"}
	}
	return strings.Split(rendered, "\n")
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
