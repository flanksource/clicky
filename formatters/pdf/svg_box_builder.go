package pdf

import (
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/tailwind"
)

// SVGBoxBuilder provides a fluent API for creating SVGBox instances
type SVGBoxBuilder struct {
	box                      api.Box
	labels                   []Label
	lines                    []SVGLine
	circles                  []CircleShape
	cuts                     []Cut
	edgeCuts                 []EdgeCut
	measureLines             []MeasureLine
	childBoxes               []ChildBox
	showDimensions           bool
	dimensionUnit            string
	actualWidth              float64
	actualHeight             float64
	svgPadding               float64
	enableCollisionAvoidance bool
	yAxisUp                  bool
}

// NewSVGBoxBuilder creates a new SVGBoxBuilder with sensible defaults
func NewSVGBoxBuilder() *SVGBoxBuilder {
	return &SVGBoxBuilder{
		box: api.Box{
			Rectangle: api.Rectangle{Width: 200, Height: 200},
			Fill:      api.Color{Hex: "#FFFFFF"},
			Border:    api.Borders{},
			Padding:   api.Padding{},
		},
		labels:                   make([]Label, 0),
		lines:                    make([]SVGLine, 0),
		circles:                  make([]CircleShape, 0),
		cuts:                     make([]Cut, 0),
		edgeCuts:                 make([]EdgeCut, 0),
		measureLines:             make([]MeasureLine, 0),
		childBoxes:               make([]ChildBox, 0),
		dimensionUnit:            "mm",
		svgPadding:               0,
		enableCollisionAvoidance: false,
		yAxisUp:                  false,
	}
}

// WithSize sets the width and height of the SVG box
func (b *SVGBoxBuilder) WithSize(width, height int) *SVGBoxBuilder {
	b.box.Width = width
	b.box.Height = height
	return b
}

// WithActualSize sets the actual physical dimensions (for dimension display)
func (b *SVGBoxBuilder) WithActualSize(width, height float64) *SVGBoxBuilder {
	b.actualWidth = width
	b.actualHeight = height
	return b
}

// WithFill sets the fill color of the SVG box
func (b *SVGBoxBuilder) WithFill(color api.Color) *SVGBoxBuilder {
	b.box.Fill = color
	return b
}

// WithStyle applies Tailwind-style classes to the SVG box, including border parsing
func (b *SVGBoxBuilder) WithStyle(styles ...string) *SVGBoxBuilder {
	// Parse borders from style classes
	if len(styles) > 0 {
		borders := tailwind.ParseBorders(styles...)

		// Convert tailwind border types to api types
		b.box.Border = api.Borders{
			Top:    api.Line{Width: borders.Top.Width, Style: api.LineStyle(borders.Top.Style), Color: api.Color{Hex: borders.Top.Color.Hex, Opacity: borders.Top.Color.Opacity}},
			Right:  api.Line{Width: borders.Right.Width, Style: api.LineStyle(borders.Right.Style), Color: api.Color{Hex: borders.Right.Color.Hex, Opacity: borders.Right.Color.Opacity}},
			Bottom: api.Line{Width: borders.Bottom.Width, Style: api.LineStyle(borders.Bottom.Style), Color: api.Color{Hex: borders.Bottom.Color.Hex, Opacity: borders.Bottom.Color.Opacity}},
			Left:   api.Line{Width: borders.Left.Width, Style: api.LineStyle(borders.Left.Style), Color: api.Color{Hex: borders.Left.Color.Hex, Opacity: borders.Left.Color.Opacity}},
		}
	}
	return b
}

// WithPadding sets the padding of the SVG box
func (b *SVGBoxBuilder) WithPadding(padding api.Padding) *SVGBoxBuilder {
	b.box.Padding = padding
	return b
}

// WithSVGPadding sets the SVG canvas padding
func (b *SVGBoxBuilder) WithSVGPadding(padding float64) *SVGBoxBuilder {
	b.svgPadding = padding
	return b
}

// WithLabel adds a label to the SVG box using variadic parameters
func (b *SVGBoxBuilder) WithLabel(label string, positionOrStyle ...string) *SVGBoxBuilder {
	labelBuilder := NewLabelBuilder(label)

	if len(positionOrStyle) > 0 {
		first := positionOrStyle[0]
		if isPositionString(first) {
			labelBuilder = labelBuilder.WithPosition(first)
			if len(positionOrStyle) > 1 {
				labelBuilder = labelBuilder.WithStyles(positionOrStyle[1:]...)
			}
		} else {
			labelBuilder = labelBuilder.WithStyles(positionOrStyle...)
		}
	}

	b.labels = append(b.labels, labelBuilder.Build())
	return b
}

