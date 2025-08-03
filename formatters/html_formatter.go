package formatters

import (
	"github.com/flanksource/clicky/api"
	"fmt"
	"html"
	"strings"
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
func (f *HTMLFormatter) Format(data *api.PrettyData) (string, error) {
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
		if field.Format == "table" {
			continue
		}

		fieldValue, exists := data.GetValue(field.Name)
		if !exists {
			continue
		}

		prettyFieldName := f.prettifyFieldName(field.Name)

		// Format field value with color styling
		fieldHTML := f.formatFieldValueHTML(fieldValue)

		result.WriteString("                    <div>\n")
		result.WriteString(fmt.Sprintf("                        <dt class=\"text-sm font-medium text-gray-500\">%s</dt>\n", html.EscapeString(prettyFieldName)))
		result.WriteString(fmt.Sprintf("                        <dd class=\"mt-1 text-sm\">%s</dd>\n", fieldHTML))
		result.WriteString("                    </div>\n")
	}
	result.WriteString("                </dl>\n")
	result.WriteString("            </div>\n")
	result.WriteString("        </div>\n")

	// Then handle tables
	for _, field := range data.Schema.Fields {
		// Check for table format
		if field.Format == "table" {
			tableData, exists := data.GetTable(field.Name)
			if exists && len(tableData) > 0 {
				// Add section title
				result.WriteString(fmt.Sprintf("        <div class=\"bg-white rounded-lg shadow\">\n"))
				result.WriteString(fmt.Sprintf("            <div class=\"px-6 py-4 border-b border-gray-200\">\n"))
				result.WriteString(fmt.Sprintf("                <h2 class=\"text-xl font-semibold text-gray-900\">%s</h2>\n",
					f.prettifyFieldName(field.Name)))
				result.WriteString("            </div>\n")

				// Format as table with Tailwind styling
				tableHTML, err := f.formatTableDataHTML(tableData, field)
				if err != nil {
					return "", err
				}
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
	// Convert snake_case and camelCase to Title Case
	var result strings.Builder
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})

	if len(words) == 0 {
		// Handle camelCase
		words = f.splitCamelCase(name)
	}

	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(strings.Title(strings.ToLower(word)))
	}

	return result.String()
}

// splitCamelCase splits camelCase strings into words
func (f *HTMLFormatter) splitCamelCase(s string) []string {
	var words []string
	var current strings.Builder

	for i, r := range s {
		if i > 0 && (r >= 'A' && r <= 'Z') {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// formatFieldValueHTML formats a FieldValue for HTML output
func (f *HTMLFormatter) formatFieldValueHTML(fieldValue api.FieldValue) string {
	// Handle nested fields by formatting them as HTML
	if fieldValue.HasNestedFields() {
		return f.formatNestedFieldValue(fieldValue)
	}

	formatted := fieldValue.Formatted()

	// Apply color styling using FieldValue.Color()
	if color := fieldValue.Color(); color != "" {
		return fmt.Sprintf("<span class=\"%s\">%s</span>", f.getColorClass(color), html.EscapeString(formatted))
	}

	// Check for special formatting
	if fieldValue.Field.Format == "currency" {
		return fmt.Sprintf("<span class=\"text-green-600 font-medium\">%s</span>", html.EscapeString(formatted))
	}

	if fieldValue.Field.Format == "date" {
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
func (f *HTMLFormatter) formatTableDataHTML(rows []api.PrettyDataRow, field api.PrettyField) (string, error) {
	if len(rows) == 0 {
		return "            <p class=\"text-gray-500 text-center py-8\">No data available</p>", nil
	}

	var result strings.Builder
	result.WriteString("            <div class=\"overflow-x-auto\">\n")
	result.WriteString("                <table class=\"min-w-full table-auto\">\n")

	// Write headers
	result.WriteString("                    <thead class=\"bg-gray-50\">\n")
	result.WriteString("                        <tr>\n")
	for _, tableField := range field.TableOptions.Fields {
		result.WriteString(fmt.Sprintf("                            <th class=\"px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider\">%s</th>\n", html.EscapeString(tableField.Name)))
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
				cellContent = f.formatFieldValueHTML(fieldValue)
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

	return result.String(), nil
}
