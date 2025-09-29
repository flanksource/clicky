package pdf

import (
	"fmt"

	"github.com/flanksource/maroto/v2/pkg/components/row"

	"github.com/flanksource/clicky/api"
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
	if g.Padding.Top.Float64() > 0 {
		topPadding := g.Padding.Top.ToMM()
		b.maroto.AddRows(row.New(topPadding))
	}

	// TODO: Implement proper grid layout using Maroto's 12-column system
	// For now, widgets are drawn sequentially

	// Process items and draw widgets
	for _, item := range g.Items {
		// Draw the widget if present
		if item.Widget != nil {
			if err := item.Widget.Draw(b); err != nil {
				return fmt.Errorf("failed to draw widget: %w", err)
			}
		}
	}

	// Apply bottom padding if specified
	if g.Padding.Bottom.Float64() > 0 {
		bottomPadding := g.Padding.Bottom.ToMM()
		b.maroto.AddRows(row.New(bottomPadding))
	}

	return nil
}
