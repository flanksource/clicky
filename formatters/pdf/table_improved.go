package pdf

import (
	"fmt"

	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"github.com/flanksource/clicky/api"
)

// TableImproved widget for rendering tables in PDF with dynamic columns
type TableImproved struct {
	Headers           []string    `json:"headers,omitempty"`
	Rows              [][]any     `json:"rows,omitempty"`
	HeaderStyle       api.Class   `json:"header_style,omitempty"`
	RowStyle          api.Class   `json:"row_style,omitempty"`
	CellPadding       api.Padding `json:"cell_padding,omitempty"`
	AlternateRowColor bool        `json:"alternate_row_color,omitempty"`
	ShowBorders       bool        `json:"show_borders,omitempty"`
	ColumnAlignments  []string    `json:"column_alignments,omitempty"` // left, center, right for each column
	ColumnWidths      []int       `json:"column_widths,omitempty"`     // Custom column widths (sum should be 12)
}

// Draw implements the Widget interface
func (ti TableImproved) Draw(b *Builder) error {
	if len(ti.Headers) == 0 && len(ti.Rows) == 0 {
		return nil // Nothing to draw
	}

	// Determine number of columns
	numColumns := len(ti.Headers)
	if numColumns == 0 && len(ti.Rows) > 0 {
		numColumns = len(ti.Rows[0])
	}

	if numColumns == 0 || numColumns > 12 {
		return fmt.Errorf("invalid number of columns: %d (must be 1-12)", numColumns)
	}

	// Calculate column widths
	colWidths := ti.calculateColumnWidths(numColumns)

	// Calculate row height
	baseHeight := 8.0 // Default row height in mm
	if ti.CellPadding.Top > 0 || ti.CellPadding.Bottom > 0 {
		baseHeight += (ti.CellPadding.Top + ti.CellPadding.Bottom) * 4
	}

	// Draw top border if enabled
	if ti.ShowBorders {
		ti.drawHorizontalLine(b, 0.5, 200, sumArray(colWidths))
	}

	// Draw headers if present
	if len(ti.Headers) > 0 {
		ti.drawHeaderRow(b, colWidths, baseHeight)

		// Draw separator line after headers
		if ti.ShowBorders {
			ti.drawHorizontalLine(b, 0.5, 200, sumArray(colWidths))
		} else {
			ti.drawHorizontalLine(b, 0.3, 230, sumArray(colWidths))
		}
	}

	// Draw data rows
	ti.drawDataRows(b, colWidths, baseHeight)

	// Draw bottom border if enabled
	if ti.ShowBorders || len(ti.Rows) > 0 {
		ti.drawHorizontalLine(b, 0.5, 200, sumArray(colWidths))
	}

	// Add spacing after table
	b.maroto.AddRows(row.New(2))

	return nil
}

// calculateColumnWidths calculates the width for each column
func (ti TableImproved) calculateColumnWidths(numColumns int) []int {
	// Use custom widths if provided
	if len(ti.ColumnWidths) == numColumns {
		sum := 0
		for _, w := range ti.ColumnWidths {
			sum += w
		}
		if sum == 12 {
			return ti.ColumnWidths
		}
	}

	// Auto-calculate widths
	widths := make([]int, numColumns)
	baseWidth := 12 / numColumns
	remainder := 12 % numColumns

	for i := 0; i < numColumns; i++ {
		widths[i] = baseWidth
		if i < remainder {
			widths[i]++
		}
	}

	return widths
}

