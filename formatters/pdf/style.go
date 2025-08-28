package pdf

import (
	"strconv"
	"strings"

	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"github.com/flanksource/clicky/api"
)

// StyleConverter handles converting api.Class and Tailwind styles to Maroto properties
type StyleConverter struct{}

// NewStyleConverter creates a new style converter for Maroto
func NewStyleConverter() *StyleConverter {
	return &StyleConverter{}
}

// ConvertToTextProps converts api.Class to Maroto text properties
func (s *StyleConverter) ConvertToTextProps(class api.Class) *props.Text {
	textProps := &props.Text{
		Size:  12, // Default size
		Style: fontstyle.Normal,
		Align: align.Left,
	}

	// Apply font properties
	if class.Font != nil {
		// Font size
		if class.Font.Size > 0 {
			textProps.Size = class.Font.Size * 12 // Convert rem to points (assuming 1rem = 12pt)
		}

		// Font style
		if class.Font.Bold && class.Font.Italic {
			textProps.Style = fontstyle.BoldItalic
		} else if class.Font.Bold {
			textProps.Style = fontstyle.Bold
		} else if class.Font.Italic {
			textProps.Style = fontstyle.Italic
		}

		// Note: Underline and Strikethrough are not directly supported in Maroto text props
	}

	// Apply text color
	if class.Foreground != nil {
		textProps.Color = s.ConvertColor(*class.Foreground)
	}

	// Apply text alignment if specified
	// This would need to be extended based on your alignment requirements

	return textProps
}

// ConvertColor converts api.Color to Maroto color
func (s *StyleConverter) ConvertColor(color api.Color) *props.Color {
	// Parse hex color
	if color.Hex != "" {
		r, g, b := hexToRGB(color.Hex)
		return &props.Color{
			Red:   r,
			Green: g,
			Blue:  b,
		}
	}

	// Default to black
	return &props.Color{
		Red:   0,
		Green: 0,
		Blue:  0,
	}
}

// ConvertBackgroundColor converts api.Color to Maroto background color
func (s *StyleConverter) ConvertBackgroundColor(color api.Color) *props.Color {
	return s.ConvertColor(color)
}

// CalculateTextHeight calculates appropriate row height for text based on font size
func (s *StyleConverter) CalculateTextHeight(class api.Class) float64 {
	baseHeight := 6.0 // Default row height in mm

	if class.Font != nil && class.Font.Size > 0 {
		// Scale height based on font size
		baseHeight = class.Font.Size * 6 // Convert rem to mm (approximately)
	}

	// Add padding if specified
	if class.Padding != nil {
		baseHeight += (class.Padding.Top + class.Padding.Bottom) * 4 // Convert rem to mm
	}

	return baseHeight
}

// CalculatePadding calculates padding for a cell
func (s *StyleConverter) CalculatePadding(padding *api.Padding) (left, top, right, bottom float64) {
	if padding == nil {
		return 0, 0, 0, 0
	}

	// Convert rem to mm (1rem ≈ 4mm for PDF)
	left = padding.Left * 4
	top = padding.Top * 4
	right = padding.Right * 4
	bottom = padding.Bottom * 4

	return
}

// ParseTailwindToProps parses Tailwind classes and returns Maroto text properties
func (s *StyleConverter) ParseTailwindToProps(tailwindClasses string) *props.Text {
	// First resolve the Tailwind classes to api.Class
	class := api.ResolveStyles(tailwindClasses)

	// Then convert to Maroto properties
	return s.ConvertToTextProps(class)
}

// ConvertToTableProps converts api.Class to table properties
// Note: Maroto v2 doesn't have direct table cell properties,
// so this returns background color for cell background
func (s *StyleConverter) ConvertToTableBackgroundColor(class api.Class) *props.Color {
	if class.Background != nil {
		return s.ConvertBackgroundColor(*class.Background)
	}
	return nil
}

// ConvertAlignment converts text alignment string to Maroto alignment
func (s *StyleConverter) ConvertAlignment(alignStr string) align.Type {
	switch strings.ToLower(alignStr) {
	case "center":
		return align.Center
	case "right":
		return align.Right
	case "justify":
		return align.Justify
	default:
		return align.Left
	}
}

// hexToRGB converts hex color string to RGB values (0-255)
func hexToRGB(hex string) (r, g, b int) {
	// Remove # if present
	hex = strings.TrimPrefix(hex, "#")

	// Parse hex values
	if len(hex) == 6 {
		if val, err := strconv.ParseInt(hex[0:2], 16, 0); err == nil {
			r = int(val)
		}
		if val, err := strconv.ParseInt(hex[2:4], 16, 0); err == nil {
			g = int(val)
		}
		if val, err := strconv.ParseInt(hex[4:6], 16, 0); err == nil {
			b = int(val)
		}
	} else if len(hex) == 3 {
		// Handle short form (#RGB)
		if val, err := strconv.ParseInt(string(hex[0])+string(hex[0]), 16, 0); err == nil {
			r = int(val)
		}
		if val, err := strconv.ParseInt(string(hex[1])+string(hex[1]), 16, 0); err == nil {
			g = int(val)
		}
		if val, err := strconv.ParseInt(string(hex[2])+string(hex[2]), 16, 0); err == nil {
			b = int(val)
		}
	}

	return r, g, b
}

// ConvertBorderColor converts api.Line to border color for lines
func (s *StyleConverter) ConvertBorderColor(line api.Line) *props.Color {
	return s.ConvertColor(line.Color)
}

// GetBorderWidth gets the border width from api.Line
func (s *StyleConverter) GetBorderWidth(line api.Line) float64 {
	return float64(line.Width)
}
