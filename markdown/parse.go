package markdown

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// ParseString parses markdown source into a structured Clicky markdown document.
func ParseString(source string, opts ...Option) (*Document, error) {
	return Parse([]byte(source), opts...)
}

// Parse parses markdown source into a structured Clicky markdown document.
func Parse(source []byte, opts ...Option) (*Document, error) {
	options := applyOptions(opts...)
	state, metadata, body, lineOffset, err := newParserState(source, options)
	if err != nil {
		return nil, err
	}
	state.body = body
	state.lineOffset = lineOffset
	state.lineStarts = buildLineMap(body)

	extensions := []goldmark.Extender{}
	if options.GFM {
		extensions = append(extensions, extension.GFM)
	}
	if options.Footnotes {
		extensions = append(extensions, extension.Footnote)
	}

	md := goldmark.New(goldmark.WithExtensions(extensions...))
	root := md.Parser().Parse(text.NewReader(body))
	state.footnoteRefs = collectFootnoteRefs(root)
	converted := state.convert(root)
	if converted.Kind == "" {
		converted.Kind = "document"
	}
	if options.Admonitions {
		converted.Children = foldAdmonitions(converted.Children)
	}

	if metadata == nil {
		metadata = map[string]any{}
	}
	if options.Filename != "" {
		metadata["filename"] = options.Filename
	}
	if len(metadata) == 0 {
		metadata = nil
	}

	return &Document{
		Version:  DocumentVersion,
		Filename: options.Filename,
		Metadata: metadata,
		Root:     converted,
	}, nil
}

// ParseFile reads and parses a markdown file.
func ParseFile(filename string, opts ...Option) (*Document, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	opts = append([]Option{WithFilename(filename)}, opts...)
	return Parse(source, opts...)
}

type parserState struct {
	options      Options
	body         []byte
	lineStarts   []int
	lineOffset   int
	footnoteRefs map[int]string
}

func newParserState(source []byte, options Options) (*parserState, map[string]any, []byte, int, error) {
	metadata, body, lineOffset, err := extractFrontmatter(source, options.Frontmatter)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	return &parserState{options: options}, metadata, body, lineOffset, nil
}

func extractFrontmatter(source []byte, enabled bool) (map[string]any, []byte, int, error) {
	if !enabled || !bytes.HasPrefix(source, []byte("---")) {
		return nil, source, 0, nil
	}
	lines := strings.SplitAfter(string(source), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return nil, source, 0, nil
	}

	offset := len(lines[0])
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" || trimmed == "..." {
			frontmatter := strings.Join(lines[1:i], "")
			var metadata map[string]any
			if strings.TrimSpace(frontmatter) != "" {
				if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
					return nil, nil, 0, fmt.Errorf("parse markdown frontmatter: %w", err)
				}
			}
			offset += len(lines[i])
			return metadata, source[offset:], i + 1, nil
		}
		offset += len(lines[i])
	}
	return nil, source, 0, nil
}

