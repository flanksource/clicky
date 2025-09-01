package formatters

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/flanksource/clicky/api"
)

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
func (f *TreeFormatter) Format(data interface{}) (string, error) {
	// Check if data implements Pretty interface first
	if pretty, ok := data.(api.Pretty); ok {
		text := pretty.Pretty()
		if f.NoColor {
			return text.String(), nil
		} else {
			return text.ANSI(), nil
		}
	}

	// Check if data is directly a TreeNode
	if treeNode, ok := data.(api.TreeNode); ok {
		return f.FormatTreeFromRoot(treeNode), nil
	}

	// Convert to PrettyData
	prettyData, err := ToPrettyData(data)
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

	// Look for tree fields
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatTree {
			if fieldValue, exists := data.Values[field.Name]; exists {
				if treeNode, ok := fieldValue.Value.(api.TreeNode); ok {
					return f.FormatTreeFromRoot(treeNode), nil
				}
			}
		}
	}

	// No tree fields found - fall back to a simple representation
	return fmt.Sprintf("No tree data found in: %v", data), nil
}

// FormatTree formats a tree node and its children recursively
func (f *TreeFormatter) FormatTree(node api.TreeNode, depth int, prefix string, isLast bool) string {
	if node == nil {
		return ""
	}

	// Check max depth
	if f.Options.MaxDepth >= 0 && depth > f.Options.MaxDepth {
		return ""
	}

	var result strings.Builder

	// Build the current line prefix
	if depth > 0 {
		result.WriteString(prefix)
		if isLast {
			result.WriteString(f.Options.LastPrefix)
		} else {
			result.WriteString(f.Options.BranchPrefix)
		}
	}

	// All TreeNodes now implement Pretty(), so use it for formatting
	prettyText := node.Pretty()
	// Convert Text to string with appropriate formatting
	if f.NoColor {
		result.WriteString(prettyText.String())
	} else {
		result.WriteString(prettyText.ANSI())
	}

	// Handle compact list node specially
	if compactNode, ok := node.(*api.CompactListNode); ok && f.Options.Compact {
		items := f.FormatCompactList(compactNode.GetItems(), "")
		if items != "" {
			result.WriteString(": ")
			result.WriteString(items)
		}
	}

	result.WriteString("\n")

	// Check if node is collapsed (using pretty text as key)
	if f.Options.CollapsedNodes != nil && f.Options.CollapsedNodes[prettyText.String()] {
		return result.String()
	}

	// Process children
	children := node.GetChildren()
	for i, child := range children {
		isLastChild := i == len(children)-1

		// Build the prefix for child nodes
		var childPrefix string
		if depth > 0 {
			childPrefix = prefix
			if isLast {
				childPrefix += f.Options.IndentPrefix
			} else {
				childPrefix += f.Options.ContinuePrefix
			}
		}

		childOutput := f.FormatTree(child, depth+1, childPrefix, isLastChild)
		result.WriteString(childOutput)
	}

	return result.String()
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

// FormatTreeFromRoot formats a tree starting from the root node
func (f *TreeFormatter) FormatTreeFromRoot(root api.TreeNode) string {
	if root == nil {
		return ""
	}
	return f.FormatTree(root, 0, "", true)
}

// applyTailwindStyle applies Tailwind-style classes to text
func (f *TreeFormatter) applyTailwindStyle(text, styleStr string) string {
	style := lipgloss.NewStyle()

	// Parse style string (simplified version - would need full implementation)
	styles := strings.Fields(styleStr)
	for _, s := range styles {
		switch {
		case strings.HasPrefix(s, "text-blue"):
			style = style.Foreground(f.Theme.Info)
		case strings.HasPrefix(s, "text-green"):
			style = style.Foreground(f.Theme.Success)
		case strings.HasPrefix(s, "text-red"):
			style = style.Foreground(f.Theme.Error)
		case strings.HasPrefix(s, "text-yellow"):
			style = style.Foreground(f.Theme.Warning)
		case s == "font-bold":
			style = style.Bold(true)
		case s == "italic":
			style = style.Italic(true)
		case s == "underline":
			style = style.Underline(true)
		}
	}

	return style.Render(text)
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
	case api.TreeNode:
		return node
	case *api.SimpleTreeNode:
		return node
	case *api.CompactListNode:
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
