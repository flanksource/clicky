package pdf

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strings"

	"github.com/flanksource/maroto/v2/pkg/components/col"
	"github.com/flanksource/maroto/v2/pkg/components/line"
	"github.com/flanksource/maroto/v2/pkg/components/row"
	"github.com/flanksource/maroto/v2/pkg/components/text"
	"github.com/flanksource/maroto/v2/pkg/consts/align"
	"github.com/flanksource/maroto/v2/pkg/consts/fontstyle"
	"github.com/flanksource/maroto/v2/pkg/core"
	"github.com/flanksource/maroto/v2/pkg/core/entity"
	"github.com/flanksource/maroto/v2/pkg/fpdf"
	"github.com/flanksource/maroto/v2/pkg/props"
	"github.com/johnfercher/go-tree/node"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
)

// Component provides a base struct for components that need direct FPDF access
type Component struct {
	Fpdf       FpdfInterface
	RenderFunc func(*entity.Cell) // Component-specific render function
}

// Render implements core.Component interface
func (c *Component) Render(provider core.Provider, cell *entity.Cell) {
	// Initialize FPDF interface once
	c.initializeFpdf(provider)

	// Call component-specific render function
	if c.RenderFunc != nil {
		c.RenderFunc(cell)
	}
}

// initializeFpdf initializes the FPDF interface from the provider
func (c *Component) initializeFpdf(provider core.Provider) {
	if c.Fpdf != nil {
		return // Already initialized
	}

	drawingHelper := fpdf.NewDrawingHelper(provider)
	fpdfInterface := drawingHelper.GetFpdf()
	c.Fpdf = WrapFpdf(fpdfInterface)
}

// SetConfig implements core.Node interface
func (c *Component) SetConfig(config *entity.Config) {
	// Base components don't need config handling
}

// GetStructure implements core.Node interface
func (c *Component) GetStructure() *node.Node[core.Structure] {
	// Base components don't need structure handling
	return nil
}

// Column represents a table column with comprehensive Tailwind styling
type Column struct {
	Label   string `json:"label"`              // Column header text
	Style   string `json:"style,omitempty"`    // All-in-one Tailwind: "w-[10ch] text-center align-middle font-bold"
	DataKey string `json:"data_key,omitempty"` // Key for data extraction

	// Private resolved fields (internal use only)
	resolvedStyle api.Class                `json:"-"`
	resolvedWidth *tailwind.WidthSpec      `json:"-"`
	resolvedAlign tailwind.ParsedAlignment `json:"-"`
}

// BaseTable contains the shared table implementation using public Tailwind utilities
type BaseTable struct {
	Columns []Column `json:"columns"`        // Column definitions
	Rows    [][]any  `json:"rows,omitempty"` // Data rows

	// Tailwind-only styling
	HeaderStyle       string `json:"header_style,omitempty"`        // "font-bold bg-blue-200 text-center align-middle"
	RowStyle          string `json:"row_style,omitempty"`           // "text-gray-800 align-middle"
	AlternateRowStyle string `json:"alternate_row_style,omitempty"` // "bg-gray-50" (merged with RowStyle)

	// Table options
	ShowBorders bool `json:"show_borders,omitempty"`
	CompactMode bool `json:"compact_mode,omitempty"`

	// Private resolved styles (internal)
	resolvedHeaderStyle    api.Class `json:"-"`
	resolvedRowStyle       api.Class `json:"-"`
	resolvedAlternateStyle api.Class `json:"-"` // RowStyle + AlternateRowStyle merged
	resolvedColumns        []Column  `json:"-"` // Columns with resolved private fields
	calculatedWidths       []float64 `json:"-"` // Calculated column widths in points/pixels
}

// FontMetrics provides font measurement capabilities for width calculations
type FontMetrics struct {
	CharWidth float64 // Average character width in points
	RemSize   float64 // 1rem in points (typically 16px = 12pt)
}

// DefaultFontMetrics provides reasonable defaults for PDF font metrics
var DefaultFontMetrics = FontMetrics{
	CharWidth: 5.5,  // Average character width for typical PDF fonts
	RemSize:   12.0, // 1rem = 12pt for PDF
}