func buildLineMap(source []byte) []int {
	starts := []int{0}
	for i, b := range source {
		if b == '\n' && i+1 < len(source) {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func (p *parserState) lineForOffset(offset int) int {
	if len(p.lineStarts) == 0 {
		return p.lineOffset + 1
	}
	if offset < 0 {
		offset = 0
	}
	idx := sort.Search(len(p.lineStarts), func(i int) bool {
		return p.lineStarts[i] > offset
	}) - 1
	if idx < 0 {
		idx = 0
	}
	return p.lineOffset + idx + 1
}

func (p *parserState) convert(node gast.Node) Node {
	if node == nil {
		return Node{}
	}

	switch n := node.(type) {
	case *gast.Document:
		out := p.containerNode("document", n)
		out.Children = p.convertChildren(n)
		return out
	case *gast.Heading:
		out := p.containerNode("heading", n)
		out.Level = n.Level
		out.Children = p.convertChildren(n)
		return out
	case *gast.Paragraph:
		out := p.containerNode("paragraph", n)
		out.Children = p.convertChildren(n)
		return out
	case *gast.TextBlock:
		out := p.containerNode("paragraph", n)
		out.Children = p.convertChildren(n)
		return out
	case *gast.Text:
		return p.convertText(n)
	case *gast.String:
		return Node{Kind: "text", Text: string(n.Value)}
	case *gast.Emphasis:
		kind := "emphasis"
		if n.Level >= 2 {
			kind = "strong"
		}
		return Node{Kind: kind, Children: p.convertChildren(n)}
	case *east.Strikethrough:
		return Node{Kind: "strike", Children: p.convertChildren(n)}
	case *gast.CodeSpan:
		return Node{Kind: "code", Text: p.inlineText(n)}
	case *gast.Link:
		return Node{
			Kind:     "link",
			Href:     string(n.Destination),
			Title:    string(n.Title),
			Children: p.convertChildren(n),
		}
	case *gast.AutoLink:
		label := string(n.Label(p.body))
		return Node{
			Kind: "link",
			Href: string(n.URL(p.body)),
			Children: []Node{{
				Kind: "text",
				Text: label,
			}},
		}
	case *gast.Image:
		return Node{
			Kind:     "image",
			Href:     string(n.Destination),
			Title:    string(n.Title),
			Children: p.convertChildren(n),
		}
	case *gast.FencedCodeBlock:
		out := p.containerNode("code_block", n)
		out.Language = strings.TrimSpace(string(n.Language(p.body)))
		out.Source = string(n.Lines().Value(p.body))
		return out
	case *gast.CodeBlock:
		out := p.containerNode("code_block", n)
		out.Source = string(n.Lines().Value(p.body))
		return out
	case *gast.List:
		out := p.containerNode("list", n)
		out.Ordered = n.IsOrdered()
		out.Items = p.convertChildren(n)
		return out
	case *gast.ListItem:
		return p.convertListItem(n)
	case *gast.Blockquote:
		out := p.containerNode("blockquote", n)
		out.Children = p.convertChildren(n)
		return out
	case *gast.ThematicBreak:
		return p.containerNode("thematic_break", n)
	case *gast.HTMLBlock:
		return p.convertHTMLBlock(n)
	case *gast.RawHTML:
		return p.convertRawHTML(n)
	case *east.Table:
		return p.convertTable(n)
	case *east.TableHeader:
		return p.convertTableRow("table_header", n)
	case *east.TableRow:
		return p.convertTableRow("table_row", n)
	case *east.TableCell:
		return p.convertTableCell(n)
	case *east.TaskCheckBox:
		checked := n.IsChecked
		return Node{Kind: "task_checkbox", Checked: &checked}
	case *east.FootnoteLink:
		id := p.footnoteRefs[n.Index]
		if id == "" {
			id = fmt.Sprintf("%d", n.Index)
		}
		return Node{Kind: "footnote_ref", ID: id}
	case *east.Footnote:
		out := p.containerNode("footnote", n)
		out.ID = string(n.Ref)
		out.Children = p.convertChildren(n)
		return out
	case *east.FootnoteList:
		out := p.containerNode("footnotes", n)
		out.Items = p.convertChildren(n)
		return out
	default:
		kind := strings.ToLower(node.Kind().String())
		out := p.containerNode(kind, node)
		out.Children = p.convertChildren(node)
		if len(out.Children) == 0 {
			out.Text = p.inlineText(node)
			if out.Text == "" {
				// inlineText only walks child Text nodes; a leaf block keeps its
				// content on its own line segments, so fall back to those.
				out.Text = p.nodeLineText(node)
			}
		}
		return out
	}
}

func collectFootnoteRefs(root gast.Node) map[int]string {
	refs := map[int]string{}
	var walk func(gast.Node)
	walk = func(node gast.Node) {
		if node == nil {
			return
		}
		if footnote, ok := node.(*east.Footnote); ok && len(footnote.Ref) > 0 {
			refs[footnote.Index] = string(footnote.Ref)
		}
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			walk(child)
		}
	}
	walk(root)
	if len(refs) == 0 {
		return nil
	}
	return refs
}

func (p *parserState) convertChildren(node gast.Node) []Node {
	children := []Node{}
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		converted := p.convert(child)
		if converted.Kind == "" {
			continue
		}
		children = append(children, converted)
	}
	return children
}

func (p *parserState) convertText(n *gast.Text) Node {
	text := string(n.Segment.Value(p.body))
	if n.HardLineBreak() || n.SoftLineBreak() {
		text += "\n"
	}
	return Node{Kind: "text", Text: text}
}

// nodeLineText returns text held directly on a leaf block node's line segments
// (goldmark stores block content there, not as child Text nodes). Inline nodes
// have no lines and return "".
func (p *parserState) nodeLineText(node gast.Node) string {
	// Lines() panics on inline nodes; only block leaves carry line segments.
	if node.Type() != gast.TypeBlock {
		return ""
	}
	lines := node.Lines()
	if lines == nil || lines.Len() == 0 {
		return ""
	}
	return string(lines.Value(p.body))
}

func (p *parserState) inlineText(node gast.Node) string {
	var b strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch c := child.(type) {
		case *gast.Text:
			b.Write(c.Segment.Value(p.body))
		case *gast.String:
			b.Write(c.Value)
		default:
			b.WriteString(p.inlineText(child))
		}
	}
	return b.String()
}

