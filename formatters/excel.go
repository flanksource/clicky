package formatters

import (
	"fmt"
	"strconv"

	"github.com/flanksource/clicky/api"
	"github.com/xuri/excelize/v2"
)

// ExcelFormatter handles Excel file generation
type ExcelFormatter struct {
	SheetName string
}

// NewExcelFormatter creates a new Excel formatter
func NewExcelFormatter() *ExcelFormatter {
	return &ExcelFormatter{
		SheetName: "Sheet1",
	}
}

// Format is not supported for Excel as it needs to write to files
func (f *ExcelFormatter) Format(data interface{}) (string, error) {
	return "", fmt.Errorf("Excel formatter requires file output - use FormatToFile instead")
}

// FormatToFile creates an Excel file and saves it to the specified path
func (f *ExcelFormatter) FormatToFile(data interface{}, filename string) error {
	// Convert data to PrettyData for consistent handling
	prettyData, err := ToPrettyData(data)
	if err != nil {
		return fmt.Errorf("failed to convert data to PrettyData: %w", err)
	}

	file := excelize.NewFile()
	defer func() {
		_ = file.Close() // ignore error on close
	}()

	return f.FormatPrettyDataToFile(prettyData, filename, file)
}

// FormatPrettyDataToFile formats PrettyData to Excel file
func (f *ExcelFormatter) FormatPrettyDataToFile(data *api.PrettyData, filename string, file *excelize.File) error {
	if data == nil || data.Schema == nil {
		return fmt.Errorf("no data to format")
	}

	sheetName := f.SheetName
	if sheetName == "" {
		sheetName = "Sheet1"
	}

	// Rename default sheet
	if sheetIndex, err := file.GetSheetIndex("Sheet1"); err == nil && sheetIndex >= 0 {
		if err := file.SetSheetName("Sheet1", sheetName); err != nil {
			return fmt.Errorf("failed to rename sheet: %w", err)
		}
	} else {
		_, err := file.NewSheet(sheetName)
		if err != nil {
			return fmt.Errorf("failed to create sheet: %w", err)
		}
	}

	currentRow := 1
	table := data.FirstTable()
	if table == nil {
		return fmt.Errorf("no tables defined in PrettyData")
	}

	// Get headers and field names from TableOptions
	var headers = table.Headers.AsString()
	for i, header := range headers {
		cellRef := f.getCellReference(i+1, currentRow)
		if err := file.SetCellValue(sheetName, cellRef, header); err != nil {
			return fmt.Errorf("failed to set header value: %w", err)
		}
	}

	// Apply header styling
	headerStyle, err := f.createHeaderStyle(file)
	if err != nil {
		return fmt.Errorf("failed to create header style: %w", err)
	}

	if len(headers) > 0 {
		startCell := f.getCellReference(1, currentRow)
		endCell := f.getCellReference(len(headers), currentRow)
		if err := file.SetCellStyle(sheetName, startCell, endCell, headerStyle); err != nil {
			return fmt.Errorf("failed to set header style: %w", err)
		}
	}
	currentRow++

	// Write data rows using Text.String() for formatted text
	for _, row := range table.Rows {
		for i, fieldName := range headers {
			cellRef := f.getCellReference(i+1, currentRow)
			if fieldValue, exists := row[fieldName]; exists {
				if err := file.SetCellValue(sheetName, cellRef, fieldValue.String()); err != nil {
					return fmt.Errorf("failed to set cell value: %w", err)
				}
			}
		}
		currentRow++
	}

	// Auto-fit columns for the table
	if len(headers) > 0 {
		startCol := f.getColumnName(1)
		endCol := f.getColumnName(len(headers))
		if err := file.SetColWidth(sheetName, startCol, endCol, 15); err != nil {
			return fmt.Errorf("failed to set column width: %w", err)
		}
	}

	// Save the file
	if err := file.SaveAs(filename); err != nil {
		return fmt.Errorf("failed to save Excel file: %w", err)
	}

	return nil
}

// FormatPrettyData formats PrettyData to Excel bytes (for in-memory operations)
func (f *ExcelFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	file := excelize.NewFile()
	defer func() {
		_ = file.Close() // ignore error on close
	}()

	if err := f.FormatPrettyDataToFile(data, "", file); err != nil {
		return "", err
	}

	// Return buffer as string (not ideal for large files)
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return "", fmt.Errorf("failed to write to buffer: %w", err)
	}

	return buffer.String(), nil
}

// createHeaderStyle creates a header cell style
func (f *ExcelFormatter) createHeaderStyle(file *excelize.File) (int, error) {
	return file.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 12,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E8F4FD"},
			Pattern: 1,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
}

// getCellReference returns Excel cell reference (e.g., A1, B2)
func (f *ExcelFormatter) getCellReference(col, row int) string {
	return f.getColumnName(col) + strconv.Itoa(row)
}

// getColumnName converts column number to Excel column name (1=A, 2=B, 27=AA)
func (f *ExcelFormatter) getColumnName(col int) string {
	name := ""
	for col > 0 {
		col-- // Convert to 0-based
		name = string(rune('A'+col%26)) + name
		col /= 26
	}
	return name
}

// FormatValue is not needed as we use fieldValue.Plain() for formatted text
// This is kept for interface compatibility if needed
func (f *ExcelFormatter) FormatValue(value interface{}) (string, error) {
	return fmt.Sprintf("%v", value), nil
}