// AddLabel adds a pre-built Label to the SVG box
func (b *SVGBoxBuilder) AddLabel(label Label) *SVGBoxBuilder {
	b.labels = append(b.labels, label)
	return b
}

// AddLine adds a line to the SVG box with coordinates and optional styles
func (b *SVGBoxBuilder) AddLine(x1, y1, x2, y2 float64, styles ...string) *SVGBoxBuilder {
	line := SVGLine{
		X1: x1,
		Y1: y1,
		X2: x2,
		Y2: y2,
		Line: api.Line{
			Color: api.Color{Hex: "#000000"},
			Width: 1.0,
			Style: api.Solid,
		},
	}

	// Parse styles
	for _, style := range styles {
		b.parseLineStyle(&line.Line, style)
	}

	b.lines = append(b.lines, line)
	return b
}

// AddSVGLine adds a pre-built SVGLine to the SVG box
func (b *SVGBoxBuilder) AddSVGLine(line SVGLine) *SVGBoxBuilder {
	b.lines = append(b.lines, line)
	return b
}

// AddCircle adds a circle to the SVG box with optional label
func (b *SVGBoxBuilder) AddCircle(x, y, diameter float64, label ...string) *SVGBoxBuilder {
	circle := CircleShape{
		X:        x,
		Y:        y,
		Diameter: diameter,
	}

	if len(label) > 0 {
		circle.Label = label[0]
	}

	b.circles = append(b.circles, circle)
	return b
}

// AddCircleShape adds a pre-built CircleShape to the SVG box
func (b *SVGBoxBuilder) AddCircleShape(circle CircleShape) *SVGBoxBuilder {
	b.circles = append(b.circles, circle)
	return b
}

// AddCut adds a cut to the SVG box with optional label
func (b *SVGBoxBuilder) AddCut(orientation string, position, width, depth float64, label ...string) *SVGBoxBuilder {
	cut := Cut{
		Orientation: orientation,
		Position:    position,
		Width:       width,
		Depth:       depth,
	}

	if len(label) > 0 {
		cut.Label = label[0]
	}

	b.cuts = append(b.cuts, cut)
	return b
}

// AddEdgeCut adds an edge cut to the SVG box with optional label
func (b *SVGBoxBuilder) AddEdgeCut(edge string, width, depth float64, label ...string) *SVGBoxBuilder {
	edgeCut := EdgeCut{
		Edge:  edge,
		Width: width,
		Depth: depth,
	}

	if len(label) > 0 {
		edgeCut.Label = label[0]
	}

	b.edgeCuts = append(b.edgeCuts, edgeCut)
	return b
}

// AddMeasureLine adds a measure line to the SVG box
func (b *SVGBoxBuilder) AddMeasureLine(x1, y1, x2, y2 float64, label string, options ...string) *SVGBoxBuilder {
	measureLine := MeasureLine{
		X1:         x1,
		Y1:         y1,
		X2:         x2,
		Y2:         y2,
		Label:      label,
		ShowArrows: true,
		Style:      "solid",
	}

	// Parse options
	for _, option := range options {
		switch strings.ToLower(option) {
		case "dashed":
			measureLine.Style = "dashed"
		case "no-arrows":
			measureLine.ShowArrows = false
		default:
			// Try to parse as offset
			if offset, err := strconv.ParseFloat(option, 64); err == nil {
				measureLine.Offset = offset
			}
		}
	}

	b.measureLines = append(b.measureLines, measureLine)
	return b
}

// AddChildBox adds a child box at the specified position
func (b *SVGBoxBuilder) AddChildBox(child ChildBox) *SVGBoxBuilder {
	b.childBoxes = append(b.childBoxes, child)
	return b
}

// AddChildBoxAt adds a child SVGBox at the specified coordinates
func (b *SVGBoxBuilder) AddChildBoxAt(x, y float64, childBox SVGBox) *SVGBoxBuilder {
	b.childBoxes = append(b.childBoxes, ChildBox{
		Box: childBox,
		X:   x,
		Y:   y,
	})
	return b
}

// WithShowDimensions enables/disables dimension display with optional unit
func (b *SVGBoxBuilder) WithShowDimensions(show bool, unit ...string) *SVGBoxBuilder {
	b.showDimensions = show
	if len(unit) > 0 {
		b.dimensionUnit = unit[0]
	}
	return b
}

