package formatters

import (
	"fmt"
	"html"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
)

// HTMLFormatter handles HTML formatting
type HTMLFormatter struct {
	IncludeCSS bool
}

// NewHTMLFormatter creates a new HTML formatter
func NewHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{
		IncludeCSS: true,
	}
}

// ToPrettyData converts various input types to PrettyData
func (f *HTMLFormatter) ToPrettyData(data interface{}) (*api.PrettyData, error) {
	return ToPrettyData(data)
}

// getCSS returns Tailwind CSS CDN and custom styling
func (f *HTMLFormatter) getCSS() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Clicky Output</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 min-h-screen p-6">
    <div class="max-w-7xl mx-auto space-y-8">
`
}

// Format formats PrettyData into HTML output
func (f *HTMLFormatter) Format(in interface{}) (string, error) {
	// Check if input implements Pretty interface first
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

	// Then handle tables
	for _, field := range data.Schema.Fields {
		// Check for table format
		if field.Format == api.FormatTable {
			tableData, exists := data.GetTable(field.Name)
			if exists && len(tableData) > 0 {
				// Add section title
				result.WriteString(fmt.Sprintf("        <div class=\"bg-white rounded-lg shadow\">\n"))
				result.WriteString(fmt.Sprintf("            <div class=\"px-6 py-4 border-b border-gray-200\">\n"))
				result.WriteString(fmt.Sprintf("                <h2 class=\"text-xl font-semibold text-gray-900\">%s</h2>\n",
					f.prettifyFieldName(field.Name)))
				result.WriteString("            </div>\n")

				// Format as table with Tailwind styling
				tableHTML := f.formatTableDataHTML(tableData, field)
				result.WriteString(tableHTML)
				result.WriteString("        </div>\n")
			}
		} else if field.Format == api.FormatTree {
			// Handle tree format
			fieldValue, exists := data.GetValue(field.Name)
			if exists {
				// Add section title
				result.WriteString(fmt.Sprintf("        <div class=\"bg-white rounded-lg shadow\">\n"))
				result.WriteString(fmt.Sprintf("            <div class=\"px-6 py-4 border-b border-gray-200\">\n"))
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
func (f *HTMLFormatter) applyTailwindStyleToHTML(text string, styleStr string) string {
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

// splitCamelCase splits camelCase strings into words
func (f *HTMLFormatter) splitCamelCase(s string) []string {
	return SplitCamelCase(s)
}

// formatFieldValueHTML formats a FieldValue for HTML output (legacy function)
func (f *HTMLFormatter) formatFieldValueHTML(fieldValue api.FieldValue) string {
	// This is the legacy function, now delegating to the new one with empty field
	return f.formatFieldValueHTMLWithStyle(fieldValue, api.PrettyField{})
}

// formatFieldValueHTMLWithStyle formats a FieldValue with field styling for HTML output
func (f *HTMLFormatter) formatFieldValueHTMLWithStyle(fieldValue api.FieldValue, field api.PrettyField) string {
	// Check if value implements Pretty interface first
	if fieldValue.Value != nil {
		if pretty, ok := fieldValue.Value.(api.Pretty); ok {
			text := pretty.Pretty()
			return text.HTML()
		}
	}

	// Check if this is an image field
	if field.Format == "image" || f.isImageURL(fieldValue.Formatted()) {
		return f.formatImageHTML(fieldValue, field)
	}

	// Handle nested fields by formatting them as HTML
	if fieldValue.HasNestedFields() {
		return f.formatNestedFieldValue(fieldValue)
	}

	formatted := fieldValue.Formatted()

	// Apply field style if specified (highest priority)
	if field.Style != "" {
		return f.applyTailwindStyleToHTML(formatted, field.Style)
	}

	// Apply color styling using FieldValue.Color()
	if color := fieldValue.Color(); color != "" {
		return fmt.Sprintf("<span class=\"%s\">%s</span>", f.getColorClass(color), html.EscapeString(formatted))
	}

	// Check for special formatting
	if fieldValue.Field.Format == api.FormatCurrency {
		return fmt.Sprintf("<span class=\"text-green-600 font-medium\">%s</span>", html.EscapeString(formatted))
	}

	if fieldValue.Field.Format == api.FormatDate {
		return fmt.Sprintf("<span class=\"text-blue-600\">%s</span>", html.EscapeString(formatted))
	}

	return fmt.Sprintf("<span class=\"text-gray-900\">%s</span>", html.EscapeString(formatted))
}

// formatNestedFieldValue formats a FieldValue with nested fields as HTML
func (f *HTMLFormatter) formatNestedFieldValue(fieldValue api.FieldValue) string {
	var result strings.Builder
	result.WriteString(`<div class="space-y-1">`)

	keys := fieldValue.GetNestedFieldKeys()
	for _, key := range keys {
		nestedField, _ := fieldValue.GetNestedField(key)
		prettyKey := f.prettifyFieldName(key)

		result.WriteString(`<div class="flex">`)
		result.WriteString(fmt.Sprintf(`<span class="text-gray-600 font-medium w-32 flex-shrink-0">%s:</span>`, html.EscapeString(prettyKey)))

		if nestedField.HasNestedFields() {
			result.WriteString(`<div class="ml-4">`)
			result.WriteString(f.formatNestedFieldValue(nestedField))
			result.WriteString("</div>")
		} else {
			result.WriteString(f.formatFieldValueHTML(nestedField))
		}
		result.WriteString("</div>")
	}

	result.WriteString("</div>")
	return result.String()
}

// formatTableDataHTML formats table data for HTML output
func (f *HTMLFormatter) formatTableDataHTML(rows []api.PrettyDataRow, field api.PrettyField) string {
	if len(rows) == 0 {
		return "            <p class=\"text-gray-500 text-center py-8\">No data available</p>"
	}

	var result strings.Builder
	result.WriteString("            <div class=\"overflow-x-auto\">\n")
	result.WriteString("                <table class=\"min-w-full table-auto\">\n")

	// Write headers
	result.WriteString("                    <thead class=\"bg-gray-50\">\n")
	result.WriteString("                        <tr>\n")
	for _, tableField := range field.TableOptions.Fields {
		var headerHTML string
		if field.TableOptions.HeaderStyle != "" {
			headerHTML = f.applyTailwindStyleToHTML(tableField.Name, field.TableOptions.HeaderStyle)
		} else {
			headerHTML = fmt.Sprintf("<span class=\"text-xs font-medium text-gray-500 uppercase tracking-wider\">%s</span>", html.EscapeString(tableField.Name))
		}
		result.WriteString(fmt.Sprintf("                            <th class=\"px-6 py-3 text-left\">%s</th>\n", headerHTML))
	}
	result.WriteString("                        </tr>\n")
	result.WriteString("                    </thead>\n")

	// Write data rows
	result.WriteString("                    <tbody class=\"bg-white divide-y divide-gray-200\">\n")
	for _, row := range rows {
		result.WriteString("                        <tr class=\"hover:bg-gray-50\">\n")
		for _, tableField := range field.TableOptions.Fields {
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

// formatTreeFieldHTML formats a tree field for HTML output
func (f *HTMLFormatter) formatTreeFieldHTML(fieldValue api.FieldValue, field api.PrettyField) string {
	// Convert value to tree node
	var node api.TreeNode
	if fieldValue.Value != nil {
		if treeNode, ok := fieldValue.Value.(api.TreeNode); ok {
			node = treeNode
		} else {
			node = ConvertToTreeNode(fieldValue.Value)
		}
	}

	if node == nil {
		return "<p class=\"text-gray-500\">No tree data available</p>"
	}

	// Format tree using HTML elements
	return f.formatTreeNodeHTML(node, 0)
}

// formatTreeNodeHTML recursively formats a tree node as HTML
func (f *HTMLFormatter) formatTreeNodeHTML(node api.TreeNode, depth int) string {
	if node == nil {
		return ""
	}

	var result strings.Builder

	// Format current node
	var nodeContent string
	if prettyNode, ok := node.(api.Pretty); ok {
		// Use Pretty() method for rich HTML formatting
		text := prettyNode.Pretty()
		nodeContent = text.HTML()
	} else {
		// Fallback to GetLabel() with HTML escaping
		label := node.GetLabel()
		// Add icon if present
		if icon := node.GetIcon(); icon != "" {
			nodeContent = html.EscapeString(icon) + " " + html.EscapeString(label)
		} else {
			nodeContent = html.EscapeString(label)
		}
		// Apply style if present
		if style := node.GetStyle(); style != "" {
			nodeContent = fmt.Sprintf(`<span class="%s">%s</span>`, style, nodeContent)
		}
	}

	// Get children
	children := node.GetChildren()

	if depth == 0 {
		// Root node - start the tree
		result.WriteString(`<div class="tree-view">`)
		result.WriteString(`<div class="tree-node font-semibold text-lg mb-2">`)
		result.WriteString(nodeContent)
		result.WriteString(`</div>`)

		if len(children) > 0 {
			result.WriteString(`<ul class="ml-4 space-y-1">`)
			for _, child := range children {
				childHTML := f.formatTreeNodeHTML(child, depth+1)
				result.WriteString(childHTML)
			}
			result.WriteString(`</ul>`)
		}

		result.WriteString(`</div>`)
	} else {
		// Child node
		result.WriteString(`<li class="flex items-start">`)
		result.WriteString(`<span class="text-gray-400 mr-2">`)
		if len(children) > 0 {
			result.WriteString(`▸`) // or use a different tree connector symbol
		} else {
			result.WriteString(`•`) // leaf node indicator
		}
		result.WriteString(`</span>`)
		result.WriteString(`<div class="flex-1">`)
		result.WriteString(`<div class="tree-node">`)
		result.WriteString(nodeContent)
		result.WriteString(`</div>`)

		if len(children) > 0 {
			result.WriteString(`<ul class="ml-4 mt-1 space-y-1">`)
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
func (f *HTMLFormatter) formatImageHTML(fieldValue api.FieldValue, field api.PrettyField) string {
	imageURL := fieldValue.Formatted()

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
