package formatters

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
	"github.com/jung-kurt/gofpdf/v2"
)

// PDFLegacyFormatter handles PDF formatting using the legacy gofpdf approach
type PDFLegacyFormatter struct {
	pdf *gofpdf.Fpdf
}

// NewPDFLegacyFormatter creates a new legacy PDF formatter
func NewPDFLegacyFormatter() *PDFLegacyFormatter {
	return &PDFLegacyFormatter{}
}

// Format formats PrettyData as PDF
func (f *PDFLegacyFormatter) Format(data *api.PrettyData) (string, error) {
	// Create a new PDF document
	f.pdf = gofpdf.New("P", "mm", "A4", "")
	f.pdf.SetAutoPageBreak(true, 15)
	f.pdf.AddPage()

	// Set document info
	f.pdf.SetTitle("Data Report", true)
	f.pdf.SetAuthor("Clicky", true)
	f.pdf.SetCreator("Clicky", true)

	// Add title
	f.pdf.SetFont("Arial", "B", 18)

	// Format summary fields
	if len(data.Values) > 0 {
		f.formatSummarySection(data)
	}

	// Format tables
	for tableName, tableRows := range data.Tables {
		if len(tableRows) > 0 {
			// Add some space before table
			f.pdf.Ln(10)
			f.formatTableSection(tableName, tableRows, data.Schema)
		}
	}

	// Write to buffer
	var buf bytes.Buffer
	err := f.pdf.Output(&buf)
	if err != nil {
		return "", fmt.Errorf("failed to generate PDF: %w", err)
	}

	return buf.String(), nil
}

// applyTailwindStyleToPDF applies Tailwind styles to PDF text formatting
func (f *PDFLegacyFormatter) applyTailwindStyleToPDF(text string, styleStr string) string {
	if styleStr == "" {
		return text
	}

	// Parse the Tailwind style
	transformedText, style := tailwind.ApplyStyle(text, styleStr)
	
	// Apply text transformations (already done by ApplyStyle)
	text = transformedText
	
	// Convert Tailwind style to PDF styling
	f.applyPDFStyling(style)
	
	return text
}

// applyPDFStyling applies a tailwind.Style to the PDF
func (f *PDFLegacyFormatter) applyPDFStyling(style tailwind.Style) {
	// Default font style
	fontStyle := ""
	if style.Bold {
		fontStyle = "B"
	}
	if style.Italic {
		if fontStyle == "B" {
			fontStyle = "BI" // Bold + Italic
		} else {
			fontStyle = "I"
		}
	}
	
	// Set font (keep current size)
	_, currentSize := f.pdf.GetFontSize()
	f.pdf.SetFont("Arial", fontStyle, currentSize)
	
	// Apply text color if specified
	if style.Foreground != "" {
		r, g, b := f.hexToRGB(style.Foreground)
		f.pdf.SetTextColor(r, g, b)
	}
	
	// Note: PDF doesn't support underline/strikethrough easily with gofpdf
	// Background colors would require drawing rectangles behind text
}

// hexToRGB converts hex color to RGB values
func (f *PDFLegacyFormatter) hexToRGB(hex string) (int, int, int) {
	// Remove # if present
	hex = strings.TrimPrefix(hex, "#")
	
	// Default to black if invalid
	if len(hex) != 6 {
		return 0, 0, 0
	}
	
	// Parse RGB components
	r, err1 := strconv.ParseInt(hex[0:2], 16, 64)
	g, err2 := strconv.ParseInt(hex[2:4], 16, 64)
	b, err3 := strconv.ParseInt(hex[4:6], 16, 64)
	
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0 // Default to black
	}
	
	return int(r), int(g), int(b)
}

// formatSummarySection formats the summary fields with special handling for maps
func (f *PDFLegacyFormatter) formatSummarySection(data *api.PrettyData) {
	// Add section title
	f.pdf.SetFont("Arial", "B", 14)
	f.pdf.SetTextColor(0, 0, 0)
	f.pdf.Cell(0, 8, "Summary")
	f.pdf.Ln(10)

	// Process each field
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatTable {
			continue // Skip table fields in summary
		}

		fieldValue, exists := data.Values[field.Name]
		if !exists {
			continue
		}

		// Check if this is a map field
		if fieldValue.MapValue != nil && len(fieldValue.MapValue) > 0 {
			// Render map with each key-value on a new line
			f.formatMapField(field.Name, fieldValue, field)
		} else {
			// Regular field - render inline
			f.formatRegularField(field.Name, fieldValue, field)
		}
	}
}