func (p *parserState) convertListItem(n *gast.ListItem) Node {
	out := p.containerNode("list_item", n)
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch block := child.(type) {
		case *gast.Paragraph:
			paragraphNode, checked := p.convertTaskBlock(block)
			if checked != nil {
				out.Checked = checked
				out.Children = append(out.Children, paragraphNode)
				continue
			}
		case *gast.TextBlock:
			paragraphNode, checked := p.convertTaskBlock(block)
			if checked != nil {
				out.Checked = checked
				out.Children = append(out.Children, paragraphNode)
				continue
			}
		}
		converted := p.convert(child)
		if converted.Kind != "" {
			out.Children = append(out.Children, converted)
		}
	}
	return out
}

func (p *parserState) convertTaskBlock(block gast.Node) (Node, *bool) {
	out := p.containerNode("paragraph", block)
	var checked *bool
	for child := block.FirstChild(); child != nil; child = child.NextSibling() {
		if checkbox, ok := child.(*east.TaskCheckBox); ok && checked == nil {
			value := checkbox.IsChecked
			checked = &value
			continue
		}
		converted := p.convert(child)
		if converted.Kind != "" {
			out.Children = append(out.Children, converted)
		}
	}
	return out, checked
}

func (p *parserState) convertHTMLBlock(n *gast.HTMLBlock) Node {
	// gast.Node.Text is deprecated; reproduce HTMLBlock.Text exactly: the block's
	// raw lines plus its closing line (e.g. the "</div>" terminating an HTML block).
	lines := n.Lines().Value(p.body)
	if n.HasClosure() {
		lines = append(lines, n.ClosureLine.Value(p.body)...)
	}
	raw := string(lines)
	out := p.containerNode("raw-html", n)
	if !p.options.PreserveHTML {
		out.Kind = "text"
		out.Text = strings.TrimSpace(stripTags(raw))
		return out
	}
	if collapsed, ok := parseDetailsHTML(raw); ok {
		p.applyBlockSource(&collapsed, n)
		return collapsed
	}
	out.Source = raw
	out.RawHTML = raw
	return out
}

func (p *parserState) convertRawHTML(n *gast.RawHTML) Node {
	var b strings.Builder
	for i := 0; i < n.Segments.Len(); i++ {
		segment := n.Segments.At(i)
		b.Write(segment.Value(p.body))
	}
	raw := b.String()
	if !p.options.PreserveHTML {
		return Node{Kind: "text", Text: strings.TrimSpace(stripTags(raw))}
	}
	if collapsed, ok := parseDetailsHTML(raw); ok {
		return collapsed
	}
	return Node{Kind: "raw-html", Source: raw, RawHTML: raw}
}

func (p *parserState) convertTable(n *east.Table) Node {
	out := p.containerNode("table", n)
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		converted := p.convert(child)
		if converted.Kind != "" {
			out.Children = append(out.Children, converted)
		}
	}
	return out
}

func (p *parserState) convertTableRow(kind string, n gast.Node) Node {
	out := p.containerNode(kind, n)
	out.Kind = "table_row"
	out.Children = p.convertChildren(n)
	return out
}

func (p *parserState) convertTableCell(n *east.TableCell) Node {
	out := p.containerNode("table_cell", n)
	out.Align = n.Alignment.String()
	if out.Align == "none" {
		out.Align = ""
	}
	out.Children = p.convertChildren(n)
	return out
}

