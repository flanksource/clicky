package tailwind

import (
	"fmt"
	"strings"
)

// ParseTailwindColor parses a Tailwind color class and returns the hex color value
// Supports formats like:
// - "red-500" -> base color with shade
// - "bg-blue-700" -> background color prefix
// - "text-green-300" -> text color prefix
// - "border-gray-400" -> border color prefix
// - "red" -> defaults to 500 shade
// - "black", "white", "transparent" -> special colors
func ParseTailwindColor(colorClass string) (string, error) {
	if colorClass == "" {
		return "", fmt.Errorf("empty color class")
	}

	// Remove common Tailwind prefixes
	colorName := colorClass
	prefixes := []string{"bg-", "text-", "border-", "ring-", "decoration-", "divide-", "outline-", "fill-", "stroke-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(colorClass, prefix) {
			colorName = strings.TrimPrefix(colorClass, prefix)
			break
		}
	}

	// Check for special colors first
	if color, ok := TailwindSpecialColors[colorName]; ok {
		return color, nil
	}

	// Parse color and shade
	parts := strings.Split(colorName, "-")

	if len(parts) == 1 {
		// Just a color name, default to 500 shade
		if colorMap, ok := TailwindColors[parts[0]]; ok {
			if shade, ok := colorMap["500"]; ok {
				return shade, nil
			}
		}
		// Not a Tailwind color, return as-is (might be a hex color or other CSS color)
		return colorName, nil
	}

	if len(parts) == 2 {
		// Color with shade
		baseName := parts[0]
		shade := parts[1]

		if colorMap, ok := TailwindColors[baseName]; ok {
			if shadeColor, ok := colorMap[shade]; ok {
				return shadeColor, nil
			}
			return "", fmt.Errorf("invalid shade '%s' for color '%s'", shade, baseName)
		}
		// Not a Tailwind color, return as-is
		return colorName, nil
	}

	// Handle multi-word color names (shouldn't happen with standard Tailwind)
	return colorName, nil
}

// IsTailwindColor checks if a string is a valid Tailwind color class
func IsTailwindColor(colorClass string) bool {
	_, err := ParseTailwindColor(colorClass)
	return err == nil
}

// GetTailwindColorName extracts the base color name from a Tailwind class
// e.g., "bg-red-500" -> "red", "text-blue-300" -> "blue"
func GetTailwindColorName(colorClass string) string {
	// Remove prefixes
	colorName := colorClass
	prefixes := []string{"bg-", "text-", "border-", "ring-", "decoration-", "divide-", "outline-", "fill-", "stroke-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(colorClass, prefix) {
			colorName = strings.TrimPrefix(colorClass, prefix)
			break
		}
	}

	// Get base color name (before shade)
	parts := strings.Split(colorName, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	return colorName
}

// GetTailwindShade extracts the shade from a Tailwind color class
// e.g., "bg-red-500" -> "500", "blue-300" -> "300", "green" -> "500" (default)
func GetTailwindShade(colorClass string) string {
	// Remove prefixes
	colorName := colorClass
	prefixes := []string{"bg-", "text-", "border-", "ring-", "decoration-", "divide-", "outline-", "fill-", "stroke-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(colorClass, prefix) {
			colorName = strings.TrimPrefix(colorClass, prefix)
			break
		}
	}

	parts := strings.Split(colorName, "-")
	if len(parts) >= 2 {
		return parts[1]
	}
	return "500" // Default shade
}

// Color converts a Tailwind color class to a color string
// It parses the Tailwind color and returns the appropriate color value
func Color(colorClass string) string {
	// Try to parse as Tailwind color
	hexColor, err := ParseTailwindColor(colorClass)
	if err != nil {
		// If parsing fails, return the original string
		// This allows for fallback to standard CSS colors
		return colorClass
	}

	// Special handling for transparent
	if hexColor == "transparent" {
		// Return empty string for transparent
		return ""
	}
	if hexColor == "currentColor" {
		// Return as-is for currentColor
		return hexColor
	}

	// Return the hex color
	return hexColor
}

