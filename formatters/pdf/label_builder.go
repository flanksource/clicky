package pdf

import (
	"strings"

	"github.com/flanksource/clicky/api"
)

// LabelBuilder provides a fluent API for creating Label instances
type LabelBuilder struct {
	text     string
	position *LabelPosition
	absolute *api.Position
	class    api.Class
}

// NewLabelBuilder creates a new LabelBuilder with the specified text
func NewLabelBuilder(text string) *LabelBuilder {
	return &LabelBuilder{
		text: text,
	}
}

// WithStyles sets styling classes using variadic string parameters
// Supports Tailwind classes, custom classes, or style directives
func (b *LabelBuilder) WithStyles(styles ...string) *LabelBuilder {
	for _, style := range styles {
		// Resolve Tailwind/custom styles and merge with existing class
		resolvedClass := api.ResolveStyles(style)
		b.class = mergeClassesLabel(b.class, resolvedClass)
	}
	return b
}

// WithPosition sets the position using a string like "top-left", "center", "bottom-right"
func (b *LabelBuilder) WithPosition(position string) *LabelBuilder {
	pos := ParsePosition(position)
	b.position = &pos
	return b
}

// WithAbsolute sets absolute positioning coordinates
func (b *LabelBuilder) WithAbsolute(x, y int) *LabelBuilder {
	b.absolute = &api.Position{X: x, Y: y}
	return b
}

// WithClass directly sets the api.Class for advanced styling control
func (b *LabelBuilder) WithClass(class api.Class) *LabelBuilder {
	b.class = class
	return b
}

// WithFont sets font properties directly
func (b *LabelBuilder) WithFont(size float64, bold bool) *LabelBuilder {
	if b.class.Font == nil {
		b.class.Font = &api.Font{}
	}
	if size > 0 {
		b.class.Font.Size = size
	}
	b.class.Font.Bold = bold
	return b
}

// WithColor sets the text color
func (b *LabelBuilder) WithColor(color api.Color) *LabelBuilder {
	b.class.Foreground = &color
	return b
}

// Build creates and returns the Label instance
func (b *LabelBuilder) Build() Label {
	return Label{
		Positionable: Positionable{
			Position: b.position,
			Absolute: b.absolute,
		},
		Text: api.Text{
			Content: b.text,
			Class:   b.class,
		},
	}
}

// mergeClassesLabel merges two api.Class instances for labels, with the second taking precedence
func mergeClassesLabel(base, overlay api.Class) api.Class {
	result := base

	// Merge font properties
	if overlay.Font != nil {
		if result.Font == nil {
			result.Font = &api.Font{}
		}
		if overlay.Font.Size > 0 {
			result.Font.Size = overlay.Font.Size
		}
		if overlay.Font.Bold {
			result.Font.Bold = overlay.Font.Bold
		}
		if overlay.Font.Name != "" {
			result.Font.Name = overlay.Font.Name
		}
	}

	// Merge colors
	if overlay.Foreground != nil {
		result.Foreground = overlay.Foreground
	}
	if overlay.Background != nil {
		result.Background = overlay.Background
	}

	// Merge padding
	if overlay.Padding != nil {
		result.Padding = overlay.Padding
	}

	// Merge border
	if overlay.Border != nil {
		result.Border = overlay.Border
	}

	return result
}

// Helper functions for common label patterns

// NewCenteredLabel creates a centered label with the given text
func NewCenteredLabel(text string) Label {
	return NewLabelBuilder(text).WithPosition("center").Build()
}

// NewTitleLabel creates a title-style label (top-center, bold, larger font)
func NewTitleLabel(text string) Label {
	return NewLabelBuilder(text).
		WithPosition("top-center").
		WithFont(16, true).
		Build()
}

// NewCornerLabel creates a corner label with the specified position
func NewCornerLabel(text, position string) Label {
	return NewLabelBuilder(text).WithPosition(position).Build()
}

// NewStyledLabel creates a label with custom styles
func NewStyledLabel(text string, styles ...string) Label {
	builder := NewLabelBuilder(text)
	if len(styles) > 0 {
		// First style parameter is treated as position if it looks like a position
		firstStyle := strings.ToLower(styles[0])
		if isPositionString(firstStyle) {
			builder = builder.WithPosition(firstStyle)
			// Apply remaining styles
			if len(styles) > 1 {
				builder = builder.WithStyles(styles[1:]...)
			}
		} else {
			// All parameters are styles
			builder = builder.WithStyles(styles...)
		}
	}
	return builder.Build()
}

// isPositionString checks if a string looks like a position descriptor
func isPositionString(s string) bool {
	positionKeywords := []string{
		"center", "middle", "top", "bottom", "left", "right", "inside", "outside",
	}

	parts := strings.Split(s, "-")
	for _, part := range parts {
		for _, keyword := range positionKeywords {
			if part == keyword {
				return true
			}
		}
	}
	return false
}
