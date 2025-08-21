package pdf

import (
	"strings"
	
	"github.com/flanksource/clicky/api"
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
	Line    []Line
}

func (b Box) GetWidth() float64 {
	return float64(b.Rectangle.Width)
}

func (b Box) GetHeight() float64 {
	return float64(b.Rectangle.Height)
}

func (b Box) Draw(builder *Builder, opts ...DrawOptions) error {
	// Parse options
	options := &drawOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Save current state
	savedPos := builder.GetCurrentPosition()
	savedState := builder.GetStyleConverter().SaveCurrentState()

	// Apply position if specified
	if options.Position != (api.Position{}) {
		builder.MoveTo(options.Position)
	}

	pos := builder.GetCurrentPosition()
	pdf := builder.GetPDF()
	style := builder.GetStyleConverter()

	// Get box dimensions
	width := b.GetWidth()
	height := b.GetHeight()

	// Override with options if provided
	if options.Size != (api.Rectangle{}) {
		width = float64(options.Size.Width)
		height = float64(options.Size.Height)
	}

	boxX := float64(pos.X)
	boxY := float64(pos.Y)

	// Draw the main rectangle
	if b.Borders != nil {
		style.DrawBorders(boxX, boxY, width, height, b.Borders)
	} else {
		// Draw simple border
		pdf.SetDrawColor(0, 0, 0) // Black border
		pdf.Rect(boxX, boxY, width, height, "D")
	}

	// Draw labels
	for _, label := range b.Labels {
		b.drawLabel(builder, label, boxX, boxY, width, height)
	}

	// Draw lines
	for _, line := range b.Line {
		b.drawLine(builder, line, boxX, boxY, width, height)
	}

	// Update builder position to below the box
	builder.MoveTo(api.Position{X: pos.X, Y: pos.Y + int(height)})

	// Restore state
	builder.MoveTo(savedPos)
	style.RestoreState(savedState)

	return nil
}

// drawLabel draws a label at the specified position relative to the box
func (b Box) drawLabel(builder *Builder, label Label, boxX, boxY, boxWidth, boxHeight float64) {
	pdf := builder.GetPDF()
	style := builder.GetStyleConverter()

	// Save current state
	savedState := style.SaveCurrentState()

	// Apply label styling
	if label.Class != (api.Class{}) {
		style.ApplyClassToPDF(label.Class)
	} else if label.Style != "" {
		style.ParseTailwindToPDF(label.Style)
	}

	// Calculate text dimensions
	textWidth := style.GetTextWidth(label.Content)
	textHeight := style.GetTextHeight()

	// Calculate position based on LabelPosition
	var textX, textY float64

	// Horizontal positioning
	switch label.Position.Horizontal {
	case HorizontalLeft:
		textX = boxX
	case HorizontalRight:
		textX = boxX + boxWidth - textWidth
	default: // HorizontalCenter
		textX = boxX + (boxWidth-textWidth)/2
	}

	// Vertical positioning
	switch label.Position.Vertical {
	case VerticalTop:
		if label.Position.Inside == InsideBottom { // outside top
			textY = boxY - textHeight - 2
		} else { // inside top
			textY = boxY + 2
		}
	case VerticalBottom:
		if label.Position.Inside == InsideBottom { // outside bottom
			textY = boxY + boxHeight + 2
		} else { // inside bottom
			textY = boxY + boxHeight - textHeight - 2
		}
	default: // VerticalCenter
		textY = boxY + (boxHeight-textHeight)/2
	}

	// Apply absolute positioning offset if specified
	if label.Absolute != nil {
		textX += float64(label.Absolute.X)
		textY += float64(label.Absolute.Y)
	}

	// Draw the label text
	pdf.SetXY(textX, textY)
	pdf.Cell(textWidth, textHeight, label.Content)

	// Restore state
	style.RestoreState(savedState)
}

// drawLine draws a line relative to the box
func (b Box) drawLine(builder *Builder, line Line, boxX, boxY, boxWidth, boxHeight float64) {
	pdf := builder.GetPDF()
	style := builder.GetStyleConverter()

	// Save current state
	savedState := style.SaveCurrentState()

	// Apply line color and width
	if line.Color.Hex != "" {
		r, g, b := style.hexToRGB(line.Color.Hex)
		pdf.SetDrawColor(r, g, b)
	}
	if line.Width > 0 {
		pdf.SetLineWidth(line.Width)
	}

	// Calculate line positions - for now, draw a simple line from top-left to bottom-right
	// In a more sophisticated implementation, you would parse Position to determine exact coordinates
	startX, startY := boxX, boxY
	endX, endY := boxX+boxWidth, boxY+boxHeight

	// Apply position adjustments if specified
	if line.Absolute != nil {
		startX += float64(line.Absolute.X)
		startY += float64(line.Absolute.Y)
	}

	// Draw the line
	pdf.Line(startX, startY, endX, endY)

	// Draw line labels
	for _, label := range line.Labels {
		// Position label at midpoint of line
		midX := (startX + endX) / 2
		midY := (startY + endY) / 2
		
		b.drawLabelAtPosition(builder, label, midX, midY)
	}

	// Restore state
	style.RestoreState(savedState)
}

// drawLabelAtPosition draws a label at a specific coordinate
func (b Box) drawLabelAtPosition(builder *Builder, label Label, x, y float64) {
	style := builder.GetStyleConverter()
	pdf := builder.GetPDF()

	// Save state
	savedState := style.SaveCurrentState()

	// Apply styling
	if label.Class != (api.Class{}) {
		style.ApplyClassToPDF(label.Class)
	} else if label.Style != "" {
		style.ParseTailwindToPDF(label.Style)
	}

	// Calculate text dimensions and adjust position
	textWidth := style.GetTextWidth(label.Content)
	textHeight := style.GetTextHeight()

	// Center text at the specified position
	textX := x - textWidth/2
	textY := y - textHeight/2

	// Apply absolute offset if specified
	if label.Absolute != nil {
		textX += float64(label.Absolute.X)
		textY += float64(label.Absolute.Y)
	}

	// Draw text
	pdf.SetXY(textX, textY)
	pdf.Cell(textWidth, textHeight, label.Content)

	// Restore state
	style.RestoreState(savedState)
}
