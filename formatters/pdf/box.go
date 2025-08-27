package pdf

import (
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
)

type VerticalPosition string
type HorizontalPosition string
type InsidePosition string
type LabelPosition struct {
	Vertical   VerticalPosition
	Horizontal HorizontalPosition
	Inside     InsidePosition
}

const (
	VerticalTop      VerticalPosition   = "top"
	VerticalBottom   VerticalPosition   = "bottom"
	VerticalCenter   VerticalPosition   = "" // center is the default
	HorizontalLeft   HorizontalPosition = "left"
	HorizontalRight  HorizontalPosition = "right"
	HorizontalCenter HorizontalPosition = "" // center is the default
	InsideTop        InsidePosition     = "" // inside is the default
	InsideBottom     InsidePosition     = "outside"
)

type Positionable struct {
	Position *LabelPosition
	// If both position and absolute is provided, absolute is relative to position
	Absolute *api.Position
}

// ParsePosition parses position strings like "center", "top-left", "bottom-right-outside"
func ParsePosition(s string) LabelPosition {
	if s == "" {
		return LabelPosition{} // Default center position
	}

	parts := strings.Split(strings.ToLower(s), "-")
	pos := LabelPosition{}

	for _, part := range parts {
		switch part {
		case "top":
			pos.Vertical = VerticalTop
		case "bottom":
			pos.Vertical = VerticalBottom
		case "center", "middle":
			pos.Vertical = VerticalCenter
		case "left":
			pos.Horizontal = HorizontalLeft
		case "right":
			pos.Horizontal = HorizontalRight
		case "outside":
			pos.Inside = InsideBottom
		case "inside":
			pos.Inside = InsideTop
		}
	}

	return pos
}

type Label struct {
	Positionable
	api.Text
}

type Line struct {
	Positionable
	api.Line
	Labels []Label
}

type Circle struct {
	Positionable
	api.Circle
	Labels []Label
}

type Box struct {
	api.Rectangle
	Borders *api.Borders
	Labels  []Label
	Lines   []Line
}

// Draw implements the Widget interface
func (b Box) Draw(builder *Builder) error {
	// Since Maroto doesn't have a rectangle component, we'll use borders and spacing
	// to simulate a box appearance

	height := float64(b.Rectangle.Height)
	if height == 0 {
		height = 20 // Default box height
	}

	// Add top spacing
	builder.maroto.AddRows(row.New(2))

	// Draw labels if any
	if len(b.Labels) > 0 {
		for _, label := range b.Labels {
			textProps := builder.style.ConvertToTextProps(label.Class)
			textComponent := text.New(label.Content, *textProps)

			// Get horizontal alignment
			var horizontalAlign HorizontalPosition = HorizontalCenter
			if label.Position != nil {
				horizontalAlign = label.Position.Horizontal
			}

			// Create columns based on horizontal alignment
			switch horizontalAlign {
			case HorizontalLeft:
				builder.maroto.AddRow(6,
					col.New(4).Add(textComponent),
					col.New(8)) // Empty space
			case HorizontalRight:
				builder.maroto.AddRow(6,
					col.New(8), // Empty space
					col.New(4).Add(textComponent))
			default: // Center
				builder.maroto.AddRow(6,
					col.New(3), // Empty space
					col.New(6).Add(textComponent),
					col.New(3)) // Empty space
			}
		}
	} else {
		// Add empty space for the box height
		builder.maroto.AddRows(row.New(height))
	}

	// Add bottom spacing
	builder.maroto.AddRows(row.New(2))

	return nil
}