// formatMapField formats a map field with indented key-value pairs
func (f *PDFLegacyFormatter) formatMapField(fieldName string, fieldValue api.FieldValue, field api.PrettyField) {
	prettyName := f.prettifyFieldName(fieldName)
	
	// Apply label styling if specified
	if field.LabelStyle != "" {
		prettyName = f.applyTailwindStyleToPDF(prettyName, field.LabelStyle)
	} else {
		// Default label style
		f.pdf.SetFont("Arial", "B", 9)
		f.pdf.SetTextColor(50, 50, 50)
	}
	
	f.pdf.Cell(0, 6, prettyName+":")
	f.pdf.Ln(6)

	// Format the map content with indentation
	formatted := fieldValue.Formatted()
	
	// Apply field styling if specified
	if field.Style != "" {
		formatted = f.applyTailwindStyleToPDF(formatted, field.Style)
	} else {
		// Default content style
		f.pdf.SetFont("Arial", "", 8)
		f.pdf.SetTextColor(0, 0, 0)
	}
	
	lines := strings.Split(formatted, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			// Add indentation for map content
			f.pdf.Cell(10, 5, "") // Indent
			f.pdf.Cell(0, 5, line)
			f.pdf.Ln(5)
		}
	}
	
	// Add a bit of space after the map
	f.pdf.Ln(2)
}

// formatRegularField formats a regular non-map field
func (f *PDFLegacyFormatter) formatRegularField(fieldName string, fieldValue api.FieldValue, field api.PrettyField) {
	prettyName := f.prettifyFieldName(fieldName)
	
	// Apply label styling if specified
	if field.LabelStyle != "" {
		prettyName = f.applyTailwindStyleToPDF(prettyName, field.LabelStyle)
	} else {
		// Default label style
		f.pdf.SetFont("Arial", "B", 9)
		f.pdf.SetTextColor(50, 50, 50)
	}
	
	labelWidth := 50.0
	f.pdf.Cell(labelWidth, 6, prettyName+":")

	// Format field value
	formatted := fieldValue.Formatted()
	
	// Apply field styling if specified
	if field.Style != "" {
		formatted = f.applyTailwindStyleToPDF(formatted, field.Style)
	} else {
		// Default value style
		f.pdf.SetFont("Arial", "", 9)
		f.pdf.SetTextColor(0, 0, 0)
	}
	
	// Handle multi-line values
	if strings.Contains(formatted, "\n") {
		// For multi-line, just use first line with ellipsis
		lines := strings.Split(formatted, "\n")
		f.pdf.Cell(0, 6, lines[0]+"...")
	} else {
		f.pdf.Cell(0, 6, formatted)
	}
	f.pdf.Ln(6)
}

// formatTableSection formats a table
func (f *PDFLegacyFormatter) formatTableSection(tableName string, rows []api.PrettyDataRow, schema *api.PrettyObject) {
	// Find the table field in schema
	var tableField *api.PrettyField
	for _, field := range schema.Fields {
		if field.Name == tableName && field.Format == api.FormatTable {
			tableField = &field
			break
		}
	}

	if tableField == nil {
		return
	}

	// Add table title
	f.pdf.SetFont("Arial", "B", 12)
	f.pdf.SetTextColor(0, 0, 0)
	prettyName := f.prettifyFieldName(tableName)
	f.pdf.Cell(0, 8, prettyName)
	f.pdf.Ln(8)

	// Calculate column widths based on content
	numCols := len(tableField.TableOptions.Fields)
	if numCols == 0 {
		return
	}

	colWidths := f.calculateColumnWidths(tableField.TableOptions.Fields, rows)

	// Draw header row
	if tableField.TableOptions.HeaderStyle != "" {
		// Apply custom header styling
		_ = f.applyTailwindStyleToPDF("", tableField.TableOptions.HeaderStyle)
	} else {
		// Default header style
		f.pdf.SetFont("Arial", "B", 8)
		f.pdf.SetTextColor(0, 0, 0)
	}
	f.pdf.SetFillColor(230, 230, 230)

	for i, field := range tableField.TableOptions.Fields {
		headerText := field.Name
		if tableField.TableOptions.HeaderStyle != "" {
			headerText = f.applyTailwindStyleToPDF(headerText, tableField.TableOptions.HeaderStyle)
		}
		f.pdf.CellFormat(colWidths[i], 6, headerText, "1", 0, "C", true, 0, "")
	}
	f.pdf.Ln(-1)

	// Draw data rows
	f.pdf.SetFont("Arial", "", 7)
	f.pdf.SetFillColor(255, 255, 255)

	for rowIdx, row := range rows {
		// Alternate row colors for better readability
		if rowIdx%2 == 1 {
			f.pdf.SetFillColor(245, 245, 245)
		} else {
			f.pdf.SetFillColor(255, 255, 255)
		}

		for colIdx, field := range tableField.TableOptions.Fields {
			cellValue := ""
			if fieldValue, exists := row[field.Name]; exists {
				cellValue = fieldValue.Formatted()
				// Clean up for table display
				cellValue = strings.ReplaceAll(cellValue, "\n", " ")
				cellValue = f.truncateText(cellValue, colWidths[colIdx])
				
				// Apply styling with priority: field.Style > row_style
				if field.Style != "" {
					cellValue = f.applyTailwindStyleToPDF(cellValue, field.Style)
				} else if tableField.TableOptions.RowStyle != "" {
					cellValue = f.applyTailwindStyleToPDF(cellValue, tableField.TableOptions.RowStyle)
				} else {
					// Default row style
					f.pdf.SetFont("Arial", "", 7)
					f.pdf.SetTextColor(0, 0, 0)
				}
			}

			f.pdf.CellFormat(colWidths[colIdx], 5, cellValue, "1", 0, "L", true, 0, "")
		}
		f.pdf.Ln(-1)
	}
}

