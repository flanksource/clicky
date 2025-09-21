package tailwind

import (
	"strconv"
	"strings"
)

// Border structs to avoid import cycle
type BorderColor struct {
	Hex     string
	Opacity float64
}

type LineStyle string

const (
	Solid  LineStyle = "solid"
	Dashed LineStyle = "dashed"
	Dotted LineStyle = "dotted"
	Double LineStyle = "double"
	None   LineStyle = "none"
)

type Line struct {
	Color BorderColor
	Style LineStyle
	Width float64
}

type Borders struct {
	Left   Line
	Right  Line
	Top    Line
	Bottom Line
}

// ParseBorders parses Tailwind border classes and returns a Borders struct
// Supports:
// - Width: border, border-0, border-2, border-4, border-8, border-[3px]
// - Style: border-solid, border-dashed, border-dotted, border-double, border-none
// - Color: border-gray-700, border-red-500, etc.
// - Side-specific: border-t-2, border-r-0, border-b-4, border-l-2
// - Combined: border-t-2 border-t-gray-500 border-t-dashed
func ParseBorders(styles ...string) Borders {
	borders := Borders{}

	// Default border properties
	defaultWidth := 1.0
	defaultStyle := Solid
	defaultColor := BorderColor{Hex: "#000000"}

	// Parse global border properties first
	globalWidth := defaultWidth
	globalStyle := defaultStyle
	globalColor := defaultColor
	hasGlobalBorder := false

	// Parse side-specific properties
	sideProperties := map[string]*Line{
		"t": &borders.Top,
		"r": &borders.Right,
		"b": &borders.Bottom,
		"l": &borders.Left,
	}

	// Initialize all sides with defaults
	for _, line := range sideProperties {
		line.Width = 0 // Start with no border
		line.Style = defaultStyle
		line.Color = defaultColor
	}

	// Flatten all style strings into individual classes
	var allClasses []string
	for _, style := range styles {
		classes := strings.Fields(style)
		allClasses = append(allClasses, classes...)
	}

	// Process each class
	for _, class := range allClasses {
		if strings.HasPrefix(class, "border") {
			parseBorderClass(class, &borders, &globalWidth, &globalStyle, &globalColor, &hasGlobalBorder)
		}
	}

	// Apply global border properties to all sides if global border was specified
	if hasGlobalBorder {
		borders.Top = Line{Width: globalWidth, Style: globalStyle, Color: globalColor}
		borders.Right = Line{Width: globalWidth, Style: globalStyle, Color: globalColor}
		borders.Bottom = Line{Width: globalWidth, Style: globalStyle, Color: globalColor}
		borders.Left = Line{Width: globalWidth, Style: globalStyle, Color: globalColor}
	}

	return borders
}

// parseBorderClass parses a single border class and updates the appropriate properties
func parseBorderClass(class string, borders *Borders, globalWidth *float64, globalStyle *LineStyle, globalColor *BorderColor, hasGlobalBorder *bool) {
	parts := strings.Split(class, "-")

	if len(parts) < 2 {
		// Just "border" - enable global border with default width
		if class == "border" {
			*hasGlobalBorder = true
			*globalWidth = 1.0
		}
		return
	}

	// Check if it's side-specific (border-t, border-r, border-b, border-l)
	if len(parts) >= 2 && (parts[1] == "t" || parts[1] == "r" || parts[1] == "b" || parts[1] == "l") {
		side := parts[1]
		var targetLine *Line

		switch side {
		case "t":
			targetLine = &borders.Top
		case "r":
			targetLine = &borders.Right
		case "b":
			targetLine = &borders.Bottom
		case "l":
			targetLine = &borders.Left
		}

		if targetLine == nil {
			return
		}

		if len(parts) == 2 {
			// Just "border-t" - enable side with default width
			targetLine.Width = 1.0
			return
		}

		// Parse side-specific property
		property := strings.Join(parts[2:], "-")

		// Check if it's a width
		if width, ok := parseBorderWidth(property); ok {
			targetLine.Width = width
			return
		}

		// Check if it's a style
		if style, ok := parseBorderStyle(property); ok {
			targetLine.Style = style
			return
		}

		// Check if it's a color
		if colorHex := Color(property); colorHex != "" {
			targetLine.Color = BorderColor{Hex: colorHex}
			return
		}

		return
	}

	// Global border property
	property := strings.Join(parts[1:], "-")

	// Check if it's a width
	if width, ok := parseBorderWidth(property); ok {
		*hasGlobalBorder = true
		*globalWidth = width
		return
	}

	// Check if it's a style
	if style, ok := parseBorderStyle(property); ok {
		*hasGlobalBorder = true
		*globalStyle = style
		return
	}

	// Check if it's a color (border-gray-500, etc.)
	if colorHex := Color(property); colorHex != "" {
		*hasGlobalBorder = true
		*globalColor = BorderColor{Hex: colorHex}
		return
	}
}

// parseBorderWidth parses border width from a class suffix
// Supports: 0, 2, 4, 8, [3px], etc.
func parseBorderWidth(suffix string) (float64, bool) {
	switch suffix {
	case "0":
		return 0, true
	case "2":
		return 2, true
	case "4":
		return 4, true
	case "8":
		return 8, true
	default:
		// Handle arbitrary values like [3px]
		if strings.HasPrefix(suffix, "[") && strings.HasSuffix(suffix, "]") {
			value := strings.Trim(suffix, "[]")
			value = strings.TrimSuffix(value, "px") // Remove px unit
			if width, err := strconv.ParseFloat(value, 64); err == nil {
				return width, true
			}
		}
		// Handle numeric values
		if width, err := strconv.ParseFloat(suffix, 64); err == nil {
			return width, true
		}
	}
	return 0, false
}

// parseBorderStyle parses border style from a class suffix
func parseBorderStyle(suffix string) (LineStyle, bool) {
	switch suffix {
	case "solid":
		return Solid, true
	case "dashed":
		return Dashed, true
	case "dotted":
		return Dotted, true
	case "double":
		return Double, true
	case "none":
		return None, true
	}
	return Solid, false
}

// Helper functions for common border patterns

// NewSolidBorder creates a solid border with specified width and color
func NewSolidBorder(width float64, color BorderColor) Borders {
	line := Line{
		Width: width,
		Style: Solid,
		Color: color,
	}
	return Borders{
		Top:    line,
		Right:  line,
		Bottom: line,
		Left:   line,
	}
}

// NewDashedBorder creates a dashed border with specified width and color
func NewDashedBorder(width float64, color BorderColor) Borders {
	line := Line{
		Width: width,
		Style: Dashed,
		Color: color,
	}
	return Borders{
		Top:    line,
		Right:  line,
		Bottom: line,
		Left:   line,
	}
}

// NewTopBottomBorder creates borders only on top and bottom
func NewTopBottomBorder(width float64, color BorderColor, style LineStyle) Borders {
	line := Line{
		Width: width,
		Style: style,
		Color: color,
	}
	return Borders{
		Top:    line,
		Right:  Line{Width: 0},
		Bottom: line,
		Left:   Line{Width: 0},
	}
}

// NewLeftRightBorder creates borders only on left and right
func NewLeftRightBorder(width float64, color BorderColor, style LineStyle) Borders {
	line := Line{
		Width: width,
		Style: style,
		Color: color,
	}
	return Borders{
		Top:    Line{Width: 0},
		Right:  line,
		Bottom: Line{Width: 0},
		Left:   line,
	}
}
