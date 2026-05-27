package formatters

import (
	_ "embed"
	"fmt"
	"html"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
	assets "github.com/flanksource/clicky/formatters/html"
)

var treeCSS = assets.TreeCSS

var treeJS = assets.TreeJS

var gridjsThemeCSS = assets.GridjsThemeCSS

var pdfCSS = assets.PDFCSS

var tooltipsJS = assets.TooltipsJS

func init() {
	html := NewHTMLFormatter()
	RegisterFormatter("html", html.Format)
	htmlStatic := NewStaticHTMLFormatter()
	RegisterFormatter("html-static", htmlStatic.Format)
	email := NewEmailHTMLFormatter()
	RegisterFormatter("email", email.Format)
}

// NewStaticHTMLFormatter creates an HTML formatter that uses static HTML
// without JavaScript dependencies (Grid.js, Alpine.js). Suitable for PDF
// generation or environments where JavaScript may not execute.
func NewStaticHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{
		IncludeCSS: true,
		IsPDFMode:  true,
	}
}

// HTMLFormatter handles HTML formatting
type HTMLFormatter struct {
	IncludeCSS bool
	IsPDFMode  bool
	EmailMode  bool
}

// NewHTMLFormatter creates a new HTML formatter
func NewHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{
		IncludeCSS: true,
	}
}

// NewEmailHTMLFormatter creates an HTML formatter suitable for email clients.
func NewEmailHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{
		IncludeCSS: true,
		EmailMode:  true,
	}
}

// ToPrettyData converts various input types to PrettyData
func (f *HTMLFormatter) ToPrettyData(data interface{}) (*api.PrettyData, error) {
	return ToPrettyDataWithOptions(data, FormatOptions{Format: "html"})
}

