package pdf

import (
	"github.com/flanksource/maroto/v2/pkg/components/text"
	"github.com/flanksource/maroto/v2/pkg/consts/align"
	"github.com/flanksource/maroto/v2/pkg/consts/fontstyle"
	"github.com/flanksource/maroto/v2/pkg/core"
	"github.com/flanksource/maroto/v2/pkg/core/entity"
	"github.com/flanksource/maroto/v2/pkg/fpdf"
	"github.com/flanksource/maroto/v2/pkg/props"
	"github.com/johnfercher/go-tree/node"
)

// TableComponent implements core.Component interface for embedding tables in columns
type TableComponent struct {
	Headers       []string
	Rows          [][]string
	HeaderStyle   props.Text
	RowStyle      props.Text
	ShowBorders   bool
	HeaderBgColor *props.Color
	RowBgColor    *props.Color
	AltRowBgColor *props.Color
	BorderColor   *props.Color
	TopAlign      bool // Force top alignment within cell
	config        *entity.Config
}

// NewTableComponent creates a new table component with default styling
func NewTableComponent(headers []string, rows [][]string) *TableComponent {
	return &TableComponent{
		Headers:     headers,
		Rows:        rows,
		ShowBorders: true,
		TopAlign:    true, // Default to top alignment
		HeaderStyle: props.Text{
			Size:  8,
			Style: fontstyle.Bold,
			Align: align.Center,
			Color: &props.Color{Red: 0, Green: 0, Blue: 0}, // Explicit black text
		},
		RowStyle: props.Text{
			Size:  7,
			Style: fontstyle.Normal,
			Align: align.Left,
			Color: &props.Color{Red: 0, Green: 0, Blue: 0}, // Explicit black text
		},
		HeaderBgColor: &props.Color{Red: 70, Green: 130, Blue: 180},  // Steel blue
		RowBgColor:    &props.Color{Red: 255, Green: 255, Blue: 255}, // White
		AltRowBgColor: &props.Color{Red: 248, Green: 248, Blue: 248}, // Light gray
		BorderColor:   &props.Color{Red: 128, Green: 128, Blue: 128}, // Gray
	}
}

// WithHeaderStyle sets the header text styling
func (tc *TableComponent) WithHeaderStyle(style props.Text) *TableComponent {
	// Preserve black text color if not explicitly set
	if style.Color == nil {
		style.Color = &props.Color{Red: 0, Green: 0, Blue: 0}
	}
	tc.HeaderStyle = style
	return tc
}

// WithRowStyle sets the row text styling
func (tc *TableComponent) WithRowStyle(style props.Text) *TableComponent {
	// Preserve black text color if not explicitly set
	if style.Color == nil {
		style.Color = &props.Color{Red: 0, Green: 0, Blue: 0}
	}
	tc.RowStyle = style
	return tc
}

// WithColors sets the color scheme
func (tc *TableComponent) WithColors(headerBg, rowBg, altRowBg, border *props.Color) *TableComponent {
	tc.HeaderBgColor = headerBg
	tc.RowBgColor = rowBg
	tc.AltRowBgColor = altRowBg
	tc.BorderColor = border
	return tc
}

// WithBorders enables or disables border drawing
func (tc *TableComponent) WithBorders(enabled bool) *TableComponent {
	tc.ShowBorders = enabled
	return tc
}

// WithTopAlign configures whether the table should align to the top of its cell
func (tc *TableComponent) WithTopAlign(topAlign bool) *TableComponent {
	tc.TopAlign = topAlign
	return tc
}