// WithCollisionAvoidance enables/disables automatic label collision avoidance
func (b *SVGBoxBuilder) WithCollisionAvoidance(enable bool) *SVGBoxBuilder {
	b.enableCollisionAvoidance = enable
	return b
}

// WithYAxisUp sets the Y-axis direction (true for bottom-up like CAD, false for top-down like SVG)
func (b *SVGBoxBuilder) WithYAxisUp(up bool) *SVGBoxBuilder {
	b.yAxisUp = up
	return b
}

// Build creates and returns the SVGBox instance
func (b *SVGBoxBuilder) Build() SVGBox {
	return SVGBox{
		Box:                      b.box,
		Labels:                   b.labels,
		Lines:                    b.lines,
		Circles:                  b.circles,
		Cuts:                     b.cuts,
		EdgeCuts:                 b.edgeCuts,
		MeasureLines:             b.measureLines,
		ChildBoxes:               b.childBoxes,
		ShowDimensions:           b.showDimensions,
		DimensionUnit:            b.dimensionUnit,
		ActualWidth:              b.actualWidth,
		ActualHeight:             b.actualHeight,
		SVGPadding:               b.svgPadding,
		EnableCollisionAvoidance: b.enableCollisionAvoidance,
		YAxisUp:                  b.yAxisUp,
	}
}

// parseLineStyle parses a style string and applies it to an api.Line
func (b *SVGBoxBuilder) parseLineStyle(line *api.Line, style string) {
	style = strings.TrimSpace(strings.ToLower(style))

	// Parse hex colors
	if strings.HasPrefix(style, "#") && len(style) == 7 {
		line.Color = api.Color{Hex: style}
		return
	}

	// Parse width values
	if width, err := strconv.ParseFloat(style, 64); err == nil && width > 0 {
		line.Width = width
		return
	}

	// Parse named colors
	switch style {
	case "black":
		line.Color = api.Color{Hex: "#000000"}
	case "white":
		line.Color = api.Color{Hex: "#FFFFFF"}
	case "red":
		line.Color = api.Color{Hex: "#FF0000"}
	case "green":
		line.Color = api.Color{Hex: "#00FF00"}
	case "blue":
		line.Color = api.Color{Hex: "#0000FF"}
	case "gray", "grey":
		line.Color = api.Color{Hex: "#808080"}
	case "lightgray", "lightgrey":
		line.Color = api.Color{Hex: "#D3D3D3"}
	case "darkgray", "darkgrey":
		line.Color = api.Color{Hex: "#A9A9A9"}
	}

	// Parse line styles
	switch style {
	case "solid":
		line.Style = api.Solid
	case "dashed":
		line.Style = api.Dashed
	case "dotted":
		line.Style = api.Dotted
	}
}

// Helper functions for common SVG box patterns

// NewSimpleSVGBox creates a simple SVG box with dimensions and optional label
func NewSimpleSVGBox(width, height int, label ...string) SVGBox {
	builder := NewSVGBoxBuilder().WithSize(width, height)

	if len(label) > 0 && label[0] != "" {
		builder = builder.WithLabel(label[0], "center")
	}

	return builder.Build()
}

// NewWoodworkingBox creates an SVG box suitable for woodworking diagrams
func NewWoodworkingBox(width, height int, actualWidth, actualHeight float64) SVGBox {
	return NewSVGBoxBuilder().
		WithSize(width, height).
		WithActualSize(actualWidth, actualHeight).
		WithFill(api.Color{Hex: "#DEB887"}). // BurlyWood color
		WithShowDimensions(true, "mm").
		WithCollisionAvoidance(true).
		WithYAxisUp(true). // CAD-style Y-axis
		Build()
}

// NewTechnicalDrawing creates an SVG box for technical drawings
func NewTechnicalDrawing(width, height int) SVGBox {
	return NewSVGBoxBuilder().
		WithSize(width, height).
		WithFill(api.Color{Hex: "#FFFFFF"}).
		WithShowDimensions(true).
		WithCollisionAvoidance(true).
		Build()
}

// NewLayoutBox creates an SVG box for layout diagrams with child boxes
func NewLayoutBox(width, height int, title string) SVGBox {
	return NewSVGBoxBuilder().
		WithSize(width, height).
		WithFill(api.Color{Hex: "#F5F5F5"}).
		WithLabel(title, "top-center", "font-bold").
		Build()
}