// getCSS returns Tailwind CSS CDN and custom styling
func (f *HTMLFormatter) getCSS() string {
	if f.EmailMode {
		return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        table { border-collapse: collapse; }
        th, td { border: 1px solid #e5e7eb; padding: 4px 8px; text-align: left; }
        th { background-color: #f9fafb; font-weight: 600; }
    </style>
</head>
<body style="font-family: Arial, sans-serif;">
    <div style="max-width: 600px; margin: 0 auto; padding: 0 16px;">
`
	}
	css := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="https://unpkg.com/gridjs/dist/theme/mermaid.min.css" rel="stylesheet" />
    <script src="https://unpkg.com/gridjs/dist/gridjs.umd.js"></script>
    <script src="https://code.iconify.design/iconify-icon/2.0.0/iconify-icon.min.js"></script>
		`

	if !f.IsPDFMode {
		css += `
    <script src="https://unpkg.com/@popperjs/core@2"></script>
    <script src="https://unpkg.com/tippy.js@6"></script>
    <link rel="stylesheet" href="https://unpkg.com/tippy.js@6/dist/tippy.css" />
    <script defer src="https://cdn.jsdelivr.net/npm/@alpinejs/collapse@3.x.x/dist/cdn.min.js"></script>
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>`
	}

	css += `
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
<body class="bg-gray-100 min-h-screen p-6">`

	if !f.IsPDFMode {
		css += `
    <script>
        ` + tooltipsJS + `

        ` + treeJS + `
    </script>`
	}

	css += `
    <div class="mx-auto px-4 space-y-8">
`
	return css
}

// wrapHTMLBody wraps a pre-rendered HTML fragment in the same shell the
// Pretty branch uses, so callers that have produced their own structured
// HTML (e.g. StackTrace.HTML) get the surrounding Tailwind chrome and CSS
// links without re-implementing the boilerplate.
func (f *HTMLFormatter) wrapHTMLBody(body string) string {
	if !f.IncludeCSS {
		return body
	}
	var result strings.Builder
	result.WriteString(f.getCSS())
	result.WriteString("        <div class=\"bg-white rounded-lg shadow p-6\">\n")
	result.WriteString("            ")
	result.WriteString(body)
	result.WriteString("\n        </div>\n")
	result.WriteString("    </div>\n</body>\n</html>")
	return result.String()
}

// getPDFCSS returns PDF-specific style overrides
func (f *HTMLFormatter) getPDFCSS() string {
	return `
    <style>
        ` + pdfCSS + `
    </style>`
}

// Format formats PrettyData into HTML output
func (f *HTMLFormatter) Format(in interface{}, options FormatOptions) (string, error) {
	// Use a local copy to avoid mutating the shared formatter instance
	formatter := *f
	if options.PDF {
		formatter.IsPDFMode = true
	}
	return formatter.format(in, options)
}

// format is the internal implementation that performs the actual formatting.
func (f *HTMLFormatter) format(in interface{}, options FormatOptions) (string, error) {
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

	// StackTrace ships its own structured HTML renderer (semantic frame blocks,
	// monospaced gutter, syntax-highlighted source) that the generic
	// Text-tree-via-Pretty path cannot reproduce. Handle it before the Pretty
	// branch so the richer output wins.
	if trace, ok := in.(api.StackTrace); ok {
		return f.wrapHTMLBody(trace.HTML()), nil
	}
	if trace, ok := in.(*api.StackTrace); ok {
		if trace != nil {
			return f.wrapHTMLBody(trace.HTML()), nil
		}
		return "", nil
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
			if f.EmailMode {
				result.WriteString(f.formatTextableForEmail(text))
			} else {
				result.WriteString(htmlContent)
			}
			result.WriteString("\n        </div>\n")
			result.WriteString("    </div>\n</body>\n</html>")
			return result.String(), nil
		}
		if f.EmailMode {
			return f.formatTextableForEmail(text), nil
		}
		return htmlContent, nil
	}

	// Handle TextTable directly for PDF mode
	if table, ok := in.(api.TextTable); ok {
		return f.renderTableAsHTML(&table, api.PrettyField{Name: "table"})
	}
	if table, ok := in.(*api.TextTable); ok {
		return f.renderTableAsHTML(table, api.PrettyField{Name: "table"})
	}

	// Handle TextTree directly for PDF mode
	if tree, ok := in.(api.TextTree); ok {
		return f.renderTreeAsHTML(&tree, "Tree")
	}
	if tree, ok := in.(*api.TextTree); ok {
		return f.renderTreeAsHTML(tree, "Tree")
	}

	// Handle TextList with PDF-aware rendering before generic Textable
	if list, ok := in.(api.TextList); ok {
		var result strings.Builder
		if f.IncludeCSS {
			result.WriteString(f.getCSS())
		}
		f.renderTextListAsHTML(&result, list)
		if f.IncludeCSS {
			result.WriteString("    </div>\n</body>\n</html>")
		}
		return result.String(), nil
	}

	// Handle Textable inputs directly
	if textable, ok := in.(api.Textable); ok {
		htmlContent := textable.HTML()
		if f.IncludeCSS {
			var result strings.Builder
			result.WriteString(f.getCSS())
			result.WriteString("        <div class=\"bg-white rounded-lg shadow p-6\">\n")
			result.WriteString("            ")
			if f.EmailMode {
				result.WriteString(f.formatTextableForEmail(textable))
			} else {
				result.WriteString(htmlContent)
			}
			result.WriteString("\n        </div>\n")
			result.WriteString("    </div>\n</body>\n</html>")
			return result.String(), nil
		}
		if f.EmailMode {
			return f.formatTextableForEmail(textable), nil
		}
		return htmlContent, nil
	}

	// Convert to PrettyData
	data, err := f.ToPrettyData(in)
	if err != nil {
		return "", fmt.Errorf("failed to convert to PrettyData: %w", err)
	}

	if data.IsEmpty() {
		return "", nil
	}

	// Delegate to FormatPrettyData for actual rendering
	return f.FormatPrettyData(data)
}

// FormatPrettyData formats PrettyData directly as HTML
func (f *HTMLFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	if data == nil {
		return "", nil
	}

	if data.IsEmpty() {
		return "", nil
	}

	var result strings.Builder

	if f.IncludeCSS {
		result.WriteString(f.getCSS())
	}

	// Get the non-nil TypedValue using Value()
	value := data.Value()

	// Type switch on the actual data type
	switch v := value.(type) {
	case *api.TextTree:
		// Root level tree - render directly
		return f.renderTreeAsHTML(v, "Tree")
	case api.TextTree:
		// Root level tree (value type) - render directly
		tree := v
		return f.renderTreeAsHTML(&tree, "Tree")
	case *api.TextTable:
		// Root level table - render directly
		return f.renderTableAsHTML(v, api.PrettyField{Name: "table"})
	case api.TextTable:
		// Root level table (value type) - render directly
		table := v
		return f.renderTableAsHTML(&table, api.PrettyField{Name: "table"})
	case api.TypedMap:
		// Complex structure with fields - iterate and render each
		f.renderTypedMapAsHTML(&result, &v, data.Schema)
	case *api.TypedMap:
		// Complex structure with fields - iterate and render each (pointer variant)
		f.renderTypedMapAsHTML(&result, v, data.Schema)
	case api.TypedList:
		// TypedList may contain tables - render with PDF awareness
		f.renderTypedListAsHTML(&result, v)
	case api.TextList:
		// TextList may contain tables - render with PDF awareness
		f.renderTextListAsHTML(&result, v)
	case api.Textable:
		// Simple textable content
		if f.IncludeCSS {
			result.WriteString("        <div class=\"bg-white rounded-lg shadow p-6\">\n")
			result.WriteString("            ")
			result.WriteString(v.HTML())
			result.WriteString("\n        </div>\n")
		} else {
			result.WriteString(v.HTML())
		}
	}

	if f.IncludeCSS {
		result.WriteString("    </div>\n</body>\n</html>")
	}

	return result.String(), nil
}

func (f *HTMLFormatter) formatTextableForEmail(textable api.Textable) string {
	switch v := textable.(type) {
	case api.TextList:
		return f.formatTextListForEmail(v)
	case *api.TextList:
		return f.formatTextListForEmail(*v)
	case api.TextTable:
		return v.CompactHTML()
	case *api.TextTable:
		return v.CompactHTML()
	default:
		return textable.HTML()
	}
}

func (f *HTMLFormatter) formatTextListForEmail(list api.TextList) string {
	if len(list) == 0 {
		return ""
	}
	parts := make([]string, 0, len(list))
	for _, item := range list {
		switch v := item.(type) {
		case api.TextTable:
			parts = append(parts, v.CompactHTML())
		case *api.TextTable:
			parts = append(parts, v.CompactHTML())
		default:
			parts = append(parts, item.HTML())
		}
	}
	return strings.Join(parts, api.BR.HTML())
}

// renderTypedListAsHTML renders a TypedList with PDF-aware table rendering
func (f *HTMLFormatter) renderTypedListAsHTML(result *strings.Builder, list api.TypedList) {
	if f.IncludeCSS {
		result.WriteString("        <div class=\"bg-white rounded-lg shadow p-6\">\n")
		result.WriteString("            ")
	}
	for i, item := range list {
		if i > 0 {
			result.WriteString(api.BR.HTML())
		}
		if item.Table != nil {
			if f.IsPDFMode {
				result.WriteString(item.Table.StaticHTML())
			} else {
				result.WriteString(item.Table.HTML())
			}
		} else if item.Tree != nil {
			if f.IsPDFMode {
				result.WriteString(item.Tree.StaticHTML())
			} else {
				result.WriteString(item.Tree.HTML())
			}
		} else {
			result.WriteString(item.HTML())
		}
	}
	if f.IncludeCSS {
		result.WriteString("\n        </div>\n")
	}
}

// renderTextListAsHTML renders a TextList with PDF-aware table rendering
func (f *HTMLFormatter) renderTextListAsHTML(result *strings.Builder, list api.TextList) {
	if f.IncludeCSS {
		result.WriteString("        <div class=\"bg-white rounded-lg shadow p-6\">\n")
		result.WriteString("            ")
	}
	for i, item := range list {
		if i > 0 {
			result.WriteString(api.BR.HTML())
		}
		switch v := item.(type) {
		case api.TextTable:
			if f.IsPDFMode {
				result.WriteString(v.StaticHTML())
			} else {
				result.WriteString(v.HTML())
			}
		case *api.TextTable:
			if f.IsPDFMode {
				result.WriteString(v.StaticHTML())
			} else {
				result.WriteString(v.HTML())
			}
		case api.TextTree:
			if f.IsPDFMode {
				result.WriteString(v.StaticHTML())
			} else {
				result.WriteString(v.HTML())
			}
		case *api.TextTree:
			if f.IsPDFMode {
				result.WriteString(v.StaticHTML())
			} else {
				result.WriteString(v.HTML())
			}
		default:
			result.WriteString(item.HTML())
		}
	}
	if f.IncludeCSS {
		result.WriteString("\n        </div>\n")
	}
}

// renderTypedMapAsHTML renders a TypedMap (complex structure with fields)
func (f *HTMLFormatter) renderTypedMapAsHTML(result *strings.Builder, typedMap *api.TypedMap, schema *api.PrettyObject) {
	// Collect fields by category
	type deferredTable struct {
		table     *api.TextTable
		fieldName string
		field     api.PrettyField
	}
	var summaryFields []api.PrettyField
	var treeSections []api.PrettyField
	var tableSections []api.PrettyField
	var deferredTables []deferredTable

	// Iterate schema to organize fields
	for _, field := range schema.Fields {
		if fieldValue, exists := (*typedMap)[field.Name]; exists {
			// Get the actual non-nil value
			value := fieldValue.Value()

			// Type switch to categorize
			switch v := value.(type) {
			case *api.TextTree:
				treeSections = append(treeSections, field)
			case api.TextTree:
				treeSections = append(treeSections, field)
			case *api.TextTable:
				// Check if this is a deferred table (non-compact nested)
				if fieldValue.FieldMeta != nil && !fieldValue.FieldMeta.CompactItems {
					deferredTables = append(deferredTables, deferredTable{
						table:     v,
						fieldName: f.prettifyFieldName(field.Name),
						field:     field,
					})
				} else {
					tableSections = append(tableSections, field)
				}
			case api.TextTable:
				// Check if this is a deferred table (non-compact nested)
				table := v
				if fieldValue.FieldMeta != nil && !fieldValue.FieldMeta.CompactItems {
					deferredTables = append(deferredTables, deferredTable{
						table:     &table,
						fieldName: f.prettifyFieldName(field.Name),
						field:     field,
					})
				} else {
					tableSections = append(tableSections, field)
				}
			default:
				// Regular field for summary section
				summaryFields = append(summaryFields, field)
			}
		}
	}

	// Render summary section if there are regular fields
	if len(summaryFields) > 0 {
		result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
		result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
		result.WriteString("                <h2 class=\"text-xl font-semibold text-gray-900\">Summary</h2>\n")
		result.WriteString("            </div>\n")
		result.WriteString("            <div class=\"px-6 py-4\">\n")
		result.WriteString("                <dl class=\"grid grid-cols-1 md:grid-cols-2 gap-4\">\n")

		for _, field := range summaryFields {
			fieldValue := (*typedMap)[field.Name]
			prettyFieldName := f.prettifyFieldName(field.Name)
			fieldHTML := f.formatFieldValueHTMLWithStyle(fieldValue, field)

			var labelHTML string
			if field.LabelStyle != "" {
				labelHTML = f.applyTailwindStyleToHTML(prettyFieldName, field.LabelStyle)
			} else {
				labelHTML = fmt.Sprintf("<span class=\"text-sm font-medium text-gray-500\">%s</span>", html.EscapeString(prettyFieldName))
			}

			result.WriteString("                    <div>\n")
			_, _ = fmt.Fprintf(result, "                        <dt>%s</dt>\n", labelHTML)
			_, _ = fmt.Fprintf(result, "                        <dd class=\"mt-1 text-sm\">%s</dd>\n", fieldHTML)
			result.WriteString("                    </div>\n")
		}
		result.WriteString("                </dl>\n")
		result.WriteString("            </div>\n")
		result.WriteString("        </div>\n")
	}

	// Render table sections
	for _, field := range tableSections {
		fieldValue := (*typedMap)[field.Name]
		value := fieldValue.Value()
		switch table := value.(type) {
		case *api.TextTable:
			f.renderTableSectionHTML(result, table, field)
		case api.TextTable:
			f.renderTableSectionHTML(result, &table, field)
		}
	}

	// Render tree sections
	for _, field := range treeSections {
		fieldValue := (*typedMap)[field.Name]
		value := fieldValue.Value()
		switch tree := value.(type) {
		case *api.TextTree:
			f.renderTreeSectionHTML(result, tree, field)
		case api.TextTree:
			f.renderTreeSectionHTML(result, &tree, field)
		}
	}

	// Render deferred tables
	for _, deferred := range deferredTables {
		f.renderTableSectionHTML(result, deferred.table, deferred.field)
	}
}

// renderTableSectionHTML renders a table as a section with title
func (f *HTMLFormatter) renderTableSectionHTML(result *strings.Builder, table *api.TextTable, field api.PrettyField) {
	if table == nil || len(table.Rows) == 0 {
		return
	}

	if len(table.Columns) == 0 {
		table.Columns = field.TableOptions.Columns
	}

	result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
	result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
	_, _ = fmt.Fprintf(result, "                <h2 class=\"text-xl font-semibold text-gray-900\">%s</h2>\n",
		f.prettifyFieldName(field.Name))
	result.WriteString("            </div>\n")

	var tableHTML string
	if f.IsPDFMode {
		tableHTML = table.StaticHTML()
	} else {
		tableHTML = table.HTML()
	}
	result.WriteString(tableHTML)
	result.WriteString("        </div>\n")
}

// renderTreeSectionHTML renders a tree as a section with title
func (f *HTMLFormatter) renderTreeSectionHTML(result *strings.Builder, tree *api.TextTree, field api.PrettyField) {
	if tree == nil {
		return
	}

	result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
	result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
	_, _ = fmt.Fprintf(result, "                <h2 class=\"text-xl font-semibold text-gray-900\">%s</h2>\n",
		f.prettifyFieldName(field.Name))
	result.WriteString("            </div>\n")
	result.WriteString("            <div class=\"px-6 py-4\">\n")

	treeHTML := f.formatTreeFieldHTML(api.TypedValue{Tree: tree}, field)
	result.WriteString(treeHTML)

	result.WriteString("            </div>\n")
	result.WriteString("        </div>\n")
}

// renderTreeAsHTML renders a root-level tree
func (f *HTMLFormatter) renderTreeAsHTML(tree *api.TextTree, title string) (string, error) {
	var result strings.Builder

	if f.IncludeCSS {
		result.WriteString(f.getCSS())
	}

	result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
	result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
	_, _ = fmt.Fprintf(&result, "                <h2 class=\"text-xl font-semibold text-gray-900\">%s</h2>\n", html.EscapeString(title))
	result.WriteString("            </div>\n")
	result.WriteString("            <div class=\"px-6 py-4\">\n")

	var treeHTML string
	if f.IsPDFMode {
		treeHTML = tree.StaticHTML()
	} else {
		treeHTML = tree.HTML()
	}
	result.WriteString(treeHTML)

	result.WriteString("            </div>\n")
	result.WriteString("        </div>\n")

	if f.IncludeCSS {
		result.WriteString("    </div>\n</body>\n</html>")
	}

	return result.String(), nil
}

// renderTableAsHTML renders a root-level table
func (f *HTMLFormatter) renderTableAsHTML(table *api.TextTable, field api.PrettyField) (string, error) {
	var result strings.Builder

	if f.IncludeCSS {
		result.WriteString(f.getCSS())
	}
	if len(table.Columns) == 0 {
		table.Columns = field.TableOptions.Columns
	}

	result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
	result.WriteString("            <div class=\"px-6 py-4 border-b border-gray-200\">\n")
	result.WriteString("                <h2 class=\"text-xl font-semibold text-gray-900\">Table</h2>\n")
	result.WriteString("            </div>\n")

	var tableHTML string
	if f.IsPDFMode {
		tableHTML = table.StaticHTML()
	} else {
		tableHTML = table.HTML()
	}
	result.WriteString(tableHTML)
	result.WriteString("        </div>\n")

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

// prettifyFieldName converts field names to readable format
func (f *HTMLFormatter) prettifyFieldName(name string) string {
	return PrettifyFieldName(name)
}

// formatFieldValueHTMLWithStyle formats a TypedValue with field styling for HTML output
func (f *HTMLFormatter) formatFieldValueHTMLWithStyle(fieldValue api.TypedValue, field api.PrettyField) string {
	valueStr := fieldValue.String()
	if field.Format == "image" || f.isImageURL(valueStr) {
		return f.formatImageHTML(fieldValue, field)
	}

	// Handle tables - use static HTML in PDF mode
	if fieldValue.Table != nil {
		if len(fieldValue.Table.Columns) == 0 {
			fieldValue.Table.Columns = field.TableOptions.Columns
		}
		if fieldValue.FieldMeta != nil && fieldValue.FieldMeta.CompactItems {
			return fieldValue.Table.CompactHTML()
		}
		if fieldValue.FieldMeta != nil && !fieldValue.FieldMeta.CompactItems {
			return fmt.Sprintf(`<span class="text-gray-500 italic">See %s table below</span>`, html.EscapeString(fieldValue.FieldMeta.Name))
		}
		if f.IsPDFMode {
			return fieldValue.Table.StaticHTML()
		}
		return fieldValue.Table.HTML()
	}

	// Handle trees - use static HTML in PDF mode
	if fieldValue.Tree != nil {
		if f.IsPDFMode {
			return fieldValue.Tree.StaticHTML()
		}
		return fieldValue.Tree.HTML()
	}

	return fieldValue.HTML()
}

// formatTreeFieldHTML formats a tree field for HTML output
func (f *HTMLFormatter) formatTreeFieldHTML(fieldValue api.TypedValue, _ api.PrettyField) string {
	if fieldValue.Tree == nil {
		return "<p class=\"text-gray-500\">No tree data available</p>"
	}

	if f.IsPDFMode {
		return fieldValue.Tree.StaticHTML()
	}
	return fieldValue.Tree.HTML()
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

	result.WriteString("        <div class=\"bg-white rounded-lg shadow\">\n")
	result.WriteString("            <div class=\"px-6 py-4\">\n")

	textTree := api.NewTree(root)
	var treeHTML string
	if f.IsPDFMode {
		treeHTML = textTree.StaticHTML()
	} else {
		treeHTML = textTree.HTML()
	}
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