// Render implements core.Component interface
func (tc *TableComponent) Render(provider core.Provider, cell *entity.Cell) {
	if len(tc.Headers) == 0 && len(tc.Rows) == 0 {
		return
	}

	// Try to get Fpdf interface for advanced drawing
	drawingHelper := fpdf.NewDrawingHelper(provider)
	useAdvancedDrawing := drawingHelper != nil && tc.ShowBorders

	// Calculate dimensions
	numCols := len(tc.Headers)
	if numCols == 0 && len(tc.Rows) > 0 {
		numCols = len(tc.Rows[0])
	}
	if numCols == 0 {
		return
	}

	colWidth := cell.Width / float64(numCols)
	rowHeight := tc.getRowHeight()

	// Start positioning from the top of the cell
	currentY := cell.Y

	// Render header if present
	if len(tc.Headers) > 0 {
		tc.renderHeaderRow(provider, drawingHelper, cell.X, currentY, colWidth, rowHeight, useAdvancedDrawing)
		currentY += rowHeight
	}

	// Render data rows
	for i, row := range tc.Rows {
		isAltRow := i%2 == 1
		tc.renderDataRow(provider, drawingHelper, cell.X, currentY, colWidth, rowHeight, row, isAltRow, useAdvancedDrawing)
		currentY += rowHeight
	}
}

// GetHeight implements core.Component interface
func (tc *TableComponent) GetHeight(provider core.Provider, cell *entity.Cell) float64 {
	numRows := len(tc.Rows)
	if len(tc.Headers) > 0 {
		numRows++ // Add header row
	}

	// Simple calculation based on font size
	baseHeight := float64(tc.RowStyle.Size) * 0.5 // mm per row
	if baseHeight == 0 {
		baseHeight = 4.0 // default row height
	}
	return float64(numRows) * baseHeight
}

// SetConfig implements core.Node interface
func (tc *TableComponent) SetConfig(config *entity.Config) {
	tc.config = config
}

// GetStructure implements core.Node interface
func (tc *TableComponent) GetStructure() *node.Node[core.Structure] {
	str := core.Structure{
		Type:  "tablecomponent",
		Value: "Custom table component",
		Details: map[string]interface{}{
			"headers": tc.Headers,
			"rows":    len(tc.Rows),
		},
	}

	return node.New(str)
}

// getRowHeight calculates the height needed for each row
func (tc *TableComponent) getRowHeight() float64 {
	// Base height on font size with padding
	headerSize := float64(tc.HeaderStyle.Size)
	rowSize := float64(tc.RowStyle.Size)

	maxSize := headerSize
	if rowSize > maxSize {
		maxSize = rowSize
	}

	// Convert font size to height with padding (rough calculation)
	return (maxSize * 0.35) + 4 // 0.35mm per point + 4mm padding
}

// renderHeaderRow renders the header row with backgrounds and borders
func (tc *TableComponent) renderHeaderRow(provider core.Provider, drawingHelper *fpdf.DrawingHelper, x, y, colWidth, rowHeight float64, useAdvancedDrawing bool) {
	for i, header := range tc.Headers {
		cellX := x + float64(i)*colWidth

		// Create cell for this header
		headerCell := &entity.Cell{
			X:      cellX,
			Y:      y,
			Width:  colWidth,
			Height: rowHeight,
		}

		// Draw advanced graphics if available
		if useAdvancedDrawing {
			tc.drawCellBackground(drawingHelper, headerCell, tc.HeaderBgColor)
			tc.drawCellBorders(drawingHelper, headerCell)

			// Draw text directly for proper Z-order
			tc.drawCellText(drawingHelper, header, headerCell, tc.HeaderStyle)
		} else {
			// Fallback to standard Maroto text rendering
			textComponent := text.New(header, tc.HeaderStyle)
			textComponent.Render(provider, headerCell)
		}
	}
}

// renderDataRow renders a data row with alternating backgrounds and borders
func (tc *TableComponent) renderDataRow(provider core.Provider, drawingHelper *fpdf.DrawingHelper, x, y, colWidth, rowHeight float64, row []string, isAltRow, useAdvancedDrawing bool) {
	bgColor := tc.RowBgColor
	if isAltRow && tc.AltRowBgColor != nil {
		bgColor = tc.AltRowBgColor
	}

	for i, cellData := range row {
		if i >= len(tc.Headers) && len(tc.Headers) > 0 {
			break // Don't exceed number of columns if headers are defined
		}

		cellX := x + float64(i)*colWidth

		// Create cell for this data
		dataCell := &entity.Cell{
			X:      cellX,
			Y:      y,
			Width:  colWidth,
			Height: rowHeight,
		}

		// Draw advanced graphics if available
		if useAdvancedDrawing {
			tc.drawCellBackground(drawingHelper, dataCell, bgColor)
			tc.drawCellBorders(drawingHelper, dataCell)

			// Draw text directly for proper Z-order
			tc.drawCellText(drawingHelper, cellData, dataCell, tc.RowStyle)
		} else {
			// Fallback to standard Maroto text rendering
			textComponent := text.New(cellData, tc.RowStyle)
			textComponent.Render(provider, dataCell)
		}
	}
}

