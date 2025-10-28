package formatters

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
)

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
        /* Grid.js theme customizations to match Tailwind */
        .gridjs-wrapper {
            border: 1px solid #e5e7eb;
            border-radius: 0.5rem;
            overflow: hidden;
            box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06);
        }
        .gridjs-head {
            background: #f9fafb;
            border-bottom: 1px solid #e5e7eb;
        }
        .gridjs-th {
            background: #f9fafb;
            color: #6b7280;
            font-weight: 500;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            padding: 0.75rem 1.5rem;
            border-right: 1px solid #f3f4f6;
        }
        .gridjs-th:last-child {
            border-right: none;
        }
        .gridjs-td {
            padding: 1rem 1.5rem;
            font-size: 0.875rem;
            color: #111827;
            border-right: 1px solid #f9fafb;
            vertical-align: top;
        }
        .gridjs-td:last-child {
            border-right: none;
        }
        .gridjs-tr:nth-child(even) .gridjs-td {
            background-color: #fafafa;
        }
        .gridjs-tr:hover .gridjs-td {
            background: #f3f4f6;
        }
        .gridjs-search {
            margin-bottom: 1rem;
        }
        .gridjs-search-input {
            border: 1px solid #d1d5db;
            border-radius: 0.375rem;
            padding: 0.5rem 0.75rem;
            font-size: 0.875rem;
            width: 300px;
            transition: border-color 0.15s ease-in-out;
        }
        .gridjs-search-input:focus {
            outline: none;
            border-color: #3b82f6;
            box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
        }
        .gridjs-pagination {
            margin-top: 1rem;
            display: flex;
            justify-content: center;
            align-items: center;
        }
        .gridjs-pagination .gridjs-pages {
            margin: 0 0.5rem;
        }
        .gridjs-pagination button {
            padding: 0.5rem 0.75rem;
            margin: 0 0.25rem;
            border: 1px solid #d1d5db;
            border-radius: 0.375rem;
            background: white;
            color: #6b7280;
            font-size: 0.875rem;
            transition: all 0.15s ease-in-out;
        }
        .gridjs-pagination button:hover:not(:disabled) {
            background: #f9fafb;
            border-color: #9ca3af;
        }
        .gridjs-pagination button:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
        .gridjs-pagination .gridjs-currentPage {
            background: #3b82f6;
            color: white;
            border-color: #3b82f6;
        }

        /* Chroma syntax highlighting styles */
` + api.GetChromaCSS() + `

        /* Tree expand/collapse styles */
        .tree-toggle {
            cursor: pointer;
            user-select: none;
            display: inline-block;
            width: 1em;
            margin-right: 0.25em;
            transition: transform 0.2s ease;
            color: #6b7280;
        }

        .tree-toggle:hover {
            color: #374151;
        }

        .tree-toggle.expanded {
            transform: rotate(90deg);
        }

        .tree-node-wrapper {
            position: relative;
        }

        .tree-leaf-indicator {
            display: inline-block;
            width: 1em;
            margin-right: 0.25em;
            color: #9ca3af;
        }
    </style>`

	if f.IsPDFMode {
		css += f.getPDFCSS()
	}

	css += `
</head>
<body class="bg-gray-100 min-h-screen p-6">
    <script>
        // Global Tippy.js initialization function
        function initTooltips(container) {
            const target = container || document.body;
            // Use singleton for better performance with many tooltips
            tippy('[title]', {
                content(reference) {
                    const title = reference.getAttribute('title');
                    reference.removeAttribute('title');
                    reference.classList.add('tooltip-target');
                    return title;
                },
                allowHTML: true,
                theme: 'light-border',
                placement: 'top',
                arrow: true,
                animation: 'shift-away',
                duration: [200, 150],
                delay: [0, 0],
                appendTo: () => document.body,
            });
        }

        // Initialize tooltips on page load
        document.addEventListener('DOMContentLoaded', function() {
            initTooltips();
        });
    </script>
    <div class="mx-auto px-4 space-y-8">
