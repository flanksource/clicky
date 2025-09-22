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

	"github.com/flanksource/clicky/api"
)

// TableComponent implements core.Component interface for embedding tables in columns
type TableComponent struct {
	Headers           []string
	Rows              [][]string
	ShowBorders       bool
	TopAlign          bool // Force top alignment within cell
	CompactMode       bool // Use smaller fonts and tighter spacing
	HeaderClass       string
	CellClass         string
	PrimaryRowClass   string
	AlternateRowClass string
	styleConverter    *StyleConverter
	config            *entity.Config

	// Cached resolved styles to avoid repeated api.ResolveStyles() calls
	resolvedHeaderClass    api.Class
	resolvedCellClass      api.Class
	resolvedPrimaryClass   api.Class
	resolvedAlternateClass api.Class
}

// NewTableComponent creates a new table component with default Tailwind styling
func NewTableComponent(headers []string, rows [][]string) *TableComponent {
	tc := &TableComponent{
		Headers:           headers,
		Rows:              rows,
		ShowBorders:       true,
		TopAlign:          true,
		HeaderClass:       "font-bold text-white bg-blue-600 text-center text-sm",
		CellClass:         "text-sm text-gray-800",
		PrimaryRowClass:   "bg-white",
		AlternateRowClass: "bg-gray-50",
		styleConverter:    NewStyleConverter(),
	}

	// Pre-resolve all styles to avoid repeated calls during rendering
	tc.resolvedHeaderClass = api.ResolveStyles(tc.HeaderClass)
	tc.resolvedCellClass = api.ResolveStyles(tc.CellClass)
	tc.resolvedPrimaryClass = api.ResolveStyles(tc.PrimaryRowClass)
	tc.resolvedAlternateClass = api.ResolveStyles(tc.AlternateRowClass)

	return tc
}

// WithHeaderClass sets the Tailwind classes for header styling
func (tc *TableComponent) WithHeaderClass(class string) *TableComponent {
	tc.HeaderClass = class
	tc.resolvedHeaderClass = api.ResolveStyles(class)
	return tc
}

// WithCellClass sets the Tailwind classes for general cell styling
func (tc *TableComponent) WithCellClass(class string) *TableComponent {
	tc.CellClass = class
	tc.resolvedCellClass = api.ResolveStyles(class)
	return tc
}

// WithPrimaryRowClass sets the Tailwind classes for primary row styling
func (tc *TableComponent) WithPrimaryRowClass(class string) *TableComponent {
	tc.PrimaryRowClass = class
	tc.resolvedPrimaryClass = api.ResolveStyles(class)
	return tc
}

// WithAlternateRowClass sets the Tailwind classes for alternate row styling
func (tc *TableComponent) WithAlternateRowClass(class string) *TableComponent {
	tc.AlternateRowClass = class
	tc.resolvedAlternateClass = api.ResolveStyles(class)
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

// WithCompactMode enables compact mode with smaller fonts and tighter spacing
func (tc *TableComponent) WithCompactMode(enabled bool) *TableComponent {
	tc.CompactMode = enabled
	if enabled {
		// Override classes with compact versions
		tc.HeaderClass = "font-bold text-white bg-blue-600 text-center text-xs"
		tc.CellClass = "text-xs text-gray-800"
	}
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

	// Calculate height based on resolved cell styles
	baseHeight := 4.0 // Default row height in mm

	if tc.styleConverter != nil {
		// Use cached resolved cell class styling
		baseHeight = tc.styleConverter.CalculateTextHeight(tc.resolvedCellClass)
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
	// Use style converter to calculate height from Tailwind classes
	if tc.styleConverter != nil {
		// Use cached resolved styles for height calculation
		headerHeight := tc.styleConverter.CalculateTextHeight(tc.resolvedHeaderClass)
		cellHeight := tc.styleConverter.CalculateTextHeight(tc.resolvedCellClass)

		// Use the larger of the two
		if headerHeight > cellHeight {
			return headerHeight
		}
		return cellHeight
	}

	// Fallback to default height
	return 8.0 // 8mm default row height
}

// renderHeaderRow renders the header row with Tailwind-resolved backgrounds and borders
func (tc *TableComponent) renderHeaderRow(provider core.Provider, drawingHelper *fpdf.DrawingHelper, x, y, colWidth, rowHeight float64, useAdvancedDrawing bool) {
	// Use cached resolved header classes
	headerTextProps := tc.styleConverter.ConvertToTextProps(tc.resolvedHeaderClass)
	headerBgColor := tc.styleConverter.ConvertToTableBackgroundColor(tc.resolvedHeaderClass)

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
			tc.drawCellBackground(drawingHelper, headerCell, headerBgColor)
			tc.drawCellBorders(drawingHelper, headerCell)

			// Draw text directly for proper Z-order
			tc.drawCellText(drawingHelper, header, headerCell, *headerTextProps)
		} else {
			// Fallback to standard Maroto text rendering
			textComponent := text.New(header, *headerTextProps)
			textComponent.Render(provider, headerCell)
		}
	}
}

// renderDataRow renders a data row with alternating backgrounds and borders using Tailwind styles
func (tc *TableComponent) renderDataRow(provider core.Provider, drawingHelper *fpdf.DrawingHelper, x, y, colWidth, rowHeight float64, row []string, isAltRow, useAdvancedDrawing bool) {
	// Use cached resolved row classes
	var rowClass api.Class
	if isAltRow {
		rowClass = tc.resolvedAlternateClass
	} else {
		rowClass = tc.resolvedPrimaryClass
	}

	// For cell text, we'll use the cell class styles combined with row-specific styles
	// Since combining classes is complex, we'll use resolved cell class primarily
	cellTextProps := tc.styleConverter.ConvertToTextProps(tc.resolvedCellClass)
	bgColor := tc.styleConverter.ConvertToTableBackgroundColor(rowClass)

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
			tc.drawCellText(drawingHelper, cellData, dataCell, *cellTextProps)
		} else {
			// Fallback to standard Maroto text rendering
			textComponent := text.New(cellData, *cellTextProps)
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

// drawCellBorders draws borders around a cell using Fpdf with default border color
func (tc *TableComponent) drawCellBorders(drawingHelper *fpdf.DrawingHelper, cell *entity.Cell) {
	if drawingHelper == nil {
		return
	}

	// Use default gray border color (matching Tailwind's default table border)
	drawingHelper.SetDrawColor(128, 128, 128) // Gray
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
		switch style.Style {
		case fontstyle.Bold:
			fontStyleStr = "B"
		case fontstyle.Italic:
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
