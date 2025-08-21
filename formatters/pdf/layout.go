package pdf

import "github.com/flanksource/clicky/api"

type GridItem struct {
	Widget  Widget
	RowSpan int
	ColSpan int
}
type GridLayout struct {
	Padding api.Padding
	Columns int
	Items   []GridItem
}

func (g GridLayout) GetWidth() float64 {
	if g.Columns <= 0 || len(g.Items) == 0 {
		return 0
	}

	// Calculate available width (page width minus margins and padding)
	pageWidth := 190.0 // A4 width minus typical margins
	
	// Subtract horizontal padding
	paddingWidth := g.Padding.Left + g.Padding.Right
	
	return pageWidth - (paddingWidth * 4.2333) // Convert rem to mm
}

func (g GridLayout) GetHeight() float64 {
	if g.Columns <= 0 || len(g.Items) == 0 {
		return 0
	}

	// Calculate total rows needed
	rows := (len(g.Items) + g.Columns - 1) / g.Columns

	// Calculate height per row (this is a simplified calculation)
	// In a real implementation, you'd calculate based on actual widget heights
	defaultRowHeight := 20.0 // mm
	
	totalHeight := float64(rows) * defaultRowHeight
	
	// Add vertical padding
	paddingHeight := (g.Padding.Top + g.Padding.Bottom) * 4.2333 // Convert rem to mm
	
	return totalHeight + paddingHeight
}

func (g GridLayout) Draw(b *Builder, opts ...DrawOptions) error {
	if g.Columns <= 0 || len(g.Items) == 0 {
		return nil
	}

	// Parse options
	options := &drawOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Save current state
	_ = b.GetCurrentPosition() // We'll use startPos instead

	// Apply position if specified
	if options.Position != (api.Position{}) {
		b.MoveTo(options.Position)
	}

	// Calculate layout parameters
	totalWidth := g.GetWidth()
	columnWidth := totalWidth / float64(g.Columns)
	
	// Apply padding
	leftPadding := g.Padding.Left * 4.2333   // Convert rem to mm
	topPadding := g.Padding.Top * 4.2333
	
	startPos := b.GetCurrentPosition()
	gridX := float64(startPos.X) + leftPadding
	gridY := float64(startPos.Y) + topPadding

	currentRow := 0
	currentCol := 0
	maxRowHeight := 0.0

	// Draw each grid item
	for i, item := range g.Items {
		// Calculate position for this item
		itemX := gridX + float64(currentCol)*columnWidth
		itemY := gridY + float64(currentRow)*maxRowHeight

		// Set position for the widget
		b.MoveTo(api.Position{X: int(itemX), Y: int(itemY)})

		// Calculate widget size considering span
		widgetWidth := columnWidth * float64(item.ColSpan)
		if item.ColSpan <= 0 {
			item.ColSpan = 1 // Default column span
		}
		
		// Draw the widget
		if item.Widget != nil {
			err := item.Widget.Draw(b, WithSize(widgetWidth, 0))
			if err != nil {
				return err
			}
			
			// Update max row height
			widgetHeight := item.Widget.GetHeight()
			if item.RowSpan > 1 {
				widgetHeight *= float64(item.RowSpan)
			}
			if widgetHeight > maxRowHeight {
				maxRowHeight = widgetHeight
			}
		}

		// Update grid position
		currentCol += item.ColSpan
		if currentCol >= g.Columns {
			currentCol = 0
			currentRow++
			if i < len(g.Items)-1 { // Not the last item
				maxRowHeight = 20.0 // Reset for next row, use default if no widgets set it
			}
		}
	}

	// Update builder position to end of grid
	totalHeight := float64(currentRow+1)*maxRowHeight + topPadding + g.Padding.Bottom*4.2333
	b.MoveTo(api.Position{X: startPos.X, Y: startPos.Y + int(totalHeight)})

	return nil
}
