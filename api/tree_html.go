package api

import (
	"fmt"
	"strings"
)

// treeHTMLRenderer handles HTML rendering for TextTree structures
type treeHTMLRenderer struct {
	nodeCounter int
	interactive bool // false for PDF mode, true for interactive mode with Alpine.js
}

// generateNodeID generates a unique node ID for tree nodes
func (r *treeHTMLRenderer) generateNodeID() string {
	r.nodeCounter++
	return fmt.Sprintf("node-%d", r.nodeCounter)
}

// renderNode recursively renders a TextTree node as HTML
func (r *treeHTMLRenderer) renderNode(tree *TextTree, depth int) string {
	if tree == nil {
		return ""
	}

	var result strings.Builder

	// Get children
	children := tree.Children

	if depth == 0 {
		// Root node - start the tree with Alpine.js data and skip root node label
		if r.interactive {
			// Interactive mode with Alpine.js
			result.WriteString(`<div class="tree-view" x-data="createTreeData()" x-init="expandAll()">`)

			// Add Expand All / Collapse All buttons
			result.WriteString(`<div class="tree-controls mb-3 flex gap-2">`)
			result.WriteString(`<button @click="expandAll()" class="px-3 py-1 text-sm bg-blue-500 hover:bg-blue-600 text-white rounded">`)
			result.WriteString(`Expand All`)
			result.WriteString(`</button>`)
			result.WriteString(`<button @click="collapseAll()" class="px-3 py-1 text-sm bg-gray-500 hover:bg-gray-600 text-white rounded">`)
			result.WriteString(`Collapse All`)
			result.WriteString(`</button>`)
			result.WriteString(`</div>`)
		} else {
			// PDF mode - static tree
			result.WriteString(`<div class="tree-view">`)
		}

		// Skip rendering root node label and render children directly at the top level
		if len(children) > 0 {
			result.WriteString(`<ul class="tree-children space-y-1">`)
			for i := range children {
				childHTML := r.renderNode(&children[i], depth+1)
				result.WriteString(childHTML)
			}
			result.WriteString(`</ul>`)
		}

		result.WriteString(`</div>`)
	} else {
		// Child node
		result.WriteString(`<li class="flex items-start tree-node-wrapper">`)

		if len(children) > 0 && r.interactive {
			// Alpine.js toggle for nodes with children
			nodeID := r.generateNodeID()
			result.WriteString(fmt.Sprintf(`<span class="tree-toggle" @click.stop="toggleNode('%s')" data-node-id="%s">`, nodeID, nodeID))
			result.WriteString(fmt.Sprintf(`<iconify-icon icon="ion:chevron-forward" x-show="!isExpanded('%s')"></iconify-icon>`, nodeID))
			result.WriteString(fmt.Sprintf(`<iconify-icon icon="ion:chevron-down" x-show="isExpanded('%s')"></iconify-icon>`, nodeID))
			result.WriteString(`</span>`)
		} else {
			// Static indicator for leaf nodes
			result.WriteString(`<span class="tree-leaf-indicator">•</span>`)
		}

		result.WriteString(`<div class="flex-1">`)
		if len(children) > 0 && r.interactive {
			nodeID := fmt.Sprintf("node-%d", r.nodeCounter)
			result.WriteString(fmt.Sprintf(`<span class="tree-node cursor-pointer" @click="toggleNode('%s')">`, nodeID))
		} else {
			result.WriteString(`<span class="tree-node">`)
		}
		result.WriteString(tree.Node.HTML())
		result.WriteString(`</span>`)

		if len(children) > 0 {
			if r.interactive {
				nodeID := fmt.Sprintf("node-%d", r.nodeCounter)
				result.WriteString(fmt.Sprintf(`<ul class="tree-children ml-4 mt-1 space-y-1" x-show="isExpanded('%s')">`, nodeID))
			} else {
				result.WriteString(`<ul class="tree-children ml-4 mt-1 space-y-1">`)
			}
			for i := range children {
				childHTML := r.renderNode(&children[i], depth+1)
				result.WriteString(childHTML)
			}
			result.WriteString(`</ul>`)
		}

		result.WriteString(`</div>`)
		result.WriteString(`</li>`)
	}

	return result.String()
}

// RenderTreeHTML renders a TextTree as interactive HTML
// This is the main entry point for HTML tree rendering
func RenderTreeHTML(tree *TextTree, interactive bool) string {
	if tree == nil {
		return `<p class="text-gray-500">No tree data available</p>`
	}

	renderer := &treeHTMLRenderer{
		nodeCounter: 0,
		interactive: interactive,
	}

	return renderer.renderNode(tree, 0)
}