`
	return css
}

// getPDFCSS returns PDF-specific style overrides
func (f *HTMLFormatter) getPDFCSS() string {
	return `
    <style>
        /* PDF-specific overrides */
        @media print, screen {
            body {
                font-size: 12px !important;
                line-height: 1.4 !important;
                margin: 0 !important;
                padding: 20px !important;
                min-height: auto !important;
                background: white !important;
            }

            .max-w-7xl {
                max-width: 100% !important;
                margin: 0 !important;
            }

            /* Reduce all font sizes by ~15% */
            .text-xl { font-size: 16px !important; }
            .text-lg { font-size: 14px !important; }
            .text-base { font-size: 12px !important; }
            .text-sm { font-size: 11px !important; }
            .text-xs { font-size: 10px !important; }

            /* Compact spacing - reduce by ~40% */
            .p-6 { padding: 12px !important; }
            .px-6 { padding-left: 12px !important; padding-right: 12px !important; }
            .py-4 { padding-top: 8px !important; padding-bottom: 8px !important; }
            .py-3 { padding-top: 6px !important; padding-bottom: 6px !important; }
            .px-4 { padding-left: 8px !important; padding-right: 8px !important; }
            .space-y-8 > * + * { margin-top: 16px !important; }
            .space-y-4 > * + * { margin-top: 8px !important; }
            .space-y-1 > * + * { margin-top: 2px !important; }
            .gap-4 { gap: 8px !important; }
            .mt-1 { margin-top: 2px !important; }
            .mb-2 { margin-bottom: 4px !important; }
            .ml-4 { margin-left: 8px !important; }
            .mr-2 { margin-right: 4px !important; }

            /* Remove responsive grid - always use 2 columns for better space usage */
            .md\\:grid-cols-2 { grid-template-columns: repeat(2, minmax(0, 1fr)) !important; }
            .grid-cols-1 { grid-template-columns: repeat(2, minmax(0, 1fr)) !important; }

            /* No overflow scrolling - tables should fit */
            .overflow-x-auto { overflow: visible !important; }

            /* Ensure tables fit and are compact */
            table {
                width: 100% !important;
                font-size: 11px !important;
                table-layout: fixed !important;
            }

            table th {
                padding: 4px 8px !important;
                font-size: 10px !important;
            }

            table td {
                padding: 4px 8px !important;
                font-size: 11px !important;
                word-wrap: break-word !important;
            }

            .whitespace-nowrap {
                white-space: normal !important;
            }

            /* Remove hover states */
            .hover\\:bg-gray-50:hover { background: transparent !important; }

            /* Remove shadows and use simple borders for cleaner print */
            .shadow {
                box-shadow: none !important;
                border: 1px solid #e5e7eb !important;
            }
            .rounded-lg { border-radius: 4px !important; }

            /* Page break handling */
            .bg-white.rounded-lg {
                page-break-inside: avoid;
                margin-bottom: 8px !important;
            }

            /* Tree view adjustments */
            .tree-view {
                font-size: 11px !important;
            }

            .tree-node {
                font-size: 11px !important;
            }

            /* Tree expand/collapse - disable in PDF mode */
            .tree-toggle {
                display: none !important;
            }

            /* Header adjustments */
            h2 {
                font-size: 14px !important;
                margin-bottom: 4px !important;
            }

            /* Definition list adjustments */
            dl {
                font-size: 11px !important;
            }

            dt {
                font-size: 10px !important;
                margin-bottom: 1px !important;
            }

            dd {
                font-size: 11px !important;
                margin-bottom: 4px !important;
            }
        }
    </style>`
}

// Format formats PrettyData into HTML output
func (f *HTMLFormatter) Format(in interface{}) (string, error) {
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
			if exists && len(tableData) > 0 {
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
			if exists && len(tableData) > 0 {
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

	// Handle nested PrettyData structures
	if nestedData, ok := fieldValue.Value.(*api.PrettyData); ok {
		return f.formatNestedPrettyData(nestedData)
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

// formatNestedPrettyData formats a PrettyData structure as nested HTML
func (f *HTMLFormatter) formatNestedPrettyData(data *api.PrettyData) string {
	var result strings.Builder
	result.WriteString(`<div class="space-y-1">`)

	// Format regular fields
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatTable {
			continue // Skip tables in nested view
		}

		if fieldValue, ok := data.Values[field.Name]; ok {
			label := field.Label
			if label == "" {
				label = f.prettifyFieldName(field.Name)
			}

			result.WriteString(`<div class="flex">`)
			result.WriteString(fmt.Sprintf(`<span class="text-gray-600 font-medium w-32 flex-shrink-0">%s:</span>`, html.EscapeString(label)))

			// Check for further nesting
			if nestedData, ok := fieldValue.Value.(*api.PrettyData); ok {
				result.WriteString(`<div class="ml-4">`)
				result.WriteString(f.formatNestedPrettyData(nestedData))
				result.WriteString("</div>")
			} else {
				result.WriteString(f.formatFieldValueHTML(fieldValue))
			}
			result.WriteString("</div>")
		}
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

// formatTableDataHTMLWithGridJS formats table data using Grid.js for interactive features
func (f *HTMLFormatter) formatTableDataHTMLWithGridJS(rows []api.PrettyDataRow, field api.PrettyField, tableID string) string {
	if len(rows) == 0 {
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
	for i, tableField := range field.TableOptions.Fields {
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
	for i, row := range rows {
		if i > 0 {
			result.WriteString(",\n")
		}
		result.WriteString("                            [")

		for j, tableField := range field.TableOptions.Fields {
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
func (f *HTMLFormatter) formatTreeFieldHTML(fieldValue api.FieldValue, _ api.PrettyField) string {
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
		// Root node - start the tree with Alpine.js data
		if !f.IsPDFMode {
			// Interactive mode with Alpine.js
			result.WriteString(`<div class="tree-view" x-data="{`)
			result.WriteString(`expandedNodes: new Set(),`)
			result.WriteString(`expandAll() {`)
			result.WriteString(`const nodes = this.$el.querySelectorAll('[data-node-id]');`)
			result.WriteString(`nodes.forEach(n => this.expandedNodes.add(n.dataset.nodeId));`)
			result.WriteString(`},`)
			result.WriteString(`collapseAll() {`)
			result.WriteString(`this.expandedNodes.clear();`)
			result.WriteString(`},`)
			result.WriteString(`toggleNode(id) {`)
			result.WriteString(`if (this.expandedNodes.has(id)) {`)
			result.WriteString(`this.expandedNodes.delete(id);`)
			result.WriteString(`} else {`)
			result.WriteString(`this.expandedNodes.add(id);`)
			result.WriteString(`}`)
			result.WriteString(`},`)
			result.WriteString(`isExpanded(id) {`)
			result.WriteString(`return this.expandedNodes.has(id);`)
			result.WriteString(`}`)
			result.WriteString(`}" x-init="expandAll()">`)

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

		result.WriteString(`<div class="tree-node-wrapper">`)

		if len(children) > 0 && !f.IsPDFMode {
			// Add Alpine.js toggle for nodes with children
			nodeID := f.generateNodeID()
			result.WriteString(fmt.Sprintf(`<span class="tree-toggle" :class="isExpanded('%s') ? 'expanded' : ''" @click="toggleNode('%s')" data-node-id="%s">▸</span>`, nodeID, nodeID, nodeID))
		}

		result.WriteString(`<span class="tree-node font-semibold text-lg mb-2">`)
		result.WriteString(node.Pretty().HTML())
		result.WriteString(`</span>`)
		result.WriteString(`</div>`)

		if len(children) > 0 {
			if !f.IsPDFMode {
				nodeID := fmt.Sprintf("node-%d", f.nodeCounter)
				result.WriteString(fmt.Sprintf(`<ul class="tree-children ml-4 space-y-1" x-show="isExpanded('%s')" x-transition>`, nodeID))
			} else {
				result.WriteString(`<ul class="tree-children ml-4 space-y-1">`)
			}
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
			result.WriteString(fmt.Sprintf(`<span class="tree-toggle" :class="isExpanded('%s') ? 'expanded' : ''" @click="toggleNode('%s')" data-node-id="%s">▸</span>`, nodeID, nodeID, nodeID))
		} else {
			// Static indicator for leaf nodes
			result.WriteString(`<span class="tree-leaf-indicator">•</span>`)
		}

		result.WriteString(`<div class="flex-1">`)
		result.WriteString(`<span class="tree-node">`)
		result.WriteString(node.Pretty().HTML())
		result.WriteString(`</span>`)

		if len(children) > 0 {
			if !f.IsPDFMode {
				nodeID := fmt.Sprintf("node-%d", f.nodeCounter)
				result.WriteString(fmt.Sprintf(`<ul class="tree-children ml-4 mt-1 space-y-1" x-show="isExpanded('%s')" x-transition>`, nodeID))
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
