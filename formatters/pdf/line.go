package pdf

import (
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/consts/linestyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"github.com/flanksource/clicky/api"
)

// LineWidget represents a line widget
type LineWidget struct {
	Orientation   string    `json:"orientation,omitempty"` // horizontal or vertical
	Style         string    `json:"style,omitempty"`       // solid, dashed, dotted
	Color         api.Color `json:"color,omitempty"`
	Thickness     float64   `json:"thickness,omitempty"`
	Length        float64   `json:"length,omitempty"`         // Percentage of available space (0-100)
	Offset        float64   `json:"offset,omitempty"`         // Offset from start (0-100)
	TailwindClass string    `json:"tailwind_class,omitempty"` // Tailwind classes
	ColumnSpan    int       `json:"column_span,omitempty"`    // How many columns to span (1-12)
}

// Draw implements the Widget interface
func (l LineWidget) Draw(b *Builder) error {
	// Apply Tailwind classes if provided
	if l.TailwindClass != "" {
		resolvedClass := api.ResolveStyles(l.TailwindClass)
		// Use foreground color from resolved styles if available
		if resolvedClass.Foreground != nil && l.Color.Hex == "" {
			l.Color = *resolvedClass.Foreground
		}
	}

	// Set defaults
	if l.Thickness == 0 {
		l.Thickness = 0.5
	}
	if l.Length == 0 {
		l.Length = 100
	}
	if l.ColumnSpan == 0 || l.ColumnSpan > 12 {
		l.ColumnSpan = 12
	}

	// Convert to Maroto props
	lineProps := props.Line{
		Thickness:     l.Thickness,
		SizePercent:   l.Length,
		OffsetPercent: l.Offset,
	}

	// Set orientation
	if l.Orientation == "vertical" {
		lineProps.Orientation = orientation.Vertical
	} else {
		lineProps.Orientation = orientation.Horizontal
	}

	// Set style
	switch l.Style {
	case "dashed":
		lineProps.Style = linestyle.Dashed
	default:
		lineProps.Style = linestyle.Solid
	}

	// Set color
	if l.Color.Hex != "" {
		lineProps.Color = b.style.ConvertColor(l.Color)
	}

	// Add the line to the PDF
	b.maroto.AddRow(1, col.New(l.ColumnSpan).Add(line.New(lineProps)))

	return nil
}
