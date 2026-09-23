//go:build !wasm

package api

import (
	lipglosstree "github.com/charmbracelet/lipgloss/tree"
)

// buildLipglossTree converts a TextTree to a lipgloss tree
func (tt TextTree) buildLipglossTree(withColors bool, depth int) *lipglosstree.Tree {
	nodeLabel := tt.treeLabel(withColors, depth)

	// If we have no node and only one child, return the child tree directly
	if nodeLabel == "" && len(tt.Children) == 1 {
		return tt.Children[0].buildLipglossTree(withColors, depth)
	}

	// If we have no node and multiple children, we need to create a wrapper
	// Create the tree with root (use empty string if no node)
	t := lipglosstree.New().Root(nodeLabel)

	// Add children
	for _, child := range tt.Children {
		childTree := child.buildLipglossTree(withColors, depth+1)
		if childTree != nil {
			t = t.Child(childTree)
		}
	}

	return t
}

func (tt TextTree) String() string {
	return tt.renderLipgloss(false)
}

func (tt TextTree) ANSI() string {
	return tt.renderLipgloss(true)
}

func (tt TextTree) renderLipgloss(withColors bool) string {
	if tt.Node == nil && len(tt.Children) == 0 {
		return ""
	}

	t := tt.buildLipglossTree(withColors, 0)
	if t == nil {
		return ""
	}

	// Use rounded enumerator
	t = t.Enumerator(lipglosstree.RoundedEnumerator)

	return trimTreePadding(t.String())
}