// resolveStyles resolves all Tailwind styles and caches the results
func (bt *BaseTable) resolveStyles() {
	// Resolve header style
	if bt.HeaderStyle != "" {
		bt.resolvedHeaderStyle = api.ResolveStyles(bt.HeaderStyle)
	}

	// Resolve row styles
	if bt.RowStyle != "" {
		bt.resolvedRowStyle = api.ResolveStyles(bt.RowStyle)
	}

	// Merge alternate row style with base row style
	if bt.AlternateRowStyle != "" {
		alternateStyle := tailwind.MergeStyles(bt.RowStyle, bt.AlternateRowStyle)
		bt.resolvedAlternateStyle = api.ResolveStyles(alternateStyle)
	} else {
		bt.resolvedAlternateStyle = bt.resolvedRowStyle
	}

	// Resolve column styles
	bt.resolvedColumns = make([]Column, len(bt.Columns))
	for i, column := range bt.Columns {
		bt.resolvedColumns[i] = column
		if column.Style != "" {
			// Resolve main style using existing api
			bt.resolvedColumns[i].resolvedStyle = api.ResolveStyles(column.Style)

			// Extract layout information using our new utilities
			colWidth, colAlign := tailwind.ResolveLayoutFromStyle(column.Style)
			bt.resolvedColumns[i].resolvedWidth = colWidth
			bt.resolvedColumns[i].resolvedAlign = colAlign
		}
	}
}

// calculateColumnWidths calculates actual column widths based on available space
func (bt *BaseTable) calculateColumnWidths(availableWidth float64) []float64 {
	return bt.calculateColumnWidthsWithMargins(availableWidth, 0, 0, false, false)
}

// calculateColumnWidthsWithMargins calculates column widths considering page margins
func (bt *BaseTable) calculateColumnWidthsWithMargins(availableWidth, leftMargin, rightMargin float64, useMargins, debug bool) []float64 {
	if len(bt.resolvedColumns) == 0 {
		return []float64{}
	}

	// Adjust available width for margins if requested
	adjustedWidth := availableWidth
	if useMargins {
		adjustedWidth = availableWidth - leftMargin - rightMargin
		if adjustedWidth < 0 {
			adjustedWidth = availableWidth * 0.1 // Fallback to 10% if margins are too large
		}

		// Add debug logging if requested
		if debug {
			log.Printf("DEBUG: TableComponent.calculateColumnWidthsWithMargins: available width adjusted from %.2f to %.2f (margins L=%.2f R=%.2f)",
				availableWidth, adjustedWidth, leftMargin, rightMargin)
		}
	}

	numColumns := len(bt.resolvedColumns)
	widths := make([]float64, numColumns)
	metrics := DefaultFontMetrics

	// Phase 1: Calculate base widths for columns with explicit specifications
	totalExplicitWidth := 0.0
	autoColumns := []int{}

	for i, column := range bt.resolvedColumns {
		if column.resolvedWidth != nil {
			width := bt.calculateActualWidth(column.resolvedWidth, adjustedWidth, metrics)
			widths[i] = width
			totalExplicitWidth += width
		} else {
			autoColumns = append(autoColumns, i)
		}
	}

	// Phase 2: Distribute remaining width among auto columns
	remainingWidth := adjustedWidth - totalExplicitWidth
	if len(autoColumns) > 0 && remainingWidth > 0 {
		autoWidth := remainingWidth / float64(len(autoColumns))
		for _, i := range autoColumns {
			widths[i] = autoWidth
		}
	} else if len(autoColumns) > 0 {
		// Fallback: equal distribution of total width
		equalWidth := adjustedWidth / float64(numColumns)
		for _, i := range autoColumns {
			widths[i] = equalWidth
		}
	}

	// Phase 3: Handle overflow by proportionally scaling down
	totalWidth := 0.0
	for _, w := range widths {
		totalWidth += w
	}

	if totalWidth > adjustedWidth {
		scaleFactor := adjustedWidth / totalWidth
		for i := range widths {
			widths[i] *= scaleFactor
		}
	}

	bt.calculatedWidths = widths
	return widths
}

