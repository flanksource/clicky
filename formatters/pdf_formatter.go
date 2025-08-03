package formatters

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/jung-kurt/gofpdf/v2"
)

// PDFFormatter handles PDF formatting
type PDFFormatter struct {
	pdf *gofpdf.Fpdf
}

// NewPDFFormatter creates a new PDF formatter
func NewPDFFormatter() *PDFFormatter {
	return &PDFFormatter{}
}

// Format formats PrettyData as PDF
func (f *PDFFormatter) Format(data *api.PrettyData) (string, error) {
	// Create a new PDF document
	f.pdf = gofpdf.New("P", "mm", "A4", "")
	f.pdf.SetAutoPageBreak(true, 15)
	f.pdf.AddPage()
	
	// Set document info
	f.pdf.SetTitle("Data Report", true)
	f.pdf.SetAuthor("Clicky", true)
	f.pdf.SetCreator("Clicky", true)
	
	// Add title
	f.pdf.SetFont("Arial", "B", 20)
	f.pdf.Cell(0, 12, "Data Report")
	f.pdf.Ln(15)
	
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

// formatSummarySection formats the summary fields
func (f *PDFFormatter) formatSummarySection(data *api.PrettyData) {
	// Add section title
	f.pdf.SetFont("Arial", "B", 16)
	f.pdf.SetTextColor(0, 0, 0)
	f.pdf.Cell(0, 10, "Summary")
	f.pdf.Ln(12)
	
	// Format each field in a two-column layout
	f.pdf.SetFont("Arial", "", 11)
	
	for _, field := range data.Schema.Fields {
		if field.Format == api.FormatTable {
			continue // Skip table fields in summary
		}
		
		if fieldValue, exists := data.Values[field.Name]; exists {
			// Field name (bold)
			f.pdf.SetFont("Arial", "B", 11)
			f.pdf.SetTextColor(50, 50, 50)
			prettyName := f.prettifyFieldName(field.Name)
			f.pdf.Cell(60, 8, prettyName+":")
			
			// Field value (normal)
			f.pdf.SetFont("Arial", "", 11)
			f.pdf.SetTextColor(0, 0, 0)
			
			formatted := fieldValue.Formatted()
			
			// Handle multi-line values
			if strings.Contains(formatted, "\n") {
				// Save current position
				x := f.pdf.GetX()
				y := f.pdf.GetY()
				
				// Use MultiCell for multi-line content
				f.pdf.SetXY(x, y)
				f.pdf.MultiCell(0, 6, formatted, "", "L", false)
			} else {
				// For single line, use Cell
				f.pdf.Cell(0, 8, formatted)
				f.pdf.Ln(8)
			}
		}
	}
}

// formatTableSection formats a table
func (f *PDFFormatter) formatTableSection(tableName string, rows []api.PrettyDataRow, schema *api.PrettyObject) {
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
	f.pdf.SetFont("Arial", "B", 14)
	f.pdf.SetTextColor(0, 0, 0)
	prettyName := f.prettifyFieldName(tableName)
	f.pdf.Cell(0, 10, prettyName)
	f.pdf.Ln(10)
	
	// Calculate column widths
	numCols := len(tableField.TableOptions.Fields)
	if numCols == 0 {
		return
	}
	
	pageWidth := 190.0 // A4 width minus margins
	colWidth := pageWidth / float64(numCols)
	
	// Draw header row
	f.pdf.SetFont("Arial", "B", 10)
	f.pdf.SetFillColor(230, 230, 230)
	f.pdf.SetTextColor(0, 0, 0)
	
	for _, field := range tableField.TableOptions.Fields {
		f.pdf.CellFormat(colWidth, 8, field.Name, "1", 0, "C", true, 0, "")
	}
	f.pdf.Ln(-1)
	
	// Draw data rows
	f.pdf.SetFont("Arial", "", 9)
	f.pdf.SetFillColor(255, 255, 255)
	
	for i, row := range rows {
		// Alternate row colors for better readability
		if i%2 == 1 {
			f.pdf.SetFillColor(245, 245, 245)
		} else {
			f.pdf.SetFillColor(255, 255, 255)
		}
		
		for _, field := range tableField.TableOptions.Fields {
			cellValue := ""
			if fieldValue, exists := row[field.Name]; exists {
				cellValue = fieldValue.Formatted()
				// Clean up for table display
				cellValue = strings.ReplaceAll(cellValue, "\n", " ")
				cellValue = f.truncateText(cellValue, colWidth)
			}
			
			f.pdf.CellFormat(colWidth, 7, cellValue, "1", 0, "L", true, 0, "")
		}
		f.pdf.Ln(-1)
	}
}

// truncateText truncates text to fit in cell width
func (f *PDFFormatter) truncateText(text string, maxWidth float64) string {
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
func (f *PDFFormatter) prettifyFieldName(name string) string {
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
func (f *PDFFormatter) splitCamelCase(s string) []string {
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