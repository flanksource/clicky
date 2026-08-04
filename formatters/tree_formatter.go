package formatters

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss/tree"
	"github.com/charmbracelet/x/ansi"
	"github.com/flanksource/clicky/api"
)

type TextTree struct {
	Node     api.Textable
	Children []TextTree
}

// TreeFormatter handles tree structure formatting
type TreeFormatter struct {
	Theme   api.Theme
	NoColor bool
	Options *api.TreeOptions
}

// NewTreeFormatter creates a new tree formatter
func NewTreeFormatter(theme api.Theme, noColor bool, options *api.TreeOptions) *TreeFormatter {
	if options == nil {
		options = api.DefaultTreeOptions()
	}
	return &TreeFormatter{
		Theme:   theme,
		NoColor: noColor,
		Options: options,
	}
}

// Format formats data as a tree structure
func (f *TreeFormatter) Format(data ...any) (string, error) {
	if nodes, ok := ToSlice[api.TreeNode](data...); ok {
		return f.FormatTreeFromRoot(api.NewConcreteTree(nodes...)), nil
	}

	if len(data) == 1 {
		if node, ok := data[0].(api.TreeNode); ok {
			return f.FormatTreeFromRoot(node), nil
		} else if pretty, ok := data[0].(api.Pretty); ok {
			text := pretty.Pretty()
			if f.NoColor {
				return text.String(), nil
			} else {
				return text.ANSI(), nil
			}
		}
	}

	// Convert to PrettyData
	// Unwrap varargs if single element to avoid double-wrapping
	var dataToConvert any
	if len(data) == 1 {
		dataToConvert = data[0]
	} else {
		dataToConvert = data
	}
	prettyData, err := ToPrettyData(dataToConvert)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}

	if prettyData == nil || prettyData.Schema == nil {
		return "", nil
	}

	return f.FormatPrettyData(prettyData)
}

// FormatPrettyData formats PrettyData as a tree structure
func (f *TreeFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil || data.Schema == nil {
		return "", nil
	}

	// Check if data itself has a tree
	if data.Tree != nil {
		return data.Tree.String(), nil
	}

	// Look for tree fields in TypedMap
	if data.TypedMap != nil {
		for _, field := range data.Schema.Fields {
			if field.Format == api.FormatTree {
				if fieldValue, exists := (*data.TypedMap)[field.Name]; exists {
					if fieldValue.Tree != nil {
						return fieldValue.Tree.String(), nil
					}
				}
			}
		}
	}

	// No tree fields found - provide detailed diagnostic information
	return buildNoTreeDataMessage(data), nil
}

// buildNoTreeDataMessage creates a helpful error message when no tree data is found
func buildNoTreeDataMessage(data *api.PrettyData) string {
	var msg strings.Builder
	msg.WriteString("No tree data found.\n\n")

	// Show available fields
	if len(data.Schema.Fields) > 0 {
		msg.WriteString("Available fields:\n")
		for _, field := range data.Schema.Fields {
			fmt.Fprintf(&msg, "  - %s (format: %s", field.Name, field.Format)
			if field.Label != "" && field.Label != field.Name {
				fmt.Fprintf(&msg, ", label: %s", field.Label)
			}
			msg.WriteString(")\n")
		}
		msg.WriteString("\n")
	} else {
		msg.WriteString("No fields found in schema.\n\n")
	}

	// Show available table
	if data.Table != nil && len(data.Table.Rows) > 0 {
		fmt.Fprintf(&msg, "Available table with %d rows\n\n", len(data.Table.Rows))
	}

	// Show original data type if available
	if data.Original != nil {
		fmt.Fprintf(&msg, "Original data type: %T\n", data.Original)
	}

	msg.WriteString("\nExpected: At least one field with format='tree'")
	return msg.String()
}

// normalizeTreeLabel removes trailing whitespace per line and drops every blank
// line from a node label so multi-line labels don't render stray "│"-gutter
// blank lines. lipgloss prefixes every physical line of a multi-line label with
// the continuation gutter, so a blank line surfaces as a bare gutter line. A
// node label is a single tree row's content, not a paragraph — blank-line
// spacing that reads well in flat output is noise inside the tree.
func normalizeTreeLabel(s string) string {
	return normalizeTreeLabelWidth(s, api.GetTerminalWidth())
}

func normalizeTreeLabelWidth(s string, maxWidth int) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(ansi.Strip(line)) == "" {
			continue
		}
		out = append(out, ansi.Truncate(line, maxWidth, "…"))
	}
	return strings.Join(out, "\n")
}