// calculateActualWidth converts a WidthSpec to actual width in points
func (bt *BaseTable) calculateActualWidth(spec *tailwind.WidthSpec, availableWidth float64, metrics FontMetrics) float64 {
	var baseWidth float64

	switch spec.Type {
	case tailwind.WidthAuto:
		// Estimate based on content (simplified - could be enhanced)
		baseWidth = metrics.CharWidth * 15 // Default to ~15 characters

	case tailwind.WidthPercentage:
		baseWidth = availableWidth * (spec.Value / 100)

	case tailwind.WidthCharacter:
		baseWidth = metrics.CharWidth * spec.Value

	case tailwind.WidthPixel:
		// Convert pixels to points (1px ≈ 0.75pt for PDF)
		baseWidth = spec.Value * 0.75

	case tailwind.WidthRem:
		baseWidth = spec.Value * metrics.RemSize

	default:
		baseWidth = availableWidth / float64(len(bt.Columns)) // Fallback
	}

	// Apply min/max constraints
	if spec.IsMin {
		return math.Max(baseWidth, spec.Value*metrics.CharWidth) // Assume constraint is in character units for simplicity
	}
	if spec.IsMax {
		return math.Min(baseWidth, spec.Value*metrics.CharWidth)
	}

	return baseWidth
}

// Table implements the Widget interface for document flow usage
type Table struct {
	BaseTable
}

// Draw implements the Widget interface
func (t Table) Draw(b *Builder) error {
	// Resolve styles if not already done
	t.resolveStyles()

	// Calculate available width (12-column grid system)
	availableWidth := 12.0 // Maroto grid units
	columnWidths := t.calculateColumnWidths(availableWidth)

	// Convert float widths to integer grid units
	gridWidths := make([]int, len(columnWidths))
	totalGridWidth := 0
	for i, width := range columnWidths {
		gridWidths[i] = int(math.Round(width))
		totalGridWidth += gridWidths[i]
	}

	// Adjust to ensure total is exactly 12
	if totalGridWidth != 12 && len(gridWidths) > 0 {
		diff := 12 - totalGridWidth
		gridWidths[len(gridWidths)-1] += diff // Adjust last column
	}

	return t.renderAsWidget(b, gridWidths)
}

// renderAsWidget renders the table using Maroto's widget system
func (t Table) renderAsWidget(b *Builder, colWidths []int) error {
	if len(t.Columns) == 0 && len(t.Rows) == 0 {
		return nil
	}

	// Calculate row height
	baseHeight := 8.0 // Default row height in mm
	if t.CompactMode {
		baseHeight = 6.0
	}

	// Draw top border if enabled
	if t.ShowBorders {
		t.drawHorizontalLine(b, 0.5, 200, sumArray(colWidths))
	}

	// Draw headers if present
	if len(t.Columns) > 0 {
		t.drawHeaderRow(b, colWidths, baseHeight)

		// Draw separator line after headers
		if t.ShowBorders {
			t.drawHorizontalLine(b, 0.5, 200, sumArray(colWidths))
		} else {
			t.drawHorizontalLine(b, 0.3, 230, sumArray(colWidths))
		}
	}

	// Draw data rows
	t.drawDataRows(b, colWidths, baseHeight)

	// Draw bottom border if enabled
	if t.ShowBorders || len(t.Rows) > 0 {
		t.drawHorizontalLine(b, 0.5, 200, sumArray(colWidths))
	}

	// Add spacing after table
	b.maroto.AddRows(row.New(2))

	return nil
}

