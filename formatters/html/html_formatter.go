package html

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
	"github.com/flanksource/clicky/formatters"
	. "github.com/flanksource/clicky/formatters"
)

//go:embed tree.css
var treeCSS string

//go:embed tree.js
var treeJS string

//go:embed gridjs-theme.css
var gridjsThemeCSS string

//go:embed pdf.css
var pdfCSS string

//go:embed tooltips.js
var tooltipsJS string

func init() {
	html := NewHTMLFormatter()
	RegisterFormatter("html", html.Format)
}

// HTMLFormatter handles HTML formatting
type HTMLFormatter struct {
	IncludeCSS   bool
	IsPDFMode    bool
	tableCounter int // Counter for generating unique table IDs
	nodeCounter  int // Counter for generating unique tree node IDs
}

// NewHTMLFormatter creates a new HTML formatter
func NewHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{
		IncludeCSS: true,
	}
}

// ToPrettyData converts various input types to PrettyData
func (f *HTMLFormatter) ToPrettyData(data interface{}) (*api.PrettyData, error) {
	return ToPrettyDataWithOptions(data, FormatOptions{Format: "html"})
}

// getCSS returns Tailwind CSS CDN and custom styling
func (f *HTMLFormatter) getCSS() string {
	css := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Clicky Output</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="https://unpkg.com/gridjs/dist/theme/mermaid.min.css" rel="stylesheet" />
    <script src="https://unpkg.com/gridjs/dist/gridjs.umd.js"></script>
    <script src="https://code.iconify.design/iconify-icon/2.0.0/iconify-icon.min.js"></script>
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="https://unpkg.com/tippy.js@6"></script>
    <link rel="stylesheet" href="https://unpkg.com/tippy.js@6/dist/tippy.css" />
    <script defer src="https://cdn.jsdelivr.net/npm/@alpinejs/collapse@3.x.x/dist/cdn.min.js"></script>
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
    <style>
        ` + gridjsThemeCSS + `

        /* Chroma syntax highlighting styles */
` + api.GetChromaCSS() + `

        ` + treeCSS + `
    </style>`

	if f.IsPDFMode {
		css += f.getPDFCSS()
	}

	css += `
</head>
<body class="bg-gray-100 min-h-screen p-6">
    <script>
        ` + tooltipsJS + `

        ` + treeJS + `
    </script>
    <div class="mx-auto px-4 space-y-8">
`
	return css
}

// getPDFCSS returns PDF-specific style overrides
func (f *HTMLFormatter) getPDFCSS() string {
	return `
    <style>
        ` + pdfCSS + `
    </style>`
}

// Format formats PrettyData into HTML output
func (f *HTMLFormatter) Format(in interface{}, options formatters.FormatOptions) (string, error) {
	// Unwrap single-element slices from varargs
	if slice, ok := in.([]interface{}); ok && len(slice) == 1 {
		in = slice[0]
	}

	if prettData, ok := in.(*api.PrettyData); ok {
		return f.FormatPrettyData(prettData)
	}

	// Check if input is a TreeNode FIRST (before Pretty check)
	// This handles single TreeNode structs like ASTNode that need recursive rendering
	if treeNode, ok := in.(api.TreeNode); ok {
		return f.formatSingleTreeNode(treeNode), nil
	}

	// Check if input implements Pretty interface
	if pretty, ok := in.(api.Pretty); ok {
		text := pretty.Pretty()
		htmlContent := text.HTML()

		if f.IncludeCSS {
			var result strings.Builder
			result.WriteString(f.getCSS())
			result.WriteString("        <div class=\"bg-white rounded-lg shadow p-6\">\n")
			result.WriteString("            ")
			result.WriteString(htmlContent)
			result.WriteString("\n        </div>\n")
			result.WriteString("    </div>\n</body>\n</html>")
			return result.String(), nil
		}
		return htmlContent, nil
	}

	// Convert to PrettyData
	data, err := f.ToPrettyData(in)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}

	if data == nil || data.Schema == nil {
		return "", nil
	}
	if data == nil || data.Schema == nil {
		return "", nil
	}

	var result strings.Builder

	if f.IncludeCSS {
		result.WriteString(f.getCSS())
	}

	// Count non-table/non-tree fields first
	summaryFieldCount := 0
	for _, field := range data.Schema.Fields {
		if field.Format != api.FormatTable && field.Format != api.FormatTree {
			if _, exists := data.GetValue(field.Name); exists {
				summaryFieldCount++
			}
		}
	}

	// Only render Summary section if there are non-table/non-tree fields
	if summaryFieldCount > 0 {
		// Summary first - add non-table fields as a summary card
		result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
		result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
		result.WriteString("                <h2 class=\"text-xl font-semibold text-gray-900\">Summary</h2>\n")
		result.WriteString("            </div>\n")
		result.WriteString("            <div class=\"px-6 py-4\">\n")
		result.WriteString("                <dl class=\"grid grid-cols-1 md:grid-cols-2 gap-4\">\n")

		// Process summary fields (non-table, non-tree, non-hidden)
		for _, field := range data.Schema.Fields {
			// Skip table and tree fields (they get special handling)
			if field.Format == api.FormatTable || field.Format == api.FormatTree {
				continue
			}

			fieldValue, exists := data.GetValue(field.Name)
			if !exists {
				continue
			}

			prettyFieldName := f.prettifyFieldName(field.Name)

			// Format field value with styling
			fieldHTML := f.formatFieldValueHTMLWithStyle(fieldValue, field)

			// Apply label styling
			var labelHTML string
			if field.LabelStyle != "" {
				labelHTML = f.applyTailwindStyleToHTML(prettyFieldName, field.LabelStyle)
			} else {
				labelHTML = fmt.Sprintf("<span class=\"text-sm font-medium text-gray-500\">%s</span>", html.EscapeString(prettyFieldName))
			}

			result.WriteString("                    <div>\n")
			result.WriteString(fmt.Sprintf("                        <dt>%s</dt>\n", labelHTML))
			result.WriteString(fmt.Sprintf("                        <dd class=\"mt-1 text-sm\">%s</dd>\n", fieldHTML))
			result.WriteString("                    </div>\n")
		}
		result.WriteString("                </dl>\n")
		result.WriteString("            </div>\n")
		result.WriteString("        </div>\n")
	}

	// Then handle tables
	for _, field := range data.Schema.Fields {
		// Check for table format
		switch field.Format {
		case api.FormatTable:
			tableData, exists := data.GetTable(field.Name)
			if exists && tableData != nil && len(tableData.Rows) > 0 {
				// Add section title
				result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
				result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
				result.WriteString(fmt.Sprintf("                <h2 class=\"text-xl font-semibold text-gray-900\">%s</h2>\n",
					f.prettifyFieldName(field.Name)))
				result.WriteString("            </div>\n")

				// Format as table - use Grid.js unless in PDF mode
				var tableHTML string
				if f.IsPDFMode {
					// Use static HTML table for PDF generation
					tableHTML = f.formatTableDataHTML(tableData, field)
				} else {
					// Use Grid.js for interactive features
					tableID := f.generateTableID()
					tableHTML = f.formatTableDataHTMLWithGridJS(tableData, field, tableID)
				}
				result.WriteString(tableHTML)
				result.WriteString("        </div>\n")
			}
		case api.FormatTree:
			// Handle tree format
			fieldValue, exists := data.GetValue(field.Name)
			if exists {
				// Add section title
				result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
				result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
				result.WriteString(fmt.Sprintf("                <h2 class=\"text-xl font-semibold text-gray-900\">%s</h2>\n",
					f.prettifyFieldName(field.Name)))
				result.WriteString("            </div>\n")
				result.WriteString("            <div class=\"px-6 py-4\">\n")

				// Format as tree with HTML styling
				treeHTML := f.formatTreeFieldHTML(fieldValue, field)
				result.WriteString(treeHTML)

				result.WriteString("            </div>\n")
				result.WriteString("        </div>\n")
			}
		}
	}

	if f.IncludeCSS {
		result.WriteString("    </div>\n</body>\n</html>")
	}

	return result.String(), nil
}

// FormatPrettyData formats PrettyData directly as HTML
func (f *HTMLFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil || data.Schema == nil {
		return "", nil
	}

	var result strings.Builder

	if f.IncludeCSS {
		result.WriteString(f.getCSS())
	}

	// Count non-table/non-tree fields first
	summaryFieldCount := 0
	for _, field := range data.Schema.Fields {
		if field.Format != api.FormatTable && field.Format != api.FormatTree {
			if _, exists := data.GetValue(field.Name); exists {
				summaryFieldCount++
			}
		}
	}

	// Only render Summary section if there are non-table/non-tree fields
	if summaryFieldCount > 0 {
		// Summary first - add non-table fields as a summary card
		result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
		result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
		result.WriteString("                <h2 class=\"text-xl font-semibold text-gray-900\">Summary</h2>\n")
		result.WriteString("            </div>\n")
		result.WriteString("            <div class=\"px-6 py-4\">\n")
		result.WriteString("                <dl class=\"grid grid-cols-1 md:grid-cols-2 gap-4\">\n")

		// Process summary fields (non-table, non-tree, non-hidden)
		for _, field := range data.Schema.Fields {
			// Skip table and tree fields (they get special handling)
			if field.Format == api.FormatTable || field.Format == api.FormatTree {
				continue
			}

			fieldValue, exists := data.GetValue(field.Name)
			if !exists {
				continue
			}

			prettyFieldName := f.prettifyFieldName(field.Name)

			// Format field value with styling
			fieldHTML := f.formatFieldValueHTMLWithStyle(fieldValue, field)

			// Apply label styling
			var labelHTML string
			if field.LabelStyle != "" {
				labelHTML = f.applyTailwindStyleToHTML(prettyFieldName, field.LabelStyle)
			} else {
				labelHTML = fmt.Sprintf("<span class=\"text-sm font-medium text-gray-500\">%s</span>", html.EscapeString(prettyFieldName))
			}

			result.WriteString("                    <div>\n")
			result.WriteString(fmt.Sprintf("                        <dt>%s</dt>\n", labelHTML))
			result.WriteString(fmt.Sprintf("                        <dd class=\"mt-1 text-sm\">%s</dd>\n", fieldHTML))
			result.WriteString("                    </div>\n")
		}
		result.WriteString("                </dl>\n")
		result.WriteString("            </div>\n")
		result.WriteString("        </div>\n")
	}

	// Then handle tables
	for _, field := range data.Schema.Fields {
		// Check for table format
		switch field.Format {
		case api.FormatTable:
			tableData, exists := data.GetTable(field.Name)
			if exists && tableData != nil && len(tableData.Rows) > 0 {
				// Add section title
				result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
				result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
				result.WriteString(fmt.Sprintf("                <h2 class=\"text-xl font-semibold text-gray-900\">%s</h2>\n",
					f.prettifyFieldName(field.Name)))
				result.WriteString("            </div>\n")

				// Format as table - use Grid.js unless in PDF mode
				var tableHTML string
				if f.IsPDFMode {
					// Use static HTML table for PDF generation
					tableHTML = f.formatTableDataHTML(tableData, field)
				} else {
					// Use Grid.js for interactive features
					tableID := f.generateTableID()
					tableHTML = f.formatTableDataHTMLWithGridJS(tableData, field, tableID)
				}
				result.WriteString(tableHTML)
				result.WriteString("        </div>\n")
			}
		case api.FormatTree:
			// Handle tree format
			fieldValue, exists := data.GetValue(field.Name)
			if exists {
				// Add section title
				result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
				result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
				result.WriteString(fmt.Sprintf("                <h2 class=\"text-xl font-semibold text-gray-900\">%s</h2>\n",
					f.prettifyFieldName(field.Name)))
				result.WriteString("            </div>\n")
				result.WriteString("            <div class=\"px-6 py-4\">\n")

				// Format as tree with HTML styling
				treeHTML := f.formatTreeFieldHTML(fieldValue, field)
				result.WriteString(treeHTML)

				result.WriteString("            </div>\n")
				result.WriteString("        </div>\n")
			}
		}
	}

	if f.IncludeCSS {
		result.WriteString("    </div>\n</body>\n</html>")
	}

	return result.String(), nil
}

// applyTailwindStyleToHTML applies Tailwind styles to HTML content
func (f *HTMLFormatter) applyTailwindStyleToHTML(text, styleStr string) string {
	if styleStr == "" {
		return html.EscapeString(text)
	}

	// Apply text transformations and get style
	transformedText, _ := tailwind.ApplyStyle(text, styleStr)

	// Escape the transformed text and wrap with style classes
	escapedText := html.EscapeString(transformedText)
	return fmt.Sprintf("<span class=\"%s\">%s</span>", styleStr, escapedText)
}

// getColorClass returns Tailwind CSS class for color
func (f *HTMLFormatter) getColorClass(color string) string {
	switch strings.ToLower(color) {
	case "green":
		return "text-green-600 font-medium"
	case "red":
		return "text-red-600 font-medium"
	case "blue":
		return "text-blue-600 font-medium"
	case "yellow":
		return "text-yellow-600 font-medium"
	case "orange":
		return "text-orange-600 font-medium"
	case "purple":
		return "text-purple-600 font-medium"
	case "gold":
		return "text-yellow-500 font-bold"
	case "silver":
		return "text-gray-500 font-medium"
	default:
		return "text-gray-900"
	}
}

// prettifyFieldName converts field names to readable format
func (f *HTMLFormatter) prettifyFieldName(name string) string {
	return PrettifyFieldName(name)
}

// formatFieldValueHTML formats a FieldValue for HTML output (legacy function)
func (f *HTMLFormatter) formatFieldValueHTML(fieldValue api.TypedValue) string {
	// This is the legacy function, now delegating to the new one with empty field
	return f.formatFieldValueHTMLWithStyle(fieldValue, api.PrettyField{})
}

// formatFieldValueHTMLWithStyle formats a TypedValue with field styling for HTML output
func (f *HTMLFormatter) formatFieldValueHTMLWithStyle(fieldValue api.TypedValue, field api.PrettyField) string {
	// Check if this is an image field
	valueStr := fieldValue.String()
	if field.Format == "image" || f.isImageURL(valueStr) {
		return f.formatImageHTML(fieldValue, field)
	}

	// Use HTML() method from TypedValue
	return fieldValue.HTML()
}

// formatNestedPrettyData formats a PrettyData structure as nested HTML
func (f *HTMLFormatter) formatNestedPrettyData(data *api.PrettyData) string {
	var result strings.Builder
	result.WriteString(`<div class="space-y-1">`)

	// Format regular fields
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatTable {
			continue // Skip tables in nested view
		}

		if fieldValue, ok := data.GetValue(field.Name); ok {
			label := field.Label
			if label == "" {
				label = f.prettifyFieldName(field.Name)
			}

			result.WriteString(`<div class="flex">`)
			result.WriteString(fmt.Sprintf(`<span class="text-gray-600 font-medium w-32 flex-shrink-0">%s:</span>`, html.EscapeString(label)))

			result.WriteString(f.formatFieldValueHTML(fieldValue))
			result.WriteString("</div>")
		}
	}

	result.WriteString("</div>")
	return result.String()
}

// formatTableDataHTML formats table data for HTML output
func (f *HTMLFormatter) formatTableDataHTML(table *api.TextTable, field api.PrettyField) string {
	if table == nil || len(table.Rows) == 0 {
		return "            <p class=\"text-gray-500 text-center py-8\">No data available</p>"
	}

	var result strings.Builder
	result.WriteString("            <div class=\"overflow-x-auto\">\n")
	result.WriteString("                <table class=\"min-w-full table-auto\">\n")

	// Write headers
	result.WriteString("                    <thead class=\"bg-gray-50\">\n")
	result.WriteString("                        <tr>\n")
	for _, tableField := range field.TableOptions.Columns {
		// Use Label for display, fallback to prettified Name if Label is empty
		headerLabel := tableField.Label
		if headerLabel == "" {
			headerLabel = f.prettifyFieldName(tableField.Name)
		}

		var headerHTML string
		if field.TableOptions.HeaderStyle != "" {
			headerHTML = f.applyTailwindStyleToHTML(headerLabel, field.TableOptions.HeaderStyle)
		} else {
			headerHTML = fmt.Sprintf("<span class=\"text-xs font-medium text-gray-500 uppercase tracking-wider\">%s</span>", html.EscapeString(headerLabel))
		}
		result.WriteString(fmt.Sprintf("                            <th class=\"px-6 py-3 text-left\">%s</th>\n", headerHTML))
	}
	result.WriteString("                        </tr>\n")
	result.WriteString("                    </thead>\n")

	// Write data rows
	result.WriteString("                    <tbody class=\"bg-white divide-y divide-gray-200\">\n")
	for _, row := range table.Rows {
		result.WriteString("                        <tr class=\"hover:bg-gray-50\">\n")
		for _, tableField := range field.TableOptions.Columns {
			fieldValue, exists := row[tableField.Name]
			var cellContent string
			if exists {
				// Apply styling with priority: tableField.Style > row_style
				if tableField.Style != "" {
					cellContent = f.formatFieldValueHTMLWithStyle(fieldValue, tableField)
				} else if field.TableOptions.RowStyle != "" {
					// Create a temporary field with row_style
					tempField := api.PrettyField{Style: field.TableOptions.RowStyle}
					cellContent = f.formatFieldValueHTMLWithStyle(fieldValue, tempField)
				} else {
					cellContent = f.formatFieldValueHTML(fieldValue)
				}
			} else {
				cellContent = ""
			}
			result.WriteString(fmt.Sprintf("                            <td class=\"px-6 py-4 whitespace-nowrap text-sm text-gray-900\">%s</td>\n", cellContent))
		}
		result.WriteString("                        </tr>\n")
	}
	result.WriteString("                    </tbody>\n")
	result.WriteString("                </table>\n")
	result.WriteString("            </div>\n")

	return result.String()
}

// formatTableDataHTMLWithGridJS formats table data using Grid.js for interactive features
func (f *HTMLFormatter) formatTableDataHTMLWithGridJS(table *api.TextTable, field api.PrettyField, tableID string) string {
	if table == nil || len(table.Rows) == 0 {
		return "            <p class=\"text-gray-500 text-center py-8\">No data available</p>"
	}

	var result strings.Builder

	// Create a div for Grid.js to mount
	result.WriteString(fmt.Sprintf("            <div id=\"%s\"></div>\n", tableID))

	// Generate JavaScript to initialize Grid.js
	result.WriteString("            <script>\n")
	result.WriteString("                document.addEventListener('DOMContentLoaded', function() {\n")
	result.WriteString("                    new gridjs.Grid({\n")

	// Configure columns
	result.WriteString("                        columns: [\n")
	for i, tableField := range field.TableOptions.Columns {
		headerLabel := tableField.Label
		if headerLabel == "" {
			headerLabel = f.prettifyFieldName(tableField.Name)
		}

		if i > 0 {
			result.WriteString(",\n")
		}

		// Format column definition with sorting and HTML rendering enabled
		result.WriteString(fmt.Sprintf("                            { name: %s, sort: true, formatter: (cell) => gridjs.html(cell) }",
			f.jsonEscape(headerLabel)))
	}
	result.WriteString("\n                        ],\n")

	// Configure data
	result.WriteString("                        data: [\n")
	for i, row := range table.Rows {
		if i > 0 {
			result.WriteString(",\n")
		}
		result.WriteString("                            [")

		for j, tableField := range field.TableOptions.Columns {
			if j > 0 {
				result.WriteString(", ")
			}

			fieldValue, exists := row[tableField.Name]
			var cellContent string
			if exists {
				// Apply styling with HTML content for Grid.js
				if tableField.Style != "" {
					cellContent = f.formatFieldValueHTMLWithStyle(fieldValue, tableField)
				} else {
					cellContent = fieldValue.HTML()
				}
			} else {
				cellContent = ""
			}
			result.WriteString(f.jsonEscape(cellContent))
		}
		result.WriteString("]")
	}
	result.WriteString("\n                        ],\n")

	// Configure Grid.js options
	result.WriteString("                        search: true,\n")
	result.WriteString("                        pagination: false,\n")
	result.WriteString("                        sort: true,\n")
	result.WriteString("                        resizable: true,\n")
	result.WriteString("                        className: {\n")
	result.WriteString("                            table: 'gridjs-table',\n")
	result.WriteString("                            th: 'gridjs-th',\n")
	result.WriteString("                            td: 'gridjs-td'\n")
	result.WriteString("                        }\n")

	result.WriteString(fmt.Sprintf("                    }).render(document.getElementById('%s')).then(() => {\n", tableID))
	result.WriteString("                        // Reinitialize tooltips after Grid.js renders the table\n")
	result.WriteString("                        if (typeof initTooltips === 'function') {\n")
	result.WriteString("                            initTooltips();\n")
	result.WriteString("                        }\n")
	result.WriteString("                    });\n")
	result.WriteString("                });\n")
	result.WriteString("            </script>\n")

	return result.String()
}

// jsonEscape properly escapes a string for use in JSON
func (f *HTMLFormatter) jsonEscape(s string) string {
	// Use Go's JSON marshaling to properly escape the string
	escaped, _ := json.Marshal(s)
	return string(escaped)
}

// generateTableID generates a unique table ID for Grid.js
func (f *HTMLFormatter) generateTableID() string {
	f.tableCounter++
	return fmt.Sprintf("gridjs-table-%d", f.tableCounter)
}

// formatTreeFieldHTML formats a tree field for HTML output
func (f *HTMLFormatter) formatTreeFieldHTML(fieldValue api.TypedValue, _ api.PrettyField) string {
	// Check if TypedValue has a tree
	if fieldValue.Tree == nil {
		return "<p class=\"text-gray-500\">No tree data available</p>"
	}

	// Format tree using HTML elements - use the TextTree as-is
	return fieldValue.Tree.String()
}

// generateNodeID generates a unique node ID for tree nodes
func (f *HTMLFormatter) generateNodeID() string {
	f.nodeCounter++
	return fmt.Sprintf("node-%d", f.nodeCounter)
}

// formatTreeNodeHTML recursively formats a tree node as HTML
func (f *HTMLFormatter) formatTreeNodeHTML(node api.TreeNode, depth int) string {
	if node == nil {
		return ""
	}

	var result strings.Builder

	// Get children
	children := node.GetChildren()

	if depth == 0 {
		// Root node - start the tree with Alpine.js data and skip root node label
		if !f.IsPDFMode {
			// Interactive mode with Alpine.js - use embedded tree function
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
			for _, child := range children {
				childHTML := f.formatTreeNodeHTML(child, depth+1)
				result.WriteString(childHTML)
			}
			result.WriteString(`</ul>`)
		}

		result.WriteString(`</div>`)
	} else {
		// Child node
		result.WriteString(`<li class="flex items-start tree-node-wrapper">`)

		if len(children) > 0 && !f.IsPDFMode {
			// Alpine.js toggle for nodes with children
			nodeID := f.generateNodeID()
			result.WriteString(fmt.Sprintf(`<span class="tree-toggle" @click.stop="toggleNode('%s')" data-node-id="%s">`, nodeID, nodeID))
			result.WriteString(fmt.Sprintf(`<iconify-icon icon="ion:chevron-forward" x-show="!isExpanded('%s')"></iconify-icon>`, nodeID))
			result.WriteString(fmt.Sprintf(`<iconify-icon icon="ion:chevron-down" x-show="isExpanded('%s')"></iconify-icon>`, nodeID))
			result.WriteString(`</span>`)
		} else {
			// Static indicator for leaf nodes
			result.WriteString(`<span class="tree-leaf-indicator">•</span>`)
		}

		result.WriteString(`<div class="flex-1">`)
		if len(children) > 0 && !f.IsPDFMode {
			nodeID := fmt.Sprintf("node-%d", f.nodeCounter)
			result.WriteString(fmt.Sprintf(`<span class="tree-node cursor-pointer" @click="toggleNode('%s')">`, nodeID))
		} else {
			result.WriteString(`<span class="tree-node">`)
		}
		result.WriteString(node.Pretty().HTML())
		result.WriteString(`</span>`)

		if len(children) > 0 {
			if !f.IsPDFMode {
				nodeID := fmt.Sprintf("node-%d", f.nodeCounter)
				result.WriteString(fmt.Sprintf(`<ul class="tree-children ml-4 mt-1 space-y-1" x-show="isExpanded('%s')">`, nodeID))
			} else {
				result.WriteString(`<ul class="tree-children ml-4 mt-1 space-y-1">`)
			}
			for _, child := range children {
				childHTML := f.formatTreeNodeHTML(child, depth+1)
				result.WriteString(childHTML)
			}
			result.WriteString(`</ul>`)
		}

		result.WriteString(`</div>`)
		result.WriteString(`</li>`)
	}

	return result.String()
}

// formatSingleTreeNode handles rendering when a single TreeNode struct is passed
// directly to Format(). It wraps the tree in proper HTML structure and recursively
// renders all children.
func (f *HTMLFormatter) formatSingleTreeNode(root api.TreeNode) string {
	if root == nil {
		return ""
	}

	var result strings.Builder

	if f.IncludeCSS {
		result.WriteString(f.getCSS())
	}

	// Wrap in card structure
	result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
	result.WriteString("            <div class=\"px-6 py-4\">\n")

	// Render the tree recursively starting from root
	treeHTML := f.formatTreeNodeHTML(root, 0)
	result.WriteString(treeHTML)

	result.WriteString("            </div>\n")
	result.WriteString("        </div>\n")

	if f.IncludeCSS {
		result.WriteString("    </div>\n</body>\n</html>")
	}

	return result.String()
}

// isImageURL checks if a string is likely an image URL
func (f *HTMLFormatter) isImageURL(s string) bool {
	s = strings.ToLower(s)

	// Check for data URLs (base64 encoded images)
	if strings.HasPrefix(s, "data:image/") {
		return true
	}

	// Check for common image file extensions
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico"}
	for _, ext := range imageExts {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}

	// Check for URLs that might be images
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		for _, ext := range imageExts {
			if strings.Contains(s, ext) {
				return true
			}
		}
		// Check for common image hosting patterns
		if strings.Contains(s, "images") || strings.Contains(s, "img") ||
			strings.Contains(s, "photo") || strings.Contains(s, "picture") ||
			strings.Contains(s, "media") || strings.Contains(s, "cdn") {
			return true
		}
	}

	return false
}

// formatImageHTML formats an image field as HTML
func (f *HTMLFormatter) formatImageHTML(fieldValue api.TypedValue, field api.PrettyField) string {
	imageURL := fieldValue.String()

	// Get image options from field
	width := "auto"
	height := "auto"
	alt := field.Label
	if alt == "" {
		alt = field.Name
	}

	// Check format options for width/height
	if field.FormatOptions != nil {
		if w, ok := field.FormatOptions["width"]; ok {
			width = w
		}
		if h, ok := field.FormatOptions["height"]; ok {
			height = h
		}
		if a, ok := field.FormatOptions["alt"]; ok {
			alt = a
		}
	}

	// Build style attribute
	styleAttrs := []string{}
	if width != "auto" {
		if strings.HasSuffix(width, "%") || strings.HasSuffix(width, "px") {
			styleAttrs = append(styleAttrs, fmt.Sprintf("width: %s", width))
		} else {
			styleAttrs = append(styleAttrs, fmt.Sprintf("width: %spx", width))
		}
	}
	if height != "auto" {
		if strings.HasSuffix(height, "%") || strings.HasSuffix(height, "px") {
			styleAttrs = append(styleAttrs, fmt.Sprintf("height: %s", height))
		} else {
			styleAttrs = append(styleAttrs, fmt.Sprintf("height: %spx", height))
		}
	}

	style := ""
	if len(styleAttrs) > 0 {
		style = fmt.Sprintf(` style="%s"`, strings.Join(styleAttrs, "; "))
	}

	// Generate HTML
	return fmt.Sprintf(`<img src="%s" alt="%s" class="rounded-lg shadow-md" loading="lazy"%s>`,
		html.EscapeString(imageURL), html.EscapeString(alt), style)
}
