package pdf

import (
	"fmt"

	"github.com/flanksource/clicky/api"
)

type Table struct {
	Headers     []string    `json:"headers,omitempty"`
	Rows        [][]any     `json:"rows,omitempty"`
	HeaderStyle api.Class   `json:"header_style,omitempty"`
	RowStyle    api.Class   `json:"row_style,omitempty"`
	CellPadding api.Padding `json:"cell_padding,omitempty"`
}

func (t Table) GetWidth() float64 {
	// Calculate total table width based on content
	if len(t.Headers) == 0 && len(t.Rows) == 0 {
		return 0
	}

	// Use available page width (A4 width minus margins)
	pageWidth := 210.0 // A4 width in mm
	margins := 20.0    // Total left+right margins
	return pageWidth - margins
}

func (t Table) GetHeight() float64 {
	// Calculate total table height
	if len(t.Headers) == 0 && len(t.Rows) == 0 {
		return 0
	}

	rowHeight := 8.0 // Default row height in mm
	totalRows := len(t.Rows)
	if len(t.Headers) > 0 {
		totalRows++ // Add header row
	}

	// Add padding
	paddingHeight := 0.0
	if t.CellPadding.Top > 0 || t.CellPadding.Bottom > 0 {
		paddingHeight = (t.CellPadding.Top + t.CellPadding.Bottom) * 4.2333 // Convert rem to mm
	}

	return float64(totalRows)*(rowHeight+paddingHeight)
}

func (t Table) Draw(b *Builder, opts ...DrawOptions) error {
	if len(t.Headers) == 0 && len(t.Rows) == 0 {
		return nil // Nothing to draw
	}

	// Parse options
	options := &drawOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Save current state
	savedState := b.GetStyleConverter().SaveCurrentState()

	// Apply position if specified
	if options.Position != (api.Position{}) {
		b.MoveTo(options.Position)
	}

	// Calculate table dimensions
	tableWidth := t.GetWidth()
	numColumns := len(t.Headers)
	if numColumns == 0 && len(t.Rows) > 0 {
		numColumns = len(t.Rows[0])
	}

	columnWidth := tableWidth / float64(numColumns)
	rowHeight := 8.0 // mm

	// Get current position
	startPos := b.GetCurrentPosition()
	currentX := float64(startPos.X)
	currentY := float64(startPos.Y)

	pdf := b.GetPDF()
	style := b.GetStyleConverter()

	// Draw headers if present
	if len(t.Headers) > 0 {
		// Apply header styling
		if t.HeaderStyle != (api.Class{}) {
			style.ApplyClassToPDF(t.HeaderStyle)
		}

		// Draw header cells
		for i, header := range t.Headers {
			cellX := currentX + float64(i)*columnWidth
			
			// Draw cell border
			pdf.Rect(cellX, currentY, columnWidth, rowHeight, "D")
			
			// Draw background if specified
			if t.HeaderStyle.Background != nil {
				pdf.Rect(cellX, currentY, columnWidth, rowHeight, "F")
			}

			// Position text within cell
			textX := cellX + 2 // Small left padding
			textY := currentY + rowHeight/2 + style.GetTextHeight()/2 // Center vertically
			pdf.SetXY(textX, textY)
			
			// Write header text
			pdf.Cell(columnWidth-4, style.GetTextHeight(), header)
		}
		
		currentY += rowHeight
		
		// Restore style after header
		style.RestoreState(savedState)
	}

	// Draw data rows
	if t.RowStyle != (api.Class{}) {
		style.ApplyClassToPDF(t.RowStyle)
	}

	for rowIndex, row := range t.Rows {
		// Check if we need a new page
		if currentY+rowHeight > 280 { // Near bottom of A4
			b.GetPDF().AddPage()
			currentY = float64(b.GetCurrentPosition().Y)
		}

		// Draw row cells
		for colIndex, cell := range row {
			if colIndex >= numColumns {
				break // Don't exceed column count
			}

			cellX := currentX + float64(colIndex)*columnWidth
			
			// Draw cell border
			pdf.Rect(cellX, currentY, columnWidth, rowHeight, "D")
			
			// Draw background if specified and alternating rows
			if t.RowStyle.Background != nil && rowIndex%2 == 0 {
				pdf.Rect(cellX, currentY, columnWidth, rowHeight, "F")
			}

			// Position text within cell
			textX := cellX + 2 // Small left padding
			textY := currentY + rowHeight/2 + style.GetTextHeight()/2 // Center vertically
			pdf.SetXY(textX, textY)
			
			// Convert cell content to string
			cellText := fmt.Sprintf("%v", cell)
			
			// Truncate text if too long for cell
			if style.GetTextWidth(cellText) > columnWidth-4 {
				// Simple truncation - in a more sophisticated version,
				// you might wrap text or use ellipsis
				for len(cellText) > 0 && style.GetTextWidth(cellText+"...") > columnWidth-4 {
					cellText = cellText[:len(cellText)-1]
				}
				if len(cellText) > 0 {
					cellText += "..."
				}
			}
			
			// Write cell text
			pdf.Cell(columnWidth-4, style.GetTextHeight(), cellText)
		}
		
		currentY += rowHeight
	}

	// Update builder position to end of table
	b.MoveTo(api.Position{X: startPos.X, Y: int(currentY)})

	// Restore state
	style.RestoreState(savedState)

	return nil
}
