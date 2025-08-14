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

	// Process summary fields (non-table, non-hidden)
	for _, field := range data.Schema.Fields {
		// Skip table fields
		if field.Format == api.FormatTable {
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
