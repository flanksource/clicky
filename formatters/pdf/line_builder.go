package pdf

import (
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api"
)

// LineBuilder provides a fluent API for creating Line instances
type LineBuilder struct {
	position *LabelPosition
	absolute *api.Position
	line     api.Line
	labels   []Label
}

// NewLineBuilder creates a new LineBuilder
func NewLineBuilder() *LineBuilder {
	return &LineBuilder{
		line: api.Line{
			Width: 1.0,
			Style: api.Solid,
			Color: api.Color{Hex: "#000000"},
		},
		labels: make([]Label, 0),
	}
}

// WithPosition sets the position using a string like "top-left", "center", "bottom-right"
func (b *LineBuilder) WithPosition(position string) *LineBuilder {
	pos := ParsePosition(position)
	b.position = &pos
	return b
}

// WithAbsolute sets absolute positioning coordinates
func (b *LineBuilder) WithAbsolute(x, y int) *LineBuilder {
	b.absolute = &api.Position{X: x, Y: y}
	return b
}

// WithStyles sets line styles using variadic string parameters
// Supports color names, hex colors, width values, and style names
func (b *LineBuilder) WithStyles(styles ...string) *LineBuilder {
	for _, style := range styles {
		b.parseAndApplyStyle(style)
	}
	return b
}

// WithColor sets the line color
func (b *LineBuilder) WithColor(color api.Color) *LineBuilder {
	b.line.Color = color
	return b
}

// WithWidth sets the line width
func (b *LineBuilder) WithWidth(width float64) *LineBuilder {
	b.line.Width = width
	return b
}

// WithStyle sets the line style (solid, dashed, dotted)
func (b *LineBuilder) WithStyle(style string) *LineBuilder {
	switch strings.ToLower(style) {
	case "solid":
		b.line.Style = api.Solid
	case "dashed":
		b.line.Style = api.Dashed
	case "dotted":
		b.line.Style = api.Dotted
	}
	return b
}

// WithLine sets the line properties directly using api.Line
func (b *LineBuilder) WithLine(line api.Line) *LineBuilder {
	b.line = line
	return b
}

// WithLabel adds a label to the line using variadic parameters
func (b *LineBuilder) WithLabel(label string, positionOrStyle ...string) *LineBuilder {
	labelBuilder := NewLabelBuilder(label)

	if len(positionOrStyle) > 0 {
		// First parameter: check if it's a position or style
		first := positionOrStyle[0]
		if isPositionString(first) {
			labelBuilder = labelBuilder.WithPosition(first)
			// Apply remaining as styles
			if len(positionOrStyle) > 1 {
				labelBuilder = labelBuilder.WithStyles(positionOrStyle[1:]...)
			}
		} else {
			// All parameters are styles
			labelBuilder = labelBuilder.WithStyles(positionOrStyle...)
		}
	}

	b.labels = append(b.labels, labelBuilder.Build())
	return b
}

// AddLabel adds a pre-built Label to the line
func (b *LineBuilder) AddLabel(label Label) *LineBuilder {
	b.labels = append(b.labels, label)
	return b
}

// Build creates and returns the Line instance
func (b *LineBuilder) Build() Line {
	return Line{
		Positionable: Positionable{
			Position: b.position,
			Absolute: b.absolute,
		},
		Line:   b.line,
		Labels: b.labels,
	}
}

// parseAndApplyStyle parses a style string and applies it to the line
func (b *LineBuilder) parseAndApplyStyle(style string) {
	style = strings.TrimSpace(strings.ToLower(style))

	// Try to parse as hex color
	if strings.HasPrefix(style, "#") && len(style) == 7 {
		b.line.Color = api.Color{Hex: style}
		return
	}

	// Try to parse as width (number followed by optional unit)
	if width, err := strconv.ParseFloat(style, 64); err == nil {
		b.line.Width = width
		return
	}

	// Parse width with units (e.g., "2px", "1.5")
	if strings.HasSuffix(style, "px") {
		if width, err := strconv.ParseFloat(strings.TrimSuffix(style, "px"), 64); err == nil {
			b.line.Width = width
			return
		}
	}

	// Parse named colors
	switch style {
	case "black":
		b.line.Color = api.Color{Hex: "#000000"}
	case "white":
		b.line.Color = api.Color{Hex: "#FFFFFF"}
	case "red":
		b.line.Color = api.Color{Hex: "#FF0000"}
	case "green":
		b.line.Color = api.Color{Hex: "#00FF00"}
	case "blue":
		b.line.Color = api.Color{Hex: "#0000FF"}
	case "gray", "grey":
		b.line.Color = api.Color{Hex: "#808080"}
	case "lightgray", "lightgrey":
		b.line.Color = api.Color{Hex: "#D3D3D3"}
	case "darkgray", "darkgrey":
		b.line.Color = api.Color{Hex: "#A9A9A9"}
	}

	// Parse line styles
	switch style {
	case "solid":
		b.line.Style = api.Solid
	case "dashed":
		b.line.Style = api.Dashed
	case "dotted":
		b.line.Style = api.Dotted
	}
}

// Helper functions for common line patterns

// NewHorizontalLine creates a horizontal line with optional styling
func NewHorizontalLine(styles ...string) Line {
	builder := NewLineBuilder().WithPosition("center")
	if len(styles) > 0 {
		builder = builder.WithStyles(styles...)
	}
	return builder.Build()
}

// NewVerticalLine creates a vertical line with optional styling
func NewVerticalLine(styles ...string) Line {
	builder := NewLineBuilder().WithPosition("center")
	if len(styles) > 0 {
		builder = builder.WithStyles(styles...)
	}
	return builder.Build()
}

// NewBorderLine creates a line suitable for borders with color and width
func NewBorderLine(color api.Color, width float64) Line {
	return NewLineBuilder().
		WithColor(color).
		WithWidth(width).
		Build()
}

// NewDashedLine creates a dashed line with specified color
func NewDashedLine(color api.Color) Line {
	return NewLineBuilder().
		WithColor(color).
		WithStyle("dashed").
		Build()
}

// NewStyledLine creates a line with styles parsed from strings
func NewStyledLine(styles ...string) Line {
	return NewLineBuilder().WithStyles(styles...).Build()
}