// drawHeaderRow draws the header row
func (ti TableImproved) drawHeaderRow(b *Builder, colWidths []int, baseHeight float64) {
	// Apply default header styling if not specified
	headerStyle := ti.HeaderStyle
	if headerStyle.Name == "" {
		// Use Tailwind classes for default styling
		headerStyle = api.ResolveStyles("font-bold bg-gray-100")
	} else {
		// Resolve Tailwind classes
		headerStyle = api.ResolveStyles(headerStyle.Name)
	}

	// Ensure bold font for headers
	if headerStyle.Font == nil {
		headerStyle.Font = &api.Font{Bold: true}
	}

	// Set default background if not specified
	if headerStyle.Background == nil {
		headerStyle.Background = &api.Color{Hex: "#f3f4f6"} // Gray background
	}

	// Convert header style
	headerTextProps := b.style.ConvertToTextProps(headerStyle)

	// Add padding
	headerTextProps.Left = ti.CellPadding.Left * 4
	headerTextProps.Top = ti.CellPadding.Top * 4

	// Create columns for headers
	cols := make([]core.Col, 0, len(ti.Headers))
	totalColWidth := 0
	for i, header := range ti.Headers {
		if i >= len(colWidths) {
			break
		}

		// Apply alignment
		textProps := *headerTextProps
		if i < len(ti.ColumnAlignments) {
			textProps.Align = ti.parseAlignment(ti.ColumnAlignments[i])
		}

		// Create column with background color
		headerCol := col.New(colWidths[i]).Add(
			text.New(header, textProps),
		)

		// Add background if specified
		if headerStyle.Background != nil {
			bgColor := b.style.ConvertBackgroundColor(*headerStyle.Background)
			headerCol = headerCol.WithStyle(&props.Cell{
				BackgroundColor: bgColor,
			})
		}

		cols = append(cols, headerCol)
		totalColWidth += colWidths[i]
	}

	// Add empty column to fill remaining space if needed
	if totalColWidth < 12 {
		cols = append(cols, col.New(12-totalColWidth))
	}

	// Add the header row
	b.maroto.AddRow(baseHeight, cols...)
}

// drawDataRows draws all data rows
func (ti TableImproved) drawDataRows(b *Builder, colWidths []int, baseHeight float64) {
	// Apply row styling
	rowStyle := ti.RowStyle
	if rowStyle.Name != "" {
		rowStyle = api.ResolveStyles(rowStyle.Name)
	}

	// Default row style
	if rowStyle.Font == nil {
		rowStyle.Font = &api.Font{Size: 0.9}
	}

	rowTextProps := b.style.ConvertToTextProps(rowStyle)
	rowTextProps.Left = ti.CellPadding.Left * 4
	rowTextProps.Top = ti.CellPadding.Top * 4

	// Alternate row background colors
	altBgColor := &props.Color{Red: 248, Green: 248, Blue: 248} // Very light gray

	for rowIndex, dataRow := range ti.Rows {
		cols := make([]core.Col, 0, len(dataRow))
		totalColWidth := 0

		for colIndex := 0; colIndex < len(colWidths) && colIndex < len(dataRow); colIndex++ {
			cellText := fmt.Sprintf("%v", dataRow[colIndex])

			// Apply alignment
			textProps := *rowTextProps
			if colIndex < len(ti.ColumnAlignments) {
				textProps.Align = ti.parseAlignment(ti.ColumnAlignments[colIndex])
			}

			cellCol := col.New(colWidths[colIndex]).Add(
				text.New(cellText, textProps),
			)

			// Add alternating background if enabled
			if ti.AlternateRowColor && rowIndex%2 == 1 {
				cellCol = cellCol.WithStyle(&props.Cell{
					BackgroundColor: altBgColor,
				})
			}

			cols = append(cols, cellCol)
			totalColWidth += colWidths[colIndex]
		}

		// Add empty column to fill remaining space if needed
		if totalColWidth < 12 {
			cols = append(cols, col.New(12-totalColWidth))
		}

		// Add the data row
		b.maroto.AddRow(baseHeight, cols...)

		// Add row separator if borders are enabled
		if ti.ShowBorders && rowIndex < len(ti.Rows)-1 {
			ti.drawHorizontalLine(b, 0.2, 240, sumArray(colWidths))
		}
	}
}

// drawHorizontalLine draws a horizontal line
func (ti TableImproved) drawHorizontalLine(b *Builder, thickness float64, grayLevel, totalColumns int) {
	b.maroto.AddRow(0.5, col.New(totalColumns).Add(line.New(props.Line{
		Color:     &props.Color{Red: grayLevel, Green: grayLevel, Blue: grayLevel},
		Thickness: thickness,
	})))
}

// parseAlignment converts string alignment to Maroto alignment
func (ti TableImproved) parseAlignment(alignStr string) align.Type {
	switch alignStr {
	case "center":
		return align.Center
	case "right":
		return align.Right
	case "justify":
		return align.Justify
	default:
		return align.Left
	}
}

// sumArray sums all values in an integer array
func sumArray(arr []int) int {
	sum := 0
	for _, v := range arr {
		sum += v
	}
	return sum
}