// drawHeaderRow draws the header row using resolved styles
func (t Table) drawHeaderRow(b *Builder, colWidths []int, baseHeight float64) {
	// Convert resolved header style to text props
	headerTextProps := b.style.ConvertToTextProps(t.resolvedHeaderStyle)

	// Create columns for headers
	cols := make([]core.Col, 0, len(t.Columns))
	totalColWidth := 0

	for i, column := range t.resolvedColumns {
		if i >= len(colWidths) {
			break
		}

		// Apply alignment from resolved column styles
		textProps := *headerTextProps
		textProps.Align = t.convertAlignment(column.resolvedAlign.Horizontal)

		// Create column with header text
		headerCol := col.New(colWidths[i]).Add(
			text.New(column.Label, textProps),
		)

		// Add background if specified in header style
		if t.resolvedHeaderStyle.Background != nil {
			bgColor := b.style.ConvertBackgroundColor(*t.resolvedHeaderStyle.Background)
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

// drawDataRows draws all data rows with alternating styles
func (t Table) drawDataRows(b *Builder, colWidths []int, baseHeight float64) {
	for rowIndex, dataRow := range t.Rows {
		// Determine which style to use (row or alternate)
		var rowStyle api.Class
		if rowIndex%2 == 1 && t.AlternateRowStyle != "" {
			rowStyle = t.resolvedAlternateStyle
		} else {
			rowStyle = t.resolvedRowStyle
		}

		rowTextProps := b.style.ConvertToTextProps(rowStyle)

		cols := make([]core.Col, 0, len(dataRow))
		totalColWidth := 0

		for colIndex := 0; colIndex < len(colWidths) && colIndex < len(dataRow); colIndex++ {
			cellText := fmt.Sprintf("%v", dataRow[colIndex])

			// Apply column alignment if available
			textProps := *rowTextProps
			if colIndex < len(t.resolvedColumns) {
				textProps.Align = t.convertAlignment(t.resolvedColumns[colIndex].resolvedAlign.Horizontal)
			}

			cellCol := col.New(colWidths[colIndex]).Add(
				text.New(cellText, textProps),
			)

			// Add background if specified in row style
			if rowStyle.Background != nil {
				bgColor := b.style.ConvertBackgroundColor(*rowStyle.Background)
				cellCol = cellCol.WithStyle(&props.Cell{
					BackgroundColor: bgColor,
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
		if t.ShowBorders && rowIndex < len(t.Rows)-1 {
			t.drawHorizontalLine(b, 0.2, 240, sumArray(colWidths))
		}
	}
}

// drawHorizontalLine draws a horizontal line across the table
func (t Table) drawHorizontalLine(b *Builder, thickness float64, grayLevel, totalColumns int) {
	b.maroto.AddRow(0.5, col.New(totalColumns).Add(line.New(props.Line{
		Color:     &props.Color{Red: grayLevel, Green: grayLevel, Blue: grayLevel},
		Thickness: thickness,
	})))
}

// convertAlignment converts tailwind alignment to Maroto alignment
func (t Table) convertAlignment(alignment align.Type) align.Type {
	return alignment // Direct mapping as they use the same enum
}

// sumArray sums all values in an integer array
func sumArray(arr []int) int {
	sum := 0
	for _, v := range arr {
		sum += v
	}
	return sum
}

// TableComponent implements the Component interface for positioned usage
type TableComponent struct {
	Component                       // Embedded base component with FPDF access
	BaseTable
	TopAlign       bool            // Force top alignment within cell
	styleConverter *StyleConverter // For proper Unicode font handling
	Debug          bool            // Enable debug mode for detailed positioning logs
}


// NewTableComponent creates a new table component with default Tailwind styling
func NewTableComponent(headers []string, rows [][]string) *TableComponent {
	columns := make([]Column, len(headers))

	// Calculate equal column widths based on number of columns
	numCols := len(headers)
	if numCols == 0 {
		numCols = 1 // Prevent division by zero
	}
	equalWidthPercent := 100.0 / float64(numCols)

	for i, header := range headers {
		columns[i] = Column{
			Label: header,
			Style: fmt.Sprintf("w-[%.1f%%] text-sm text-gray-800 text-left align-middle", equalWidthPercent),
		}
	}

	// Convert rows to [][]any
	convertedRows := make([][]any, len(rows))
	for i, row := range rows {
		convertedRows[i] = make([]any, len(row))
		for j, cell := range row {
			convertedRows[i][j] = cell
		}
	}

	tc := &TableComponent{
		BaseTable: BaseTable{
			Columns:           columns,
			Rows:              convertedRows,
			HeaderStyle:       "font-bold text-white bg-blue-600 text-center text-sm",
			RowStyle:          "text-sm text-gray-800",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
		TopAlign:       true,
		styleConverter: NewStyleConverter(), // Initialize StyleConverter for proper Unicode support
	}

	// Set the component's render function to point to our method
	tc.Component.RenderFunc = tc.renderComponent

	return tc
}

// renderComponent is the component-specific render function
func (tc *TableComponent) renderComponent(cell *entity.Cell) {
	// Resolve styles if not already done
	tc.resolveStyles()

	// Fix margin positioning
	adjustedCell := tc.adjustForMargins(cell)

	// Calculate column widths and render the table
	columnWidths := tc.calculateColumnWidths(adjustedCell.Width)
	tc.renderAsComponent(adjustedCell, columnWidths)
}

// adjustForMargins adjusts cell coordinates for page margins
func (tc *TableComponent) adjustForMargins(cell *entity.Cell) *entity.Cell {
	if tc.Fpdf == nil {
		return cell // No adjustment if FPDF not available
	}

	leftMargin, topMargin, _, _ := tc.Fpdf.GetMargins()

	adjustedCell := *cell
	// Investigation: Test if we need to adjust for margins or if Maroto handles this

	if tc.Debug {
		log.Printf("DEBUG: TableComponent.adjustForMargins: original=(%.2f,%.2f) margins=(%.2f,%.2f)",
			cell.X, cell.Y, leftMargin, topMargin)
	}

	return &adjustedCell
}


// renderAsComponent renders the table as a positioned component
func (tc *TableComponent) renderAsComponent(cell *entity.Cell, colWidths []float64) {
	if len(tc.Columns) == 0 && len(tc.Rows) == 0 {
		if tc.Debug {
			log.Printf("DEBUG: TableComponent.renderAsComponent: empty table, skipping render")
		}
		return
	}

	// Force standard text rendering for TableComponent to avoid empty cells
	// Advanced drawing mode has incomplete drawCellText implementation
	useAdvancedDrawing := true // Disabled to ensure text content is visible

	// Calculate dimensions
	numCols := len(tc.Columns)
	if numCols == 0 {
		return
	}

	rowHeight := tc.getRowHeight()

	// Start positioning from the top of the cell
	currentY := cell.Y

	if tc.Debug {
		log.Printf("DEBUG: TableComponent.renderAsComponent: rendering table with %d columns, %d rows", numCols, len(tc.Rows))
		log.Printf("DEBUG: TableComponent.renderAsComponent: starting Y position=%.2f, row height=%.2f", currentY, rowHeight)
		totalHeight := float64(len(tc.Rows)+1) * rowHeight // +1 for header
		log.Printf("DEBUG: TableComponent.renderAsComponent: estimated total height=%.2f", totalHeight)
	}

	// Render header if present
	if len(tc.Columns) > 0 {
		if tc.Debug {
			log.Printf("DEBUG: TableComponent.renderAsComponent: rendering header at Y=%.2f", currentY)
		}
		tc.renderHeaderRowComponent(cell.X, currentY, colWidths, rowHeight, useAdvancedDrawing)
		currentY += rowHeight
	}

	// Render data rows
	for i, row := range tc.Rows {
		isAltRow := i%2 == 1
		if tc.Debug {
			log.Printf("DEBUG: TableComponent.renderAsComponent: rendering row %d at Y=%.2f", i, currentY)
		}
		tc.renderDataRowComponent(cell.X, currentY, colWidths, rowHeight, row, isAltRow, useAdvancedDrawing)
		currentY += rowHeight
	}

	if tc.Debug {
		log.Printf("DEBUG: TableComponent.renderAsComponent: final Y position=%.2f", currentY)
	}
}

// getRowHeight calculates the height needed for each row using font metrics
func (tc *TableComponent) getRowHeight() float64 {
	// Use actual font metrics if FPDF is available
	if tc.Fpdf != nil {
		fontSizePoints, _ := tc.Fpdf.GetFontSize()
		fontSize := NewFontSize(fontSizePoints)

		// Get style padding (convert Tailwind padding to mm)
		paddingMM := NewMM(tc.extractPaddingFromStyle(tc.resolvedRowStyle))

		// Calculate height: font size + line spacing + padding
		lineHeightMM := fontSize.LineHeight(1.2) // 20% line spacing
		baseHeightMM := lineHeightMM.Add(paddingMM.Multiply(2)) // Top + bottom padding

		if tc.CompactMode {
			baseHeightMM = baseHeightMM.Multiply(0.8) // 20% reduction for compact mode
		}

		if tc.Debug {
			log.Printf("DEBUG: TableComponent.getRowHeight: font=%s, calculated=%s (compact=%v)",
				fontSize.String(), baseHeightMM.String(), tc.CompactMode)
		}

		return baseHeightMM.Float64()
	}

	// Fallback to hardcoded values if FPDF not available
	baseHeight := 8.0 // 8mm default row height
	if tc.CompactMode {
		baseHeight = 6.0
	}

	if tc.Debug {
		log.Printf("DEBUG: TableComponent.getRowHeight: fallback height=%.2fmm (compact=%v)", baseHeight, tc.CompactMode)
	}

	return baseHeight
}

// extractPaddingFromStyle extracts padding from Tailwind style classes
func (tc *TableComponent) extractPaddingFromStyle(style api.Class) float64 {
	// Extract padding from Tailwind classes (p-1, p-2, etc.)
	// For now, return a reasonable default
	// TODO: Parse actual Tailwind padding values
	return 2.0 // Default 2mm padding
}

// renderHeaderRowComponent renders the header row for component interface
func (tc *TableComponent) renderHeaderRowComponent(x, y float64, colWidths []float64, rowHeight float64, useAdvancedDrawing bool) {
	if tc.Debug {
		log.Printf("DEBUG: TableComponent.renderHeaderRowComponent: rendering header row at x=%.2f, y=%.2f, rowHeight=%.2f", x, y, rowHeight)
	}

	for i, column := range tc.resolvedColumns {
		if i >= len(colWidths) {
			break
		}

		cellX := x + tc.sumWidths(colWidths[:i])

		// Create cell for this header
		headerCell := &entity.Cell{
			X:      cellX,
			Y:      y,
			Width:  colWidths[i],
			Height: rowHeight,
		}

		if tc.Debug {
			log.Printf("DEBUG: TableComponent.renderHeaderRowComponent: header cell[%d] '%s' at (%.2f,%.2f) size(%.2f×%.2f)",
				i, column.Label, cellX, y, colWidths[i], rowHeight)
		}

		if useAdvancedDrawing {
			// Use advanced drawing with backgrounds and borders
			if err := tc.renderCellWithStyle(column.Label, headerCell, tc.resolvedHeaderStyle, column.resolvedAlign); err != nil {
				log.Printf("ERROR: Failed to render header cell for column %d (%s): %v", i, column.Label, err)
				// Continue with next column instead of failing completely
			}
		} else {
			// Fallback to standard text rendering using our direct FPDF access
			headerTextProps := tc.convertStyleToTextProps(tc.resolvedHeaderStyle, column.resolvedAlign)
			if err := tc.drawCellText(column.Label, headerCell, headerTextProps); err != nil {
				log.Printf("ERROR: Failed to render header text for column %d (%s): %v", i, column.Label, err)
			}
		}
	}
}

// renderDataRowComponent renders a data row for component interface
func (tc *TableComponent) renderDataRowComponent(x, y float64, colWidths []float64, rowHeight float64, row []any, isAltRow, useAdvancedDrawing bool) {
	// Determine which style to use
	var rowStyle api.Class
	if isAltRow && tc.AlternateRowStyle != "" {
		rowStyle = tc.resolvedAlternateStyle
	} else {
		rowStyle = tc.resolvedRowStyle
	}

	for i, cellData := range row {
		if i >= len(colWidths) || i >= len(tc.resolvedColumns) {
			break
		}

		cellX := x + tc.sumWidths(colWidths[:i])
		cellText := fmt.Sprintf("%v", cellData)

		// Create cell for this data
		dataCell := &entity.Cell{
			X:      cellX,
			Y:      y,
			Width:  colWidths[i],
			Height: rowHeight,
		}

		// Use advanced drawing with backgrounds and borders
		if err := tc.renderCellWithStyle(cellText, dataCell, rowStyle, tc.resolvedColumns[i].resolvedAlign); err != nil {
			log.Printf("ERROR: Failed to render data cell for row %d, column %d (%s): %v", len(tc.Rows), i, cellText, err)
			// Continue with next cell instead of failing completely
		}
	}
}

// renderCellWithStyle renders a cell with full styling support
func (tc *TableComponent) renderCellWithStyle(text string, cell *entity.Cell, style api.Class, alignment tailwind.ParsedAlignment) error {
	// Draw background
	if style.Background != nil {
		tc.drawCellBackground(cell, tc.convertToPropsColor(*style.Background))
	}

	// Draw borders
	if tc.ShowBorders {
		tc.drawCellBorders(cell)
	}

	// Draw text
	textProps := tc.convertStyleToTextProps(style, alignment)
	if err := tc.drawCellText(text, cell, textProps); err != nil {
		return fmt.Errorf("failed to draw cell text: %w", err)
	}

	return nil
}

// Helper methods for component rendering
func (tc *TableComponent) sumWidths(widths []float64) float64 {
	sum := 0.0
	for _, w := range widths {
		sum += w
	}
	return sum
}

func (tc *TableComponent) convertToPropsColor(color api.Color) *props.Color {
	// Parse hex color
	if strings.HasPrefix(color.Hex, "#") && len(color.Hex) == 7 {
		// Simple hex parsing - could be enhanced
		return &props.Color{Red: 128, Green: 128, Blue: 128} // Placeholder
	}
	return &props.Color{Red: 128, Green: 128, Blue: 128}
}

func (tc *TableComponent) convertStyleToTextProps(style api.Class, alignment tailwind.ParsedAlignment) props.Text {
	// Use StyleConverter for proper font handling including Unicode support
	textProps := tc.styleConverter.ConvertToTextProps(style)

	// Apply the specific alignment from Tailwind parsing
	textProps.Align = alignment.Horizontal

	return *textProps
}

func (tc *TableComponent) drawCellBackground(cell *entity.Cell, color *props.Color) {
	if tc.Fpdf == nil || color == nil {
		return
	}
	tc.Fpdf.SetFillColor(color.Red, color.Green, color.Blue)
	tc.Fpdf.Rect(cell.X, cell.Y, cell.Width, cell.Height, "F")
}

func (tc *TableComponent) drawCellBorders(cell *entity.Cell) {
	if tc.Fpdf == nil {
		return
	}
	tc.Fpdf.SetDrawColor(128, 128, 128) // Gray
	tc.Fpdf.Rect(cell.X, cell.Y, cell.Width, cell.Height, "D")
}

// getMarginInfo safely extracts margin and page information from the FPDF interface
func (tc *TableComponent) getMarginInfo() (leftMargin, topMargin, rightMargin, bottomMargin, pageWidth, pageHeight float64, err error) {
	if tc.Fpdf == nil {
		err = errors.New("FPDF interface is nil")
		return
	}

	// Get margin information directly from our FPDF interface
	leftMargin, topMargin, rightMargin, bottomMargin = tc.Fpdf.GetMargins()
	pageWidth, pageHeight = tc.Fpdf.GetPageSize()

	if tc.Debug {
		log.Printf("DEBUG: TableComponent.getMarginInfo: FPDF margins L=%.2f R=%.2f T=%.2f B=%.2f",
			leftMargin, rightMargin, topMargin, bottomMargin)
		usableWidth := pageWidth - leftMargin - rightMargin
		usableHeight := pageHeight - topMargin - bottomMargin
		log.Printf("DEBUG: TableComponent.getMarginInfo: FPDF page size %.2f×%.2f, usable area %.2f×%.2f",
			pageWidth, pageHeight, usableWidth, usableHeight)
	}

	return leftMargin, topMargin, rightMargin, bottomMargin, pageWidth, pageHeight, nil
}

func (tc *TableComponent) drawCellText(text string, cell *entity.Cell, style props.Text) error {
	if text == "" {
		return nil // Empty text is not an error, just return success
	}

	if tc.Fpdf == nil {
		err := errors.New("FPDF interface is nil")
		log.Printf("ERROR: TableComponent.drawCellText: %v", err)
		return err
	}

	if cell == nil {
		err := errors.New("cell is nil")
		log.Printf("ERROR: TableComponent.drawCellText: %v", err)
		return err
	}

	// Configure font with Unicode support
	fontFamily := style.Family
	if fontFamily == "" {
		fontFamily = "Arial" // Default font with Unicode support
	}

	// Convert fontstyle enum to string
	fontStyleStr := ""
	switch style.Style {
	case fontstyle.Bold:
		fontStyleStr = "B"
	case fontstyle.Italic:
		fontStyleStr = "I"
	case fontstyle.BoldItalic:
		fontStyleStr = "BI"
	default:
		fontStyleStr = ""
	}

	// Validate and set font size
	fontSize := style.Size
	if fontSize <= 0 {
		fontSize = 8.0 // Default size
	}

	// Set font properties
	tc.Fpdf.SetFont(fontFamily, fontStyleStr, fontSize)

	// Set text color
	if style.Color != nil {
		tc.Fpdf.SetTextColor(style.Color.Red, style.Color.Green, style.Color.Blue)
	} else {
		tc.Fpdf.SetTextColor(0, 0, 0) // Default to black
	}

	// Calculate alignment string for CellFormat
	var alignStr string
	switch style.Align {
	case align.Center:
		alignStr = "C"
	case align.Right:
		alignStr = "R"
	case align.Justify:
		alignStr = "J"
	default: // Left align (default)
		alignStr = "L"
	}

	// Add vertical alignment modifier
	if tc.TopAlign {
		alignStr = "T" + alignStr // Top-aligned: TL, TC, TR, TJ
	} else {
		alignStr = "M" + alignStr // Middle-aligned: ML, MC, MR, MJ
	}

	// Validate cell dimensions
	if cell.Width <= 0 || cell.Height <= 0 {
		err := fmt.Errorf("invalid cell dimensions (w=%.2f, h=%.2f)", cell.Width, cell.Height)
		log.Printf("WARNING: TableComponent.drawCellText: %v", err)
		return err
	}

	// Position and render text
	tc.Fpdf.SetXY(cell.X, cell.Y)

	// Use CellFormat for proper text positioning and rendering
	// Parameters: width, height, text, border, line break, alignment, fill, link, linkStr
	tc.Fpdf.CellFormat(cell.Width, cell.Height, text, "", 0, alignStr, false, 0, "")

	return nil
}

// GetHeight implements core.Component interface
func (tc *TableComponent) GetHeight(provider core.Provider, cell *entity.Cell) float64 {
	numRows := len(tc.Rows)
	if len(tc.Columns) > 0 {
		numRows++ // Add header row
	}

	return float64(numRows) * tc.getRowHeight()
}

// SetConfig implements core.Node interface
func (tc *TableComponent) SetConfig(config *entity.Config) {
	// Store config if needed
}

// GetStructure implements core.Node interface
func (tc *TableComponent) GetStructure() *node.Node[core.Structure] {
	str := core.Structure{
		Type:  "unified_table_component",
		Value: "Unified table component with Tailwind styling",
		Details: map[string]interface{}{
			"columns": len(tc.Columns),
			"rows":    len(tc.Rows),
		},
	}

	return node.New(str)
}
