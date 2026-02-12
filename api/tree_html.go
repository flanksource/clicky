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
	children := tree.Children

	if depth == 0 {
		if r.interactive {
			result.WriteString(`<div class="tree-view" x-data="createTreeData()" x-init="expandAll()">`)
			result.WriteString(`<div class="tree-controls mb-3 flex gap-2">`)
			result.WriteString(`<button @click="expandAll()" class="px-3 py-1 text-sm bg-blue-500 hover:bg-blue-600 text-white rounded">Expand All</button>`)
			result.WriteString(`<button @click="collapseAll()" class="px-3 py-1 text-sm bg-gray-500 hover:bg-gray-600 text-white rounded">Collapse All</button>`)
			result.WriteString(`</div>`)
			if len(children) > 0 {
				result.WriteString(`<ul class="tree-children space-y-1">`)
				for i := range children {
					result.WriteString(r.renderInteractiveNode(&children[i], depth+1))
				}
				result.WriteString(`</ul>`)
			}
		} else {
			result.WriteString(`<div class="tree-view tree-static" style="font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace; font-size: 0.875rem; line-height: 1.5;">`)
			if len(children) > 0 {
				for i := range children {
					r.renderStaticNode(&result, &children[i], "", i == len(children)-1)
				}
			}
		}
		result.WriteString(`</div>`)
		return result.String()
	}

	return result.String()
}

// renderInteractiveNode renders a single node in interactive mode
func (r *treeHTMLRenderer) renderInteractiveNode(tree *TextTree, depth int) string {
	var result strings.Builder
	children := tree.Children

	result.WriteString(`<li class="flex items-start tree-node-wrapper">`)

	if len(children) > 0 {
		nodeID := r.generateNodeID()
		result.WriteString(fmt.Sprintf(`<span class="tree-toggle" @click.stop="toggleNode('%s')" data-node-id="%s">`, nodeID, nodeID))
		result.WriteString(fmt.Sprintf(`<iconify-icon icon="ion:chevron-forward" x-show="!isExpanded('%s')"></iconify-icon>`, nodeID))
		result.WriteString(fmt.Sprintf(`<iconify-icon icon="ion:chevron-down" x-show="isExpanded('%s')"></iconify-icon>`, nodeID))
		result.WriteString(`</span>`)
	} else {
		result.WriteString(`<span class="tree-leaf-indicator">•</span>`)
	}

	result.WriteString(`<div class="flex-1">`)
	if len(children) > 0 {
		nodeID := fmt.Sprintf("node-%d", r.nodeCounter)
		result.WriteString(fmt.Sprintf(`<span class="tree-node cursor-pointer" @click="toggleNode('%s')">`, nodeID))
	} else {
		result.WriteString(`<span class="tree-node">`)
	}
	result.WriteString(tree.Node.HTML())
	result.WriteString(`</span>`)

	if len(children) > 0 {
		nodeID := fmt.Sprintf("node-%d", r.nodeCounter)
		result.WriteString(fmt.Sprintf(`<ul class="tree-children ml-4 mt-1 space-y-1" x-show="isExpanded('%s')">`, nodeID))
		for i := range children {
			childHTML := r.renderInteractiveNode(&children[i], depth+1)
			result.WriteString(childHTML)
		}
		result.WriteString(`</ul>`)
	}

	result.WriteString(`</div></li>`)
	return result.String()
}

// renderStaticNode renders a node in static (PDF) mode with text-based tree connectors
// prefix contains the accumulated indentation characters (│ and spaces) for proper alignment
func (r *treeHTMLRenderer) renderStaticNode(result *strings.Builder, tree *TextTree, prefix string, isLast bool) {
	if tree == nil {
		return
	}

	children := tree.Children
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	result.WriteString(`<div style="white-space: pre;">`)
	fmt.Fprintf(result, `<span style="color: #9ca3af;">%s%s</span>`, prefix, connector)
	result.WriteString(tree.Node.HTML())
	result.WriteString(`</div>`)

	if len(children) > 0 {
		childPrefix := prefix
		if isLast {
			childPrefix += "    " // 4 spaces when no continuing line
		} else {
			childPrefix += "│   " // vertical line + 3 spaces
		}
		for i := range children {
			r.renderStaticNode(result, &children[i], childPrefix, i == len(children)-1)
		}
	}
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
