package pdf

import (
	"fmt"

	"github.com/flanksource/clicky/api"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
)

// Table widget for rendering tables in PDF
type Table struct {
	Headers     []string    `json:"headers,omitempty"`
	Rows        [][]any     `json:"rows,omitempty"`
	HeaderStyle api.Class   `json:"header_style,omitempty"`
	RowStyle    api.Class   `json:"row_style,omitempty"`
	CellPadding api.Padding `json:"cell_padding,omitempty"`
}

// Draw implements the Widget interface
func (t Table) Draw(b *Builder) error {
	if len(t.Headers) == 0 && len(t.Rows) == 0 {
		return nil // Nothing to draw
	}

	// Determine number of columns
	numColumns := len(t.Headers)
	if numColumns == 0 && len(t.Rows) > 0 {
		numColumns = len(t.Rows[0])
	}

	if numColumns == 0 {
		return nil
	}

	// Calculate column width (12-column grid divided by number of columns)
	colWidth := 12 / numColumns
	if colWidth < 1 {
		colWidth = 1 // Minimum column width
	}

	// Calculate row height
	baseHeight := 8.0 // Default row height in mm
	if t.CellPadding.Top > 0 || t.CellPadding.Bottom > 0 {
		baseHeight += (t.CellPadding.Top + t.CellPadding.Bottom) * 4
	}

	// Draw headers if present
	if len(t.Headers) > 0 {
		// Convert header style
		headerTextProps := b.style.ConvertToTextProps(t.HeaderStyle)
		
		// Create header row based on number of columns
		switch numColumns {
		case 1:
			headerText := text.New(t.Headers[0], *headerTextProps)
			b.maroto.AddRow(baseHeight, col.New(12).Add(headerText))
		case 2:
			h1 := text.New(t.Headers[0], *headerTextProps)
			h2 := ""
			if len(t.Headers) > 1 {
				h2 = t.Headers[1]
			}
			h2Text := text.New(h2, *headerTextProps)
			b.maroto.AddRow(baseHeight, 
				col.New(6).Add(h1),
				col.New(6).Add(h2Text))
		case 3:
			h1 := text.New(t.Headers[0], *headerTextProps)
			h2 := ""
			h3 := ""
			if len(t.Headers) > 1 {
				h2 = t.Headers[1]
			}
			if len(t.Headers) > 2 {
				h3 = t.Headers[2]
			}
			h2Text := text.New(h2, *headerTextProps)
			h3Text := text.New(h3, *headerTextProps)
			b.maroto.AddRow(baseHeight,
				col.New(4).Add(h1),
				col.New(4).Add(h2Text),
				col.New(4).Add(h3Text))
		default:
			// For more columns, just use the first 4
			h1 := text.New(t.Headers[0], *headerTextProps)
			h2 := ""
			h3 := ""
			h4 := ""
			if len(t.Headers) > 1 {
				h2 = t.Headers[1]
			}
			if len(t.Headers) > 2 {
				h3 = t.Headers[2]
			}
			if len(t.Headers) > 3 {
				h4 = t.Headers[3]
			}
			h2Text := text.New(h2, *headerTextProps)
			h3Text := text.New(h3, *headerTextProps)
			h4Text := text.New(h4, *headerTextProps)
			b.maroto.AddRow(baseHeight,
				col.New(3).Add(h1),
				col.New(3).Add(h2Text),
				col.New(3).Add(h3Text),
				col.New(3).Add(h4Text))
		}
	}

	// Draw data rows
	rowTextProps := b.style.ConvertToTextProps(t.RowStyle)
	
	for _, dataRow := range t.Rows {
		// Create row based on number of columns
		switch numColumns {
		case 1:
			cellText := fmt.Sprintf("%v", dataRow[0])
			cellTextComponent := text.New(cellText, *rowTextProps)
			b.maroto.AddRow(baseHeight, col.New(12).Add(cellTextComponent))
		case 2:
			c1 := fmt.Sprintf("%v", dataRow[0])
			c2 := ""
			if len(dataRow) > 1 {
				c2 = fmt.Sprintf("%v", dataRow[1])
			}
			c1Text := text.New(c1, *rowTextProps)
			c2Text := text.New(c2, *rowTextProps)
			b.maroto.AddRow(baseHeight,
				col.New(6).Add(c1Text),
				col.New(6).Add(c2Text))
		case 3:
			c1 := fmt.Sprintf("%v", dataRow[0])
			c2 := ""
			c3 := ""
			if len(dataRow) > 1 {
				c2 = fmt.Sprintf("%v", dataRow[1])
			}
			if len(dataRow) > 2 {
				c3 = fmt.Sprintf("%v", dataRow[2])
			}
			c1Text := text.New(c1, *rowTextProps)
			c2Text := text.New(c2, *rowTextProps)
			c3Text := text.New(c3, *rowTextProps)
			b.maroto.AddRow(baseHeight,
				col.New(4).Add(c1Text),
				col.New(4).Add(c2Text),
				col.New(4).Add(c3Text))
		default:
			// For more columns, just use the first 4
			c1 := fmt.Sprintf("%v", dataRow[0])
			c2 := ""
			c3 := ""
			c4 := ""
			if len(dataRow) > 1 {
				c2 = fmt.Sprintf("%v", dataRow[1])
			}
			if len(dataRow) > 2 {
				c3 = fmt.Sprintf("%v", dataRow[2])
			}
			if len(dataRow) > 3 {
				c4 = fmt.Sprintf("%v", dataRow[3])
			}
			c1Text := text.New(c1, *rowTextProps)
			c2Text := text.New(c2, *rowTextProps)
			c3Text := text.New(c3, *rowTextProps)
			c4Text := text.New(c4, *rowTextProps)
			b.maroto.AddRow(baseHeight,
				col.New(3).Add(c1Text),
				col.New(3).Add(c2Text),
				col.New(3).Add(c3Text),
				col.New(3).Add(c4Text))
		}
	}

	// Add a spacing row at the bottom
	b.maroto.AddRows(row.New(2))

	return nil
}