// calculateColumnWidths calculates optimal column widths based on content
func (f *PDFLegacyFormatter) calculateColumnWidths(fields []api.PrettyField, rows []api.PrettyDataRow) []float64 {
	pageWidth := 190.0 // A4 width minus margins
	numCols := len(fields)
	colWidths := make([]float64, numCols)
	maxWidths := make([]float64, numCols)

	// Save current font settings
	origSize, _ := f.pdf.GetFontSize()
	origFamily := "Arial"
	origStyle := ""

	// Calculate max width needed for each column
	for i, field := range fields {
		// Check header width
		f.pdf.SetFont("Arial", "B", 8)
		headerWidth := f.pdf.GetStringWidth(field.Name) + 4 // Add padding
		maxWidths[i] = headerWidth

		// Check all cell widths
		f.pdf.SetFont("Arial", "", 7)
		for _, row := range rows {
			if fieldValue, exists := row[field.Name]; exists {
				cellValue := fieldValue.Formatted()
				cellValue = strings.ReplaceAll(cellValue, "\n", " ")
				
				// Limit max width per column to prevent one column from taking too much space
				cellWidth := f.pdf.GetStringWidth(cellValue) + 4
				if cellWidth > maxWidths[i] {
					maxWidths[i] = cellWidth
				}
				
				// Cap maximum width at 60mm for any single column
				if maxWidths[i] > 60 {
					maxWidths[i] = 60
				}
			}
		}
	}

	// Restore original font
	f.pdf.SetFont(origFamily, origStyle, origSize)

	// Calculate total width needed
	totalNeeded := 0.0
	for _, width := range maxWidths {
		totalNeeded += width
	}

	// If total needed width fits on page, use it
	if totalNeeded <= pageWidth {
		return maxWidths
	}

	// Otherwise, scale proportionally
	scale := pageWidth / totalNeeded
	for i := range colWidths {
		colWidths[i] = maxWidths[i] * scale
		// Ensure minimum width
		if colWidths[i] < 15 {
			colWidths[i] = 15
		}
	}

	// Adjust to ensure total equals page width
	totalWidth := 0.0
	for _, width := range colWidths {
		totalWidth += width
	}
	if totalWidth != pageWidth {
		// Adjust last column
		colWidths[len(colWidths)-1] += pageWidth - totalWidth
	}

	return colWidths
}

// truncateText truncates text to fit in cell width
func (f *PDFLegacyFormatter) truncateText(text string, maxWidth float64) string {
	// Get string width
	strWidth := f.pdf.GetStringWidth(text)

	if strWidth <= maxWidth-2 { // Leave some padding
		return text
	}

	// Truncate with ellipsis
	for len(text) > 0 {
		truncated := text + "..."
		if f.pdf.GetStringWidth(truncated) <= maxWidth-2 {
			return truncated
		}
		// Remove one character at a time from the end
		runes := []rune(text)
		if len(runes) > 0 {
			text = string(runes[:len(runes)-1])
		} else {
			break
		}
	}

	return "..."
}

// prettifyFieldName converts field names to readable format
func (f *PDFLegacyFormatter) prettifyFieldName(name string) string {
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
func (f *PDFLegacyFormatter) splitCamelCase(s string) []string {
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