func (p *parserState) containerNode(kind string, node gast.Node) Node {
	out := Node{
		Kind:       kind,
		Attributes: nodeAttributes(node),
	}
	p.applyBlockSource(&out, node)
	return out
}

func (p *parserState) applyBlockSource(out *Node, node gast.Node) {
	if node == nil || node.Type() != gast.TypeBlock {
		return
	}
	lines := node.Lines()
	if lines == nil || lines.Len() == 0 {
		return
	}
	first := lines.At(0)
	last := lines.At(lines.Len() - 1)
	if p.options.SourceSpans {
		out.LineStart = p.lineForOffset(first.Start)
		out.LineEnd = p.lineForOffset(max(0, last.Stop-1))
	}
}

func nodeAttributes(node gast.Node) map[string]string {
	attrs := node.Attributes()
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		out[string(attr.Name)] = fmt.Sprint(attr.Value)
	}
	return out
}

func foldAdmonitions(nodes []Node) []Node {
	out := make([]Node, 0, len(nodes))
	for i := 0; i < len(nodes); i++ {
		node := nodes[i]
		if node.Kind != "paragraph" {
			out = append(out, node)
			continue
		}
		header := strings.TrimSpace(node.String())
		if !strings.HasPrefix(header, "!!!") {
			out = append(out, node)
			continue
		}
		admonition, inlineBody, ok := parseAdmonitionHeader(header)
		if !ok {
			out = append(out, node)
			continue
		}
		admonition.LineStart = node.LineStart
		admonition.LineEnd = node.LineEnd
		if inlineBody != "" {
			admonition.Children = append(admonition.Children, Node{
				Kind: "paragraph",
				Children: []Node{{
					Kind: "text",
					Text: inlineBody,
				}},
				LineStart: node.LineStart,
				LineEnd:   node.LineEnd,
			})
		}
		if i+1 < len(nodes) && nodes[i+1].Kind == "code_block" && nodes[i+1].Language == "" {
			body := nodes[i+1]
			admonition.Children = append(admonition.Children, Node{
				Kind: "paragraph",
				Children: []Node{{
					Kind: "text",
					Text: strings.TrimSpace(body.Source),
				}},
				LineStart: body.LineStart,
				LineEnd:   body.LineEnd,
			})
			admonition.LineEnd = body.LineEnd
			i++
		}
		out = append(out, admonition)
	}
	return out
}

func parseAdmonitionHeader(header string) (Node, string, bool) {
	lines := strings.SplitN(strings.TrimSpace(header), "\n", 2)
	headLine := lines[0]
	inlineBody := ""
	if len(lines) == 2 {
		inlineBody = strings.TrimSpace(lines[1])
	}
	fields := strings.Fields(strings.TrimSpace(strings.TrimPrefix(headLine, "!!!")))
	if len(fields) == 0 {
		return Node{}, "", false
	}
	severity := strings.ToLower(fields[0])
	title := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(headLine, "!!!")), fields[0]))
	return Node{
		Kind:     "admonition",
		Severity: severity,
		Title:    title,
	}, inlineBody, true
}

func parseDetailsHTML(raw string) (Node, bool) {
	lower := strings.ToLower(raw)
	if !strings.Contains(lower, "<details") || !strings.Contains(lower, "<summary") {
		return Node{}, false
	}
	summaryStart := strings.Index(lower, "<summary")
	if summaryStart < 0 {
		return Node{}, false
	}
	summaryOpenEnd := strings.Index(lower[summaryStart:], ">")
	if summaryOpenEnd < 0 {
		return Node{}, false
	}
	titleStart := summaryStart + summaryOpenEnd + 1
	summaryClose := strings.Index(lower[titleStart:], "</summary>")
	if summaryClose < 0 {
		return Node{}, false
	}
	titleEnd := titleStart + summaryClose
	contentStart := titleEnd + len("</summary>")
	contentEnd := strings.LastIndex(lower, "</details>")
	if contentEnd < contentStart {
		contentEnd = len(raw)
	}
	title := strings.TrimSpace(stripTags(raw[titleStart:titleEnd]))
	content := strings.TrimSpace(raw[contentStart:contentEnd])
	child := Node{Kind: "raw-html", Source: content, RawHTML: content}
	return Node{
		Kind:     "collapsed",
		Title:    title,
		Children: []Node{child},
		Source:   raw,
		RawHTML:  raw,
	}, true
}

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
