package pdf

import (
	"strings"

	"github.com/flanksource/clicky/api"
)

// BoxBuilder provides a fluent API for creating Box instances
type BoxBuilder struct {
	rectangle api.Rectangle
	borders   *api.Borders
	labels    []Label
	lines     []Line
}

// NewBoxBuilder creates a new BoxBuilder
func NewBoxBuilder() *BoxBuilder {
	return &BoxBuilder{
		labels: make([]Label, 0),
		lines:  make([]Line, 0),
	}
}

// WithSize sets the width and height of the box
func (b *BoxBuilder) WithSize(width, height int) *BoxBuilder {
	b.rectangle.Width = width
	b.rectangle.Height = height
	return b
}

// WithRectangle sets the rectangle dimensions using an api.Rectangle
func (b *BoxBuilder) WithRectangle(rect api.Rectangle) *BoxBuilder {
	b.rectangle = rect
	return b
}

// WithBorders sets the borders of the box
func (b *BoxBuilder) WithBorders(borders *api.Borders) *BoxBuilder {
	b.borders = borders
	return b
}

// WithBorder sets all borders to the same line style
func (b *BoxBuilder) WithBorder(line api.Line) *BoxBuilder {
	b.borders = &api.Borders{
		Top:    line,
		Right:  line,
		Bottom: line,
		Left:   line,
	}
	return b
}

// WithBorderColor sets all borders to the same color with default width
func (b *BoxBuilder) WithBorderColor(color api.Color) *BoxBuilder {
	line := api.Line{
		Color: color,
		Width: 1.0,
		Style: api.Solid,
	}
	return b.WithBorder(line)
}

// WithLabel adds a label to the box using variadic parameters
// First parameter is the label text
// Additional parameters can be position ("top-left") or styles ("font-bold")
func (b *BoxBuilder) WithLabel(label string, positionOrStyle ...string) *BoxBuilder {
	b.labels = append(b.labels, Label{
		Positionable: Positionable{},
		Text: api.Text{
			Content: label,
			Style:   strings.Join(positionOrStyle, " "),
		},
	})
	return b
}

// AddLabel adds a pre-built Label to the box
func (b *BoxBuilder) AddLabel(label Label) *BoxBuilder {
	b.labels = append(b.labels, label)
	return b
}

// WithLabels sets multiple labels at once
func (b *BoxBuilder) WithLabels(labels []Label) *BoxBuilder {
	b.labels = labels
	return b
}

// AddLine adds a line to the box
func (b *BoxBuilder) AddLine(line Line) *BoxBuilder {
	b.lines = append(b.lines, line)
	return b
}

// WithLines sets multiple lines at once
func (b *BoxBuilder) WithLines(lines []Line) *BoxBuilder {
	b.lines = lines
	return b
}

// Build creates and returns the Box instance
func (b *BoxBuilder) Build() Box {
	return Box{
		Rectangle: b.rectangle,
		Borders:   b.borders,
		Labels:    b.labels,
		Lines:     b.lines,
	}
}

// Helper functions for common box patterns

// NewSimpleBox creates a simple box with dimensions and optional label
func NewSimpleBox(width, height int, label ...string) Box {
	builder := NewBoxBuilder().WithSize(width, height)

	if len(label) > 0 && label[0] != "" {
		builder = builder.WithLabel(label[0], "center")
	}

	return builder.Build()
}

// NewBorderedBox creates a box with borders and optional label
func NewBorderedBox(width, height int, borderColor api.Color, label ...string) Box {
	builder := NewBoxBuilder().
		WithSize(width, height).
		WithBorderColor(borderColor)

	if len(label) > 0 && label[0] != "" {
		builder = builder.WithLabel(label[0], "center")
	}

	return builder.Build()
}

// NewTitledBox creates a box with a title label at the top
func NewTitledBox(width, height int, title string) Box {
	return NewBoxBuilder().
		WithSize(width, height).
		WithLabel(title, "top-center", "font-bold").
		Build()
}

// NewCardBox creates a card-style box with border and title
func NewCardBox(width, height int, title string, borderColor api.Color) Box {
	return NewBoxBuilder().
		WithSize(width, height).
		WithBorderColor(borderColor).
		WithLabel(title, "top-center", "font-bold").
		Build()
}
