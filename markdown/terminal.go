package markdown

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api"
)

type terminalMode bool

const (
	terminalPlain terminalMode = false
	terminalANSI  terminalMode = true
)

func (d *Document) String() string {
	if d == nil {
		return ""
	}
	return renderTerminal(d.Root, terminalPlain)
}

func (d *Document) ANSI() string {
	if d == nil {
		return ""
	}
	return renderTerminal(d.Root, terminalANSI)
}

func (n Node) ANSI() string {
	return renderTerminal(n, terminalANSI)
}

func renderTerminal(n Node, mode terminalMode) string {
	if isTerminalInline(n.Kind) {
		return renderTerminalInline(n, mode)
	}
	return strings.TrimSpace(renderTerminalBlock(n, mode))
}

func renderTerminalBlock(n Node, mode terminalMode) string {
	switch n.Kind {
	case "document":
		return renderTerminalBlocks(n.Children, mode)
	case "paragraph", "table_cell", "table_row", "table_header":
		return renderTerminalInlines(n.Children, mode)
	case "heading":
		return renderTerminalHeading(n, mode)
	case "code_block":
		return mode.render(api.NewCode(n.Source, n.Language))
	case "list":
		return renderTerminalList(n, mode)
	case "list_item":
		return renderTerminalBlocks(n.Children, mode)
	case "blockquote":
		return mode.render(api.Blockquote{Content: api.Text{}.Append(renderTerminalBlocks(n.Children, mode))})
	case "admonition":
		return renderTerminalAdmonition(n, mode)
	case "collapsed":
		return renderTerminalCollapsed(n, mode)
	case "thematic_break":
		return mode.render(api.Text{}.Append("────────", "text-muted"))
	case "table":
		return mode.render(terminalTable(n))
	case "footnote":
		return renderTerminalFootnote(n, mode)
	case "footnotes":
		return renderTerminalFootnotes(n, mode)
	default:
		return renderTerminalInlines(n.Children, mode)
	}
}

func renderTerminalBlocks(nodes []Node, mode terminalMode) string {
	parts := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if rendered := strings.TrimSpace(renderTerminal(node, mode)); rendered != "" {
			parts = append(parts, rendered)
		}
	}
	return strings.Join(parts, "\n\n")
}

func renderTerminalHeading(n Node, mode terminalMode) string {
	content := api.Text{}.Append(renderTerminalInlines(n.Children, terminalPlain))
	return mode.render(api.Heading{Level: n.Level, Content: content})
}

func renderTerminalAdmonition(n Node, mode terminalMode) string {
	var title api.Textable
	if n.Title != "" {
		title = api.Text{}.Append(n.Title)
	}
	var body api.Textable
	if rendered := renderTerminalBlocks(n.Children, mode); rendered != "" {
		body = api.Text{}.Append(rendered)
	}
	return mode.render(api.Admonition{
		Severity: api.ParseSeverity(n.Severity),
		Title:    title,
		Body:     body,
	})
}

func renderTerminalCollapsed(n Node, mode terminalMode) string {
	title := mode.render(api.Text{}.Append(n.Title, "font-bold"))
	body := renderTerminalBlocks(n.Children, mode)
	if body == "" {
		return title
	}
	return title + "\n" + body
}

func renderTerminalList(n Node, mode terminalMode) string {
	items := make([]string, 0, len(n.Items))
	for i, item := range n.Items {
		prefix := terminalListPrefix(n, item, i)
		content := renderTerminalBlocks(item.Children, mode)
		items = append(items, mode.render(api.Text{}.Append(prefix, "text-muted"))+indentContinuation(content, len(prefix)))
	}
	return strings.Join(items, "\n")
}

func terminalListPrefix(list, item Node, index int) string {
	prefix := "- "
	if list.Ordered {
		prefix = fmt.Sprintf("%d. ", index+1)
	}
	if item.Checked == nil {
		return prefix
	}
	if *item.Checked {
		return prefix + "[x] "
	}
	return prefix + "[ ] "
}

func terminalTable(n Node) api.TextTable {
	rows := tableRows(n)
	if len(rows) == 0 {
		return api.TextTable{}
	}
	table := api.TextTable{}
	for i, cell := range rows[0].Children {
		field := fmt.Sprintf("column_%d", i+1)
		table.Headers = append(table.Headers, api.Text{}.Append(renderTerminalInlines(cell.Children, terminalPlain)))
		table.FieldNames = append(table.FieldNames, field)
	}
	for _, row := range rows[1:] {
		table.Rows = append(table.Rows, terminalTableRow(row, table.FieldNames))
	}
	return table
}

func terminalTableRow(row Node, fields []string) api.TableRow {
	values := api.TableRow{}
	for i, field := range fields {
		cell := api.Text{}
		if i < len(row.Children) {
			cell = cell.Append(renderTerminalInlines(row.Children[i].Children, terminalPlain))
		}
		values[field] = api.TypedValue{Textable: cell}
	}
	return values
}

func renderTerminalFootnote(n Node, mode terminalMode) string {
	prefix := fmt.Sprintf("[^%s]: ", n.ID)
	return prefix + indentContinuation(renderTerminalBlocks(n.Children, mode), len(prefix))
}

func renderTerminalFootnotes(n Node, mode terminalMode) string {
	parts := make([]string, 0, len(n.Items))
	for _, item := range n.Items {
		parts = append(parts, renderTerminalFootnote(item, mode))
	}
	return strings.Join(parts, "\n")
}

func renderTerminalInlines(nodes []Node, mode terminalMode) string {
	var rendered strings.Builder
	for _, node := range nodes {
		rendered.WriteString(renderTerminalInline(node, mode))
	}
	return rendered.String()
}

func renderTerminalInline(n Node, mode terminalMode) string {
	switch n.Kind {
	case "text":
		return n.Text
	case "linebreak":
		return "\n"
	case "emphasis":
		return renderTerminalStyled(n.Children, mode, "italic")
	case "strong":
		return renderTerminalStyled(n.Children, mode, "font-bold")
	case "strike":
		return renderTerminalStyled(n.Children, mode, "line-through")
	case "code":
		return mode.render(api.Text{}.Append(n.Text, "text-cyan-500"))
	case "link":
		return mode.render(api.NewLink(n.Href).Append(renderTerminalInlines(n.Children, terminalPlain)))
	case "image":
		return renderTerminalInlines(n.Children, mode)
	case "footnote_ref":
		return api.FootnoteRef{ID: n.ID}.String()
	case "raw-html", "html":
		return terminalRawHTML(n)
	default:
		return renderTerminalBlock(n, mode)
	}
}

func renderTerminalStyled(nodes []Node, mode terminalMode, style string) string {
	return mode.render(api.Text{}.Append(renderTerminalInlines(nodes, terminalPlain), style))
}

func terminalRawHTML(n Node) string {
	if n.Source != "" {
		return n.Source
	}
	if n.RawHTML != "" {
		return n.RawHTML
	}
	return n.Text
}

func isTerminalInline(kind string) bool {
	switch kind {
	case "text", "linebreak", "emphasis", "strong", "strike", "code", "link", "image", "footnote_ref", "raw-html", "html":
		return true
	default:
		return false
	}
}

func (mode terminalMode) render(textable api.Textable) string {
	if mode == terminalANSI {
		return textable.ANSI()
	}
	return textable.String()
}
