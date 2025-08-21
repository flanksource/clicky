package pdf

import (
	"github.com/flanksource/clicky/api"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
)

// GridItem represents an item in the grid layout
type GridItem struct {
	Widget  Widget
	RowSpan int
	ColSpan int
}

// GridLayout implements a grid-based layout using Maroto's row/column system
type GridLayout struct {
	Padding api.Padding
	Columns int
	Items   []GridItem
}

// Draw implements the Widget interface
func (g GridLayout) Draw(b *Builder) error {
	if g.Columns <= 0 || len(g.Items) == 0 {
		return nil
	}

	// Apply top padding if specified
	if g.Padding.Top > 0 {
		topPadding := g.Padding.Top * 4 // Convert rem to mm
		b.maroto.AddRows(row.New(topPadding))
	}

	// Maroto uses a 12-column grid system
	// Calculate how many Maroto columns each logical column should use
	marotoColsPerLogicalCol := 12 / g.Columns
	if marotoColsPerLogicalCol < 1 {
		marotoColsPerLogicalCol = 1
	}

	// Process items and draw widgets
	for _, item := range g.Items {
		// Draw the widget if present
		if item.Widget != nil {
			item.Widget.Draw(b)
		}
	}

	// Apply bottom padding if specified
	if g.Padding.Bottom > 0 {
		bottomPadding := g.Padding.Bottom * 4 // Convert rem to mm
		b.maroto.AddRows(row.New(bottomPadding))
	}

	return nil
}

// drawRow draws a single row of grid items (simplified - just draws widgets sequentially)
func (g GridLayout) drawRow(b *Builder, items []GridItem) {
	// For simplicity, just draw each widget
	for _, item := range items {
		if item.Widget != nil {
			item.Widget.Draw(b)
		}
	}
}