// buildLipglossTree builds a lipgloss tree structure from a TreeNode
func (f *TreeFormatter) buildLipglossTree(node api.TreeNode, depth int) *tree.Tree {
	if node == nil {
		return tree.New()
	}

	// All TreeNodes implement Pretty(), so use it for formatting
	prettyText := node.Pretty()

	// Render the complete Text value so Textable children remain visible in
	// tree labels, then bound each physical line to the terminal width left at
	// this depth. ANSI-aware truncation preserves SGR state and prevents one
	// long captured-output line from forcing lipgloss to pad every sibling.
	var nodeLabel string
	if f.NoColor {
		nodeLabel = prettyText.String()
	} else {
		nodeLabel = prettyText.ANSI()
	}
	nodeLabel = normalizeTreeLabelWidth(nodeLabel, max(1, api.GetTerminalWidth()-depth*4-2))

	// Handle compact list node specially
	if compactNode, ok := node.(*api.CompactListNode); ok && f.Options.Compact {
		items := f.FormatCompactList(compactNode.GetItems(), "")
		if items != "" {
			nodeLabel = nodeLabel + ": " + items
		}
	}

	// Create the tree with this node as root
	t := tree.New().Root(nodeLabel)

	// Check if node is collapsed
	if f.Options.CollapsedNodes != nil && f.Options.CollapsedNodes[prettyText.String()] {
		return t
	}

	// Process children if not at max depth
	if f.Options.MaxDepth < 0 || depth < f.Options.MaxDepth {
		children := node.GetChildren()
		for _, child := range children {
			childTree := f.buildLipglossTree(child, depth+1)
			if childTree != nil {
				t = t.Child(childTree)
			}
		}
	}

	return t
}

// FormatCompactList formats a list of items in compact mode
func (f *TreeFormatter) FormatCompactList(items []string, separator string) string {
	if len(items) == 0 {
		return ""
	}

	if separator == "" {
		separator = ", "
	}

	// Join items with separator
	return strings.Join(items, separator)
}

// FormatTreeFromRoot formats a tree starting from the root node using lipgloss
func (f *TreeFormatter) FormatTreeFromRoot(root api.TreeNode) string {
	if root == nil {
		return ""
	}

	t := f.buildLipglossTree(root, 0)
	if t == nil {
		return ""
	}

	// Configure the tree enumerator (rounded style)
	t = t.Enumerator(tree.RoundedEnumerator)

	// Use ASCII characters if UseUnicode is false
	if !f.Options.UseUnicode {
		t = t.Enumerator(func(children tree.Children, i int) string {
			if children.Length()-1 == i {
				return "`-- "
			}
			return "+-- "
		})
		t = t.Indenter(func(children tree.Children, i int) string {
			if children.Length()-1 == i {
				return "    "
			}
			return "|   "
		})
	}

	return trimTreePadding(t.String())
}

func trimTreePadding(rendered string) string {
	lines := strings.Split(rendered, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}

// FormatInlineTree formats a tree structure for inline display
func (f *TreeFormatter) FormatInlineTree(nodes []api.TreeNode, separator string) string {
	if len(nodes) == 0 {
		return ""
	}

	if separator == "" {
		separator = " → "
	}

	var parts []string
	for _, node := range nodes {
		prettyText := node.Pretty()
		if f.NoColor {
			parts = append(parts, prettyText.String())
		} else {
			parts = append(parts, prettyText.ANSI())
		}
	}

	return strings.Join(parts, separator)
}

// WrapCompactList wraps a compact list to fit within a specified width
func (f *TreeFormatter) WrapCompactList(items []string, maxWidth int, indent string) string {
	if len(items) == 0 {
		return ""
	}

	var lines []string
	var currentLine strings.Builder
	currentLine.WriteString(indent)
	currentWidth := len(indent)

	for _, item := range items {
		itemLen := len(item)
		separatorLen := 2 // ", "

		// Check if adding this item would exceed max width
		if currentWidth > len(indent) && currentWidth+separatorLen+itemLen > maxWidth {
			// Start a new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(indent)
			currentWidth = len(indent)
		}

		// Add separator if not the first item on the line
		if currentWidth > len(indent) {
			currentLine.WriteString(", ")
			currentWidth += separatorLen
		}

		currentLine.WriteString(item)
		currentWidth += itemLen
	}

	// Add the last line
	if currentLine.Len() > len(indent) {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}

// ConvertToTreeNode converts various types to TreeNode interface
func ConvertToTreeNode(v interface{}) api.TreeNode {
	switch node := v.(type) {
	case *api.SimpleTreeNode:
		return node
	case *api.CompactListNode:
		return node
	case api.TreeNode:
		return node
	case map[string]interface{}:
		// Convert map to tree node
		return mapToTreeNode(node)
	default:
		// Create a simple node with string representation
		return &api.SimpleTreeNode{
			Label: fmt.Sprintf("%v", v),
		}
	}
}

// mapToTreeNode converts a map to a tree node
func mapToTreeNode(m map[string]interface{}) api.TreeNode {
	node := &api.SimpleTreeNode{
		Metadata: make(map[string]interface{}),
	}

	// Extract known fields
	if label, ok := m["label"].(string); ok {
		node.Label = label
	} else if name, ok := m["name"].(string); ok {
		node.Label = name
	}

	if icon, ok := m["icon"].(string); ok {
		node.Icon = icon
	}

	if style, ok := m["style"].(string); ok {
		node.Style = style
	}

	// Handle children
	if children, ok := m["children"].([]interface{}); ok {
		for _, child := range children {
			if childNode := ConvertToTreeNode(child); childNode != nil {
				node.Children = append(node.Children, childNode)
			}
		}
	}

	// Store other fields as metadata
	for k, v := range m {
		if k != "label" && k != "name" && k != "icon" && k != "style" && k != "children" {
			node.Metadata[k] = v
		}
	}

	return node
}