// drawCellBackground draws the background color for a cell using Fpdf
func (tc *TableComponent) drawCellBackground(drawingHelper *fpdf.DrawingHelper, cell *entity.Cell, color *props.Color) {
	if drawingHelper == nil || color == nil {
		return
	}

	// Set fill color and draw rectangle
	drawingHelper.SetFillColor(color.Red, color.Green, color.Blue)
	drawingHelper.DrawRect(cell.X, cell.Y, cell.Width, cell.Height, "F")
}

// drawCellBorders draws borders around a cell using Fpdf
func (tc *TableComponent) drawCellBorders(drawingHelper *fpdf.DrawingHelper, cell *entity.Cell) {
	if drawingHelper == nil || tc.BorderColor == nil {
		return
	}

	// Set border color and draw rectangle border
	drawingHelper.SetDrawColor(tc.BorderColor.Red, tc.BorderColor.Green, tc.BorderColor.Blue)
	drawingHelper.DrawRect(cell.X, cell.Y, cell.Width, cell.Height, "D")
}

// drawCellText draws text directly using Fpdf for proper Z-order above backgrounds
func (tc *TableComponent) drawCellText(drawingHelper *fpdf.DrawingHelper, text string, cell *entity.Cell, style props.Text) {
	if drawingHelper == nil {
		return
	}

	// Get direct access to the underlying Fpdf interface
	fpdfInterface := drawingHelper.GetFpdf()
	if fpdfInterface == nil {
		return
	}

	// Use type assertion to access Cell method for proper Z-order rendering
	if fpdfObj, ok := fpdfInterface.(interface {
		SetFont(familyStr, styleStr string, size float64)
		SetTextColor(r, g, b int)
		SetXY(x, y float64)
		CellFormat(w, h float64, txtStr string, borderStr string, ln int, alignStr string, fill bool, link int, linkStr string)
		GetStringWidth(s string) float64
	}); ok {
		// Set font properties
		fontFamily := style.Family
		if fontFamily == "" {
			fontFamily = "Arial" // Default font
		}

		fontStyleStr := ""
		if style.Style == fontstyle.Bold {
			fontStyleStr = "B"
		} else if style.Style == fontstyle.Italic {
			fontStyleStr = "I"
		}

		fontSize := style.Size
		if fontSize <= 0 {
			fontSize = 8 // Default size
		}

		fpdfObj.SetFont(fontFamily, fontStyleStr, fontSize)

		// Set text color
		if style.Color != nil {
			fpdfObj.SetTextColor(style.Color.Red, style.Color.Green, style.Color.Blue)
		} else {
			fpdfObj.SetTextColor(0, 0, 0) // Default to black
		}

		// Determine alignment string for CellFormat
		var alignStr string
		switch style.Align {
		case align.Center:
			alignStr = "C"
		case align.Right:
			alignStr = "R"
		default: // Left align (default)
			alignStr = "L"
		}

		// Add vertical alignment modifier for top alignment
		if tc.TopAlign {
			alignStr = "T" + alignStr // TL, TC, or TR for top-aligned text
		} else {
			alignStr = "M" + alignStr // ML, MC, or MR for middle-aligned text
		}

		// Position the cell and draw text
		fpdfObj.SetXY(cell.X, cell.Y)

		// Use CellFormat to draw the text with proper positioning
		// Parameters: width, height, text, border, line break, alignment, fill, link, linkStr
		fpdfObj.CellFormat(cell.Width, cell.Height, text, "", 0, alignStr, false, 0, "")
	}
}