// Style represents a parsed Tailwind style with transform info
// This mirrors the key fields from lipgloss.Style without importing lipgloss
type Style struct {
	// Text styling
	Foreground string // Text color
	Background string // Background color
	Bold       bool   // Bold text
	Faint      bool   // Faint text
	Italic     bool   // Italic text
	Underline  bool   // Underlined text
	Strikethrough bool // Strikethrough text
	
	// Layout
	MaxWidth int // Maximum width
	
	// Text transformation (not in lipgloss, but handled separately)
	TextTransform string
}

// ParseStyle parses a Tailwind style string and returns a Style struct
func ParseStyle(styleStr string) Style {
	style := Style{}

	if styleStr == "" {
		return style
	}

	// Split multiple classes
	classes := strings.Fields(styleStr)
	for _, class := range classes {
		// Parse Tailwind color classes
		if strings.HasPrefix(class, "text-") {
			// Check if it's a color or another text utility
			if !isTextUtilityClass(class) {
				style.Foreground = Color(class)
			}
		} else if strings.HasPrefix(class, "bg-") {
			style.Background = Color(class)
		}

		// Font weight classes
		switch class {
		case "bold", "font-bold":
			style.Bold = true
		case "font-semibold", "font-medium":
			// Use bold as fallback for semibold/medium
			style.Bold = true
		case "font-light", "font-thin", "font-extralight":
			// Use faint for light weights
			style.Faint = true
		case "font-normal":
			// Reset bold and faint
			style.Bold = false
			style.Faint = false
		}

		// Font style classes
		switch class {
		case "italic", "font-italic":
			style.Italic = true
		case "not-italic":
			style.Italic = false
		}

		// Text decoration classes
		switch class {
		case "underline":
			style.Underline = true
		case "line-through", "strikethrough":
			style.Strikethrough = true
		case "no-underline":
			style.Underline = false
		case "overline":
			// Use underline as fallback for overline
			style.Underline = true
		}

		// Text transform classes
		switch class {
		case "uppercase":
			style.TextTransform = "uppercase"
		case "lowercase":
			style.TextTransform = "lowercase"
		case "capitalize":
			style.TextTransform = "capitalize"
		case "normal-case":
			style.TextTransform = ""
		}

		// Additional text utilities
		switch class {
		case "truncate", "text-ellipsis", "text-clip":
			style.MaxWidth = 50 // Example max width
		}

		// Visibility utilities
		switch class {
		case "invisible":
			style.Faint = true
		case "visible":
			style.Faint = false
		}

		// Opacity utilities (using Faint as approximation)
		switch class {
		case "opacity-50", "opacity-75", "opacity-25":
			style.Faint = true
		case "opacity-100":
			style.Faint = false
		}
	}

	return style
}

// isTextUtilityClass checks if a text- prefixed class is a utility rather than a color
func isTextUtilityClass(class string) bool {
	utilities := []string{
		"text-left", "text-center", "text-right", "text-justify",
		"text-xs", "text-sm", "text-base", "text-lg", "text-xl",
		"text-2xl", "text-3xl", "text-4xl", "text-5xl", "text-6xl",
		"text-7xl", "text-8xl", "text-9xl",
		"text-ellipsis", "text-clip", "text-wrap", "text-nowrap",
	}

	for _, util := range utilities {
		if class == util {
			return true
		}
	}
	return false
}


// TransformText applies text transformation based on the transform type
func TransformText(text string, transform string) string {
	switch transform {
	case "uppercase":
		return strings.ToUpper(text)
	case "lowercase":
		return strings.ToLower(text)
	case "capitalize":
		return capitalizeWords(text)
	default:
		return text
	}
}

// capitalizeWords capitalizes the first letter of each word
func capitalizeWords(text string) string {
	words := strings.Fields(text)
	for i, word := range words {
		if len(word) > 0 {
			// Capitalize first letter, keep rest as-is
			words[i] = strings.ToUpper(string(word[0])) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// ApplyStyle applies a Tailwind style string to text, including text transforms
func ApplyStyle(text string, styleStr string) (string, Style) {
	parsedStyle := ParseStyle(styleStr)

	// Apply text transform first
	if parsedStyle.TextTransform != "" {
		text = TransformText(text, parsedStyle.TextTransform)
	}

	return text, parsedStyle
}
