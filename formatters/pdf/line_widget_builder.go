package pdf

import (
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api"
)

// LineWidgetBuilder provides a fluent API for creating LineWidget instances
type LineWidgetBuilder struct {
	orientation   string
	style         string
	color         api.Color
	thickness     float64
	length        float64
	offset        float64
	tailwindClass string
	columnSpan    int
}

// NewLineWidgetBuilder creates a new LineWidgetBuilder with sensible defaults
func NewLineWidgetBuilder() *LineWidgetBuilder {
	return &LineWidgetBuilder{
		orientation: "horizontal",
		style:       "solid",
		color:       api.Color{Hex: "#000000"},
		thickness:   0.5,
		length:      100.0,
		offset:      0.0,
		columnSpan:  12,
	}
}

// WithOrientation sets the line orientation ("horizontal" or "vertical")
func (b *LineWidgetBuilder) WithOrientation(orientation string) *LineWidgetBuilder {
	b.orientation = strings.ToLower(orientation)
	return b
}

// WithStyle sets the line style ("solid", "dashed", "dotted")
func (b *LineWidgetBuilder) WithStyle(style string) *LineWidgetBuilder {
	b.style = strings.ToLower(style)
	return b
}

// WithColor sets the line color
func (b *LineWidgetBuilder) WithColor(color api.Color) *LineWidgetBuilder {
	b.color = color
	return b
}

// WithThickness sets the line thickness
func (b *LineWidgetBuilder) WithThickness(thickness float64) *LineWidgetBuilder {
	b.thickness = thickness
	return b
}

// WithLength sets the line length as percentage (0-100)
func (b *LineWidgetBuilder) WithLength(length float64) *LineWidgetBuilder {
	b.length = length
	return b
}

// WithOffset sets the line offset as percentage (0-100)
func (b *LineWidgetBuilder) WithOffset(offset float64) *LineWidgetBuilder {
	b.offset = offset
	return b
}

// WithColumnSpan sets how many columns the line should span (1-12)
func (b *LineWidgetBuilder) WithColumnSpan(span int) *LineWidgetBuilder {
	if span < 1 {
		span = 1
	} else if span > 12 {
		span = 12
	}
	b.columnSpan = span
	return b
}

// WithStyles sets styling using variadic string parameters
// Supports Tailwind classes, colors, thickness values, orientations, styles
func (b *LineWidgetBuilder) WithStyles(styles ...string) *LineWidgetBuilder {
	var tailwindClasses []string

	for _, style := range styles {
		if b.parseAndApplyStyle(style) {
			continue // Style was parsed and applied
		}
		// If not parsed as a direct style, treat as Tailwind class
		tailwindClasses = append(tailwindClasses, style)
	}

	// Combine Tailwind classes
	if len(tailwindClasses) > 0 {
		if b.tailwindClass != "" {
			b.tailwindClass += " "
		}
		b.tailwindClass += strings.Join(tailwindClasses, " ")
	}

	return b
}

// Build creates and returns the LineWidget instance
func (b *LineWidgetBuilder) Build() LineWidget {
	return LineWidget{
		Orientation:   b.orientation,
		Style:         b.style,
		Color:         b.color,
		Thickness:     b.thickness,
		Length:        b.length,
		Offset:        b.offset,
		TailwindClass: b.tailwindClass,
		ColumnSpan:    b.columnSpan,
	}
}

// parseAndApplyStyle parses a style string and applies it directly if recognized
// Returns true if the style was parsed and applied, false otherwise
func (b *LineWidgetBuilder) parseAndApplyStyle(style string) bool {
	style = strings.TrimSpace(strings.ToLower(style))

	// Parse hex colors
	if strings.HasPrefix(style, "#") && len(style) == 7 {
		b.color = api.Color{Hex: style}
		return true
	}

	// Parse thickness values
	if thickness, err := strconv.ParseFloat(style, 64); err == nil && thickness > 0 {
		b.thickness = thickness
		return true
	}

	// Parse thickness with units
	if strings.HasSuffix(style, "px") {
		if thickness, err := strconv.ParseFloat(strings.TrimSuffix(style, "px"), 64); err == nil {
			b.thickness = thickness
			return true
		}
	}

	// Parse named colors
	switch style {
	case "black":
		b.color = api.Color{Hex: "#000000"}
		return true
	case "white":
		b.color = api.Color{Hex: "#FFFFFF"}
		return true
	case "red":
		b.color = api.Color{Hex: "#FF0000"}
		return true
	case "green":
		b.color = api.Color{Hex: "#00FF00"}
		return true
	case "blue":
		b.color = api.Color{Hex: "#0000FF"}
		return true
	case "gray", "grey":
		b.color = api.Color{Hex: "#808080"}
		return true
	case "lightgray", "lightgrey":
		b.color = api.Color{Hex: "#D3D3D3"}
		return true
	case "darkgray", "darkgrey":
		b.color = api.Color{Hex: "#A9A9A9"}
		return true
	}

	// Parse orientations
	switch style {
	case "horizontal", "h":
		b.orientation = "horizontal"
		return true
	case "vertical", "v":
		b.orientation = "vertical"
		return true
	}

	// Parse line styles
	switch style {
	case "solid":
		b.style = "solid"
		return true
	case "dashed":
		b.style = "dashed"
		return true
	case "dotted":
		b.style = "dotted"
		return true
	}

	// Parse percentages for length/offset
	if strings.HasSuffix(style, "%") {
		if value, err := strconv.ParseFloat(strings.TrimSuffix(style, "%"), 64); err == nil {
			// Heuristic: larger values are likely length, smaller ones offset
			if value > 50 {
				b.length = value
			} else {
				b.offset = value
			}
			return true
		}
	}

	return false // Style not recognized
}

// Helper functions for common line widget patterns

// NewHorizontalLineWidget creates a horizontal line widget
func NewHorizontalLineWidget(styles ...string) LineWidget {
	builder := NewLineWidgetBuilder().WithOrientation("horizontal")
	if len(styles) > 0 {
		builder = builder.WithStyles(styles...)
	}
	return builder.Build()
}

// NewVerticalLineWidget creates a vertical line widget
func NewVerticalLineWidget(styles ...string) LineWidget {
	builder := NewLineWidgetBuilder().WithOrientation("vertical")
	if len(styles) > 0 {
		builder = builder.WithStyles(styles...)
	}
	return builder.Build()
}

// NewDividerLine creates a horizontal divider line (commonly used for separating content)
func NewDividerLine() LineWidget {
	return NewLineWidgetBuilder().
		WithOrientation("horizontal").
		WithColor(api.Color{Hex: "#D3D3D3"}).
		WithThickness(1.0).
		WithLength(100.0).
		Build()
}

// NewSeparatorLine creates a thin separator line with custom color
func NewSeparatorLine(color api.Color) LineWidget {
	return NewLineWidgetBuilder().
		WithOrientation("horizontal").
		WithColor(color).
		WithThickness(0.5).
		WithLength(80.0).
		Build()
}

// NewBorderLine creates a line suitable for creating borders
func NewBorderLineWidget(thickness float64, color api.Color) LineWidget {
	return NewLineWidgetBuilder().
		WithThickness(thickness).
		WithColor(color).
		WithLength(100.0).
		Build()
}

// NewStyledLineWidget creates a line widget with styles parsed from strings
func NewStyledLineWidget(styles ...string) LineWidget {
	return NewLineWidgetBuilder().WithStyles(styles...).Build()
}
