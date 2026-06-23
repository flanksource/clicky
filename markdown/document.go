package markdown

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
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

func (d *Document) String() string {
	if d == nil {
		return ""
	}
	return d.Root.String()
}

func (d *Document) ANSI() string {
	return d.String()
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

// ClickyDocument exports the parsed tree using the same JSON envelope consumed
// by @flanksource/clicky-ui.
func (d *Document) ClickyDocument() formatters.ClickyDocument {
	if d == nil {
		return formatters.ClickyDocument{Version: DocumentVersion, Node: formatters.ClickyNode{Kind: "document"}}
	}
	return formatters.ClickyDocument{
		Version:  DocumentVersion,
		Metadata: cloneMetadata(d.Metadata),
		Node:     d.Root.ClickyNode(),
	}
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

func (n Node) ANSI() string {
	return n.String()
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

// ClickyNode converts the markdown node to Clicky's concrete document JSON node.
func (n Node) ClickyNode() formatters.ClickyNode {
	node := formatters.ClickyNode{
		Kind:       n.Kind,
		Plain:      n.String(),
		Text:       n.Text,
		HTML:       n.RawHTML,
		Level:      n.Level,
		Language:   n.Language,
		Source:     n.Source,
		Href:       n.Href,
		ID:         n.ID,
		Severity:   n.Severity,
		Ordered:    n.Ordered,
		Checked:    n.Checked,
		Attributes: cloneStringMap(n.Attributes),
	}

	switch n.Kind {
	case "document":
		node.Kind = "document"
		node.Text = ""
		node.Children = convertNodes(n.Children)
	case "paragraph", "table_cell", "table_row":
		node.Text = ""
		node.Children = convertNodes(n.Children)
	case "heading":
		node.Text = ""
		content := textContainerNode(n.Children)
		node.Content = &content
	case "emphasis":
		node.Kind = "text"
		node.Text = ""
		node.Children = convertNodes(n.Children)
		node.Style = &formatters.ClickyStyle{Italic: true}
	case "strong":
		node.Kind = "text"
		node.Text = ""
		node.Children = convertNodes(n.Children)
		node.Style = &formatters.ClickyStyle{Bold: true}
	case "strike":
		node.Kind = "text"
		node.Text = ""
		node.Children = convertNodes(n.Children)
		node.Style = &formatters.ClickyStyle{Strikethrough: true}
	case "linebreak":
		node.Kind = "text"
		node.Text = "\n"
		node.Plain = "\n"
	case "code":
		node.Kind = "code"
		node.Inline = true
		node.Source = n.Text
		node.Text = n.Text
	case "code_block":
		node.Kind = "code"
		node.Text = ""
		node.HighlightedHTML = api.NewCode(n.Source, n.Language).HTML()
	case "link", "image":
		node.Text = ""
		content := textContainerNode(n.Children)
		node.Content = &content
	case "list":
		node.Items = convertNodes(n.Items)
	case "list_item":
		node.Children = convertNodes(n.Children)
		content := textContainerNode(n.Children)
		node.Content = &content
	case "blockquote", "admonition", "collapsed":
		if n.Title != "" {
			label := formatters.ClickyNode{Kind: "text", Plain: n.Title, Text: n.Title}
			node.Label = &label
		}
		content := textContainerNode(n.Children)
		node.Content = &content
	case "table":
		node.Columns, node.Rows = tableClicky(n)
		node.Children = nil
	case "footnote":
		content := textContainerNode(n.Children)
		node.Content = &content
	case "footnotes":
		node.Items = convertNodes(n.Items)
	}

	return node
}

func convertNodes(nodes []Node) []formatters.ClickyNode {
	out := make([]formatters.ClickyNode, 0, len(nodes))
	for _, child := range nodes {
		out = append(out, child.ClickyNode())
	}
	return out
}

func textContainerNode(nodes []Node) formatters.ClickyNode {
	if len(nodes) == 1 {
		return nodes[0].ClickyNode()
	}
	return formatters.ClickyNode{
		Kind:     "fragment",
		Plain:    strings.TrimSpace(joinNodeStrings(nodes, "")),
		Children: convertNodes(nodes),
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

func tableMarkdown(table Node) string {
	rows := tableRows(table)
	if len(rows) == 0 {
		return ""
	}
	header := rows[0]
	lines := []string{"| " + strings.Join(markdownCells(header), " | ") + " |"}
	aligns := make([]string, len(header.Children))
	for i, cell := range header.Children {
		switch cell.Align {
		case "right":
			aligns[i] = "---:"
		case "center":
			aligns[i] = ":---:"
		default:
			aligns[i] = "---"
		}
	}
	lines = append(lines, "| "+strings.Join(aligns, " | ")+" |")
	for _, row := range rows[1:] {
		lines = append(lines, "| "+strings.Join(markdownCells(row), " | ")+" |")
	}
	return strings.Join(lines, "\n")
}

func markdownCells(row Node) []string {
	cells := make([]string, 0, len(row.Children))
	for _, cell := range row.Children {
		text := strings.ReplaceAll(cell.Markdown(), "|", `\|`)
		cells = append(cells, strings.TrimSpace(text))
	}
	return cells
}

func tableString(table Node, includeHeader bool) string {
	rows := tableRows(table)
	if !includeHeader && len(rows) > 0 {
		rows = rows[1:]
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, strings.Join(markdownCells(row), "\t"))
	}
	return strings.Join(lines, "\n")
}

func tableHTML(table Node) string {
	rows := tableRows(table)
	if len(rows) == 0 {
		return "<table></table>"
	}
	var b strings.Builder
	b.WriteString("<table>")
	b.WriteString("<thead>")
	b.WriteString(rowHTML(rows[0], "th"))
	b.WriteString("</thead>")
	if len(rows) > 1 {
		b.WriteString("<tbody>")
		for _, row := range rows[1:] {
			b.WriteString(rowHTML(row, "td"))
		}
		b.WriteString("</tbody>")
	}
	b.WriteString("</table>")
	return b.String()
}

func rowHTML(row Node, cellTag string) string {
	var b strings.Builder
	b.WriteString("<tr>")
	for _, cell := range row.Children {
		align := ""
		if cell.Align != "" && cell.Align != "none" {
			align = ` style="text-align:` + html.EscapeString(cell.Align) + `"`
		}
		b.WriteString("<" + cellTag + align + ">" + joinNodeHTML(cell.Children, "") + "</" + cellTag + ">")
	}
	b.WriteString("</tr>")
	return b.String()
}

func tableRows(table Node) []Node {
	rows := make([]Node, 0, len(table.Children))
	for _, row := range table.Children {
		if row.Kind == "table_row" || row.Kind == "table_header" {
			rows = append(rows, row)
		}
	}
	return rows
}

func tableClicky(table Node) ([]formatters.ClickyColumn, []formatters.ClickyRow) {
	rows := tableRows(table)
	if len(rows) == 0 {
		return nil, nil
	}
	header := rows[0]
	columns := make([]formatters.ClickyColumn, 0, len(header.Children))
	seen := map[string]int{}
	for i, cell := range header.Children {
		name := uniqueColumnName(slugColumnName(cell.String(), i), seen)
		headerNode := cell.ClickyNode()
		columns = append(columns, formatters.ClickyColumn{
			Name:   name,
			Label:  cell.String(),
			Header: &headerNode,
			Align:  cell.Align,
		})
	}
	clickyRows := make([]formatters.ClickyRow, 0, len(rows)-1)
	for _, row := range rows[1:] {
		clickyRow := formatters.ClickyRow{Cells: map[string]formatters.ClickyNode{}}
		for i, column := range columns {
			if i >= len(row.Children) {
				clickyRow.Cells[column.Name] = formatters.ClickyNode{Kind: "text"}
				continue
			}
			clickyRow.Cells[column.Name] = row.Children[i].ClickyNode()
		}
		clickyRows = append(clickyRows, clickyRow)
	}
	return columns, clickyRows
}

var nonColumnName = regexp.MustCompile(`[^a-z0-9_]+`)

func slugColumnName(label string, index int) string {
	name := strings.ToLower(strings.TrimSpace(label))
	name = strings.ReplaceAll(name, " ", "_")
	name = nonColumnName.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		return fmt.Sprintf("column_%d", index+1)
	}
	return name
}

// uniqueColumnName disambiguates duplicate column slugs by suffixing repeats
// (_2, _3, …). ClickyRow.Cells is keyed by column name, so two headers slugging
// to the same name would otherwise collide and silently drop a column's cells.
func uniqueColumnName(name string, seen map[string]int) string {
	seen[name]++
	if n := seen[name]; n > 1 {
		return fmt.Sprintf("%s_%d", name, n)
	}
	return name
}
