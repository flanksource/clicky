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

// initializeFpdf initializes the FPDF interface directly from the provider
func (c *Component) initializeFpdf(provider core.Provider) {
	if c.Fpdf != nil {
		return // Already initialized
	}

	// The provider itself is typically the FPDF wrapper we need
	// This bypasses the DrawingHelper layer and gives us direct access
	// to all FPDF methods through our expanded interface
	c.Fpdf = WrapFpdf(provider)
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

	// Calculate row height based on actual font size instead of hardcoded values
	defaultFontSize := NewFontSize(12.0)
	if t.CompactMode {
		defaultFontSize = NewFontSize(10.0) // Smaller font for compact
	}
	baseHeight := defaultFontSize.ToMM().Float64()

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
	Component // Embedded base component with FPDF access
	BaseTable
	TopAlign       bool            // Force top alignment within cell
	styleConverter *StyleConverter // For proper Unicode font handling
	Debug          bool            // Enable debug mode for detailed positioning logs
}

// NewTableComponent creates a new table component with default Tailwind styling
func NewTableComponent(headers []string, rows [][]any) *TableComponent {
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
			Style: fmt.Sprintf("w-[%.1f%%] text-sm text-gray-600 text-left align-middle", equalWidthPercent),
		}
	}

	// Rows are already [][]any, no conversion needed
	tc := &TableComponent{
		BaseTable: BaseTable{
			Columns:           columns,
			Rows:              rows,
			HeaderStyle:       "font-bold text-white bg-blue-600 text-center text-sm",
			RowStyle:          "text-sm text-gray-800 bg-white",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
		TopAlign:       true,
		styleConverter: NewStyleConverter(), // Initialize StyleConverter for proper Unicode support
	}

	// Set the component's render function to point to our method
	tc.RenderFunc = tc.renderComponent

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

// getRowHeight calculates the height needed for each row using pure font size + Point padding
func (tc *TableComponent) getRowHeight() float64 {
	if tc.Fpdf != nil {
		// Get actual font size in points
		fontSizePoints, _ := tc.Fpdf.GetFontSize()

		// Convert font size to MM - this is the base text height (no line height multiplier)
		fontHeight := NewFontSize(fontSizePoints).ToMM()

		// Add padding from styles (converted from Points to MM)
		totalHeight := fontHeight
		if tc.resolvedRowStyle.Padding != nil {
			totalHeight += NewMM(tc.resolvedRowStyle.Padding.TopMM())
			totalHeight += NewMM(tc.resolvedRowStyle.Padding.BottomMM())
		}

		if tc.Debug {
			paddingMM := 0.0
			if tc.resolvedRowStyle.Padding != nil {
				paddingMM = tc.resolvedRowStyle.Padding.TopMM() + tc.resolvedRowStyle.Padding.BottomMM()
			}
			log.Printf("DEBUG: TableComponent.getRowHeight: font=%.1fpt (%s), padding=%.2fmm, total=%s",
				fontSizePoints, fontHeight.String(), paddingMM, totalHeight.String())
		}

		return totalHeight.Float64()
	}

	// Fallback: 12pt font converted to MM
	return NewFontSize(12.0).ToMM().Float64()
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
			if err := tc.drawCellText(column.Label, headerCell, headerTextProps, tc.resolvedHeaderStyle); err != nil {
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
	if err := tc.drawCellText(text, cell, textProps, style); err != nil {
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
func (tc *TableComponent) drawCellText(text string, cell *entity.Cell, style props.Text, cellStyle api.Class) error {
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

	// Configure font with proper validation and error handling
	fontFamily := style.Family
	if fontFamily == "" {
		return fmt.Errorf("font family is required - style.Family cannot be empty")
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

	// Validate font size
	fontSize := style.Size
	if fontSize <= 0 {
		return fmt.Errorf("font size must be positive, got %.2f", style.Size)
	}

	// Set font properties with verification
	tc.Fpdf.SetFont(fontFamily, fontStyleStr, fontSize)

	// Verify font was set correctly by checking current font
	actualSize, _ := tc.Fpdf.GetFontSize()
	if actualSize != fontSize {
		if tc.Debug {
			log.Printf("WARNING: TableComponent.drawCellText: font size mismatch - requested %.2f, got %.2f", fontSize, actualSize)
		}
	}

	if tc.Debug {
		log.Printf("DEBUG: TableComponent.drawCellText: set font %s %s %.2fpt", fontFamily, fontStyleStr, fontSize)
	}

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

	// Apply Point-based padding to adjust text position and available space
	var paddingTopMM, paddingLeftMM, paddingRightMM, paddingBottomMM float64
	if cellStyle.Padding != nil {
		paddingTopMM = cellStyle.Padding.TopMM()
		paddingLeftMM = cellStyle.Padding.LeftMM()
		paddingRightMM = cellStyle.Padding.RightMM()
		paddingBottomMM = cellStyle.Padding.BottomMM()
	}

	// Calculate adjusted cell position and dimensions with padding
	adjustedX := cell.X + paddingLeftMM
	adjustedY := cell.Y + paddingTopMM
	adjustedWidth := cell.Width - paddingLeftMM - paddingRightMM
	adjustedHeight := cell.Height - paddingTopMM - paddingBottomMM

	// Ensure adjusted dimensions are valid
	if adjustedWidth <= 0 || adjustedHeight <= 0 {
		// If padding is too large, use original cell dimensions
		adjustedX = cell.X
		adjustedY = cell.Y
		adjustedWidth = cell.Width
		adjustedHeight = cell.Height
	}

	// Position and render text with padding adjustments
	tc.Fpdf.SetXY(adjustedX, adjustedY)

	// Use CellFormat for proper text positioning and rendering
	// Parameters: width, height, text, border, line break, alignment, fill, link, linkStr
	tc.Fpdf.CellFormat(adjustedWidth, adjustedHeight, text, "", 0, alignStr, false, 0, "")

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

// WithStyle applies a custom style to all table cells (merges with existing styles)
func (tc *TableComponent) WithStyle(style string) *TableComponent {
	// Apply to all columns
	for i := range tc.Columns {
		tc.Columns[i].Style = tailwind.MergeStyles(tc.Columns[i].Style, style)
	}
	return tc
}

// WithCellPadding applies padding to all table cells
func (tc *TableComponent) WithCellPadding(padding string) *TableComponent {
	// Apply padding to all columns
	for i := range tc.Columns {
		tc.Columns[i].Style = tailwind.MergeStyles(tc.Columns[i].Style, padding)
	}
	// Also apply to row styles
	tc.RowStyle = tailwind.MergeStyles(tc.RowStyle, padding)
	tc.HeaderStyle = tailwind.MergeStyles(tc.HeaderStyle, padding)
	return tc
}

// WithColumnStyle applies or replaces style for a specific column
func (tc *TableComponent) WithColumnStyle(index int, style string) *TableComponent {
	if index >= 0 && index < len(tc.Columns) {
		tc.Columns[index].Style = style
	}
	return tc
}

// WithHeaderStyle replaces the header style
func (tc *TableComponent) WithHeaderStyle(style string) *TableComponent {
	tc.HeaderStyle = style
	return tc
}

// WithRowStyle replaces the row style
func (tc *TableComponent) WithRowStyle(style string) *TableComponent {
	tc.RowStyle = style
	return tc
}

// WithAlternateRowStyle replaces the alternate row style
func (tc *TableComponent) WithAlternateRowStyle(style string) *TableComponent {
	tc.AlternateRowStyle = style
	return tc
}

// WithCompactMode enables or disables compact mode
func (tc *TableComponent) WithCompactMode(compact bool) *TableComponent {
	tc.CompactMode = compact
	return tc
}

// WithBorders enables or disables table borders
func (tc *TableComponent) WithBorders(show bool) *TableComponent {
	tc.ShowBorders = show
	return tc
}

// NewTable creates a new table component with sensible defaults and fluent API
func NewTable() *TableComponent {
	tc := &TableComponent{
		BaseTable: BaseTable{
			Columns:           []Column{},
			Rows:              [][]any{},
			HeaderStyle:       "font-bold text-white bg-blue-600 text-center text-sm p-2",
			RowStyle:          "text-sm text-gray-800 bg-white p-2",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
		TopAlign:       true,
		styleConverter: NewStyleConverter(),
	}

	// Set the component's render function
	tc.RenderFunc = tc.renderComponent

	return tc
}

// WithHeader adds a single header column with optional style
func (tc *TableComponent) WithHeader(name, style string) *TableComponent {
	column := Column{
		Label:   name,
		DataKey: name, // Use name as default data key
	}

	// Apply style if provided, otherwise use default column style
	if style != "" {
		column.Style = style
	} else {
		column.Style = "text-sm text-gray-600 text-left align-middle"
	}

	tc.Columns = append(tc.Columns, column)
	return tc
}

// WithHeaders adds multiple header columns with default styling
func (tc *TableComponent) WithHeaders(names ...string) *TableComponent {
	for _, name := range names {
		tc.WithHeader(name, "")
	}
	return tc
}

// WithRows adds rows from a slice of maps where keys match column labels/data keys
func (tc *TableComponent) WithRows(data []map[string]any) *TableComponent {
	for _, rowMap := range data {
		row := make([]any, len(tc.Columns))

		// Extract values for each column using DataKey or Label as fallback
		for i, column := range tc.Columns {
			key := column.DataKey
			if key == "" {
				key = column.Label
			}

			if value, exists := rowMap[key]; exists {
				row[i] = value
			} else {
				row[i] = "" // Default to empty string if key not found
			}
		}

		tc.Rows = append(tc.Rows, row)
	}
	return tc
}

// WithRowSlice adds a single row from a slice of values
func (tc *TableComponent) WithRowSlice(row []any) *TableComponent {
	tc.Rows = append(tc.Rows, row)
	return tc
}
