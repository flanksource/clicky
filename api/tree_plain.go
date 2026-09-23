package api

import "strings"

const (
	plainTreeBranch     = "├── "
	plainTreeLastBranch = "╰── "
	plainTreeGutter     = "│   "
	plainTreeLastGutter = "    "
	// plainTreeTab is how the native renderer expands a tab in a node label.
	plainTreeTab = "    "
)

// plainTree renders the tree with the same rounded connectors, continuation
// gutters and label normalization as the native lipgloss renderer, without a
// layout dependency.
func (tt TextTree) plainTree(withColors bool) string {
	if tt.Node == nil && len(tt.Children) == 0 {
		return ""
	}
	root, label := tt.collapsedTreeLevel(withColors, 0)
	var lines []string
	if label != "" {
		lines = strings.Split(label, "\n")
	}
	lines = root.appendPlainTreeChildren(lines, withColors, 1, "")
	return trimTreePadding(strings.Join(lines, "\n"))
}

// collapsedTreeLevel skips nodeless levels holding a single child, as the
// native renderer does, and returns the node to draw with its label.
func (tt TextTree) collapsedTreeLevel(withColors bool, depth int) (TextTree, string) {
	label := tt.treeLabel(withColors, depth)
	if label == "" && len(tt.Children) == 1 {
		return tt.Children[0].collapsedTreeLevel(withColors, depth)
	}
	return tt, strings.ReplaceAll(label, "\t", plainTreeTab)
}

func (tt TextTree) appendPlainTreeChildren(lines []string, withColors bool, depth int, prefix string) []string {
	for i, child := range tt.Children {
		branch, gutter := plainTreeBranch, plainTreeGutter
		if i == len(tt.Children)-1 {
			branch, gutter = plainTreeLastBranch, plainTreeLastGutter
		}
		node, label := child.collapsedTreeLevel(withColors, depth)
		for j, line := range strings.Split(label, "\n") {
			if j == 0 {
				lines = append(lines, prefix+branch+line)
			} else {
				lines = append(lines, prefix+gutter+line)
			}
		}
		lines = node.appendPlainTreeChildren(lines, withColors, depth+1, prefix+gutter)
	}
	return lines
}
