package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

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

// ClickyNode converts the markdown node to Clicky's concrete document JSON node.
func (n Node) ClickyNode() formatters.ClickyNode {
	node := formatters.ClickyNode{
		Kind: n.Kind, Plain: n.String(), Text: n.Text, HTML: n.RawHTML,
		Level: n.Level, Language: n.Language, Source: n.Source, Href: n.Href,
		ID: n.ID, Severity: n.Severity, Ordered: n.Ordered, Checked: n.Checked,
		Attributes: cloneStringMap(n.Attributes),
	}
	applyClickyNodeKind(n, &node)
	return node
}

func applyClickyNodeKind(n Node, node *formatters.ClickyNode) {
	switch n.Kind {
	case "document":
		node.Kind, node.Text, node.Children = "document", "", convertNodes(n.Children)
	case "paragraph", "table_cell", "table_row":
		node.Text, node.Children = "", convertNodes(n.Children)
	case "heading":
		node.Text = ""
		node.Content = clickyContent(n.Children)
	case "emphasis", "strong", "strike":
		applyClickyStyle(n, node)
	case "linebreak":
		node.Kind, node.Text, node.Plain = "text", "\n", "\n"
	case "code":
		node.Kind, node.Inline, node.Source, node.Text = "code", true, n.Text, n.Text
	case "code_block":
		node.Kind, node.Text = "code", ""
		node.HighlightedHTML = api.NewCode(n.Source, n.Language).HTML()
	case "link", "image":
		node.Text = ""
		node.Content = clickyContent(n.Children)
	case "list":
		node.Items = convertNodes(n.Items)
	case "list_item":
		node.Children = convertNodes(n.Children)
		node.Content = clickyContent(n.Children)
	case "blockquote", "admonition", "collapsed":
		applyClickyBlock(n, node)
	case "table":
		node.Columns, node.Rows = tableClicky(n)
		node.Children = nil
	case "footnote":
		node.Content = clickyContent(n.Children)
	case "footnotes":
		node.Items = convertNodes(n.Items)
	}
}

func applyClickyStyle(n Node, node *formatters.ClickyNode) {
	node.Kind, node.Text, node.Children = "text", "", convertNodes(n.Children)
	node.Style = &formatters.ClickyStyle{
		Italic:        n.Kind == "emphasis",
		Bold:          n.Kind == "strong",
		Strikethrough: n.Kind == "strike",
	}
}

func applyClickyBlock(n Node, node *formatters.ClickyNode) {
	if n.Title != "" {
		label := formatters.ClickyNode{Kind: "text", Plain: n.Title, Text: n.Title}
		node.Label = &label
	}
	node.Content = clickyContent(n.Children)
}

func clickyContent(nodes []Node) *formatters.ClickyNode {
	content := textContainerNode(nodes)
	return &content
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
		Kind: "fragment", Plain: strings.TrimSpace(joinNodeStrings(nodes, "")), Children: convertNodes(nodes),
	}
}

func tableClicky(table Node) ([]formatters.ClickyColumn, []formatters.ClickyRow) {
	rows := tableRows(table)
	if len(rows) == 0 {
		return nil, nil
	}
	columns := clickyColumns(rows[0])
	clickyRows := make([]formatters.ClickyRow, 0, len(rows)-1)
	for _, row := range rows[1:] {
		clickyRows = append(clickyRows, clickyRow(row, columns))
	}
	return columns, clickyRows
}

func clickyColumns(header Node) []formatters.ClickyColumn {
	columns := make([]formatters.ClickyColumn, 0, len(header.Children))
	seen := map[string]int{}
	for i, cell := range header.Children {
		name := uniqueColumnName(slugColumnName(cell.String(), i), seen)
		headerNode := cell.ClickyNode()
		columns = append(columns, formatters.ClickyColumn{Name: name, Label: cell.String(), Header: &headerNode, Align: cell.Align})
	}
	return columns
}

func clickyRow(row Node, columns []formatters.ClickyColumn) formatters.ClickyRow {
	result := formatters.ClickyRow{Cells: map[string]formatters.ClickyNode{}}
	for i, column := range columns {
		if i >= len(row.Children) {
			result.Cells[column.Name] = formatters.ClickyNode{Kind: "text"}
			continue
		}
		result.Cells[column.Name] = row.Children[i].ClickyNode()
	}
	return result
}

var nonColumnName = regexp.MustCompile(`[^a-z0-9_]+`)

func slugColumnName(label string, index int) string {
	name := strings.ToLower(strings.TrimSpace(label))
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.Trim(nonColumnName.ReplaceAllString(name, "_"), "_")
	if name == "" {
		return fmt.Sprintf("column_%d", index+1)
	}
	return name
}

func uniqueColumnName(name string, seen map[string]int) string {
	seen[name]++
	if seen[name] > 1 {
		return fmt.Sprintf("%s_%d", name, seen[name])
	}
	return name
}
