package tailwind

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/muesli/termenv"
)

// ParseTailwindColor parses a Tailwind color class and returns the hex color value
// with adaptive color adjustment for terminal background.
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
		isDark := isBackgroundDark()
		return adaptColorForBackground(color, isDark), nil
	}

	// Parse color and shade
	parts := strings.Split(colorName, "-")

	if len(parts) == 1 {
		// Just a color name, default to 500 shade
		if colorMap, ok := TailwindColors[parts[0]]; ok {
			if shade, ok := colorMap["500"]; ok {
				isDark := isBackgroundDark()
				return adaptColorForBackground(shade, isDark), nil
			}
		}
		// Not a Tailwind color, return as-is (might be a hex color or other CSS color)
		isDark := isBackgroundDark()
		return adaptColorForBackground(colorName, isDark), nil
	}

	if len(parts) == 2 {
		// Color with shade
		baseName := parts[0]
		shade := parts[1]

		if colorMap, ok := TailwindColors[baseName]; ok {
			if shadeColor, ok := colorMap[shade]; ok {
				isDark := isBackgroundDark()
				return adaptColorForBackground(shadeColor, isDark), nil
			}
			return "", fmt.Errorf("invalid shade '%s' for color '%s'", shade, baseName)
		}
		// Not a Tailwind color, return as-is
		isDark := isBackgroundDark()
		return adaptColorForBackground(colorName, isDark), nil
	}

	// Handle multi-word color names (shouldn't happen with standard Tailwind)
	hexColor := colorName

	// Apply adaptive color mapping for better visibility
	isDark := isBackgroundDark()
	return adaptColorForBackground(hexColor, isDark), nil
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

// Color converts a Tailwind color class to a raw color string without adaptation
// It parses the Tailwind color and returns the appropriate color value
func Color(colorClass string) string {
	// Try to parse as raw Tailwind color (without background adaptation)
	hexColor, err := ParseRawTailwindColor(colorClass)
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

// ParseRawTailwindColor parses a Tailwind color class and returns the raw hex color value
// without any background adaptation. This is used for getting the original Tailwind color values.
func ParseRawTailwindColor(colorClass string) (string, error) {
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
		return color, nil // Return raw color without adaptation
	}

	// Parse color and shade
	parts := strings.Split(colorName, "-")

	if len(parts) == 1 {
		// Just a color name, default to 500 shade
		if colorMap, ok := TailwindColors[parts[0]]; ok {
			if shade, ok := colorMap["500"]; ok {
				return shade, nil // Return raw color without adaptation
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
				return shadeColor, nil // Return raw color without adaptation
			}
			return "", fmt.Errorf("invalid shade '%s' for color '%s'", shade, baseName)
		}
		// Not a Tailwind color, return as-is
		return colorName, nil
	}

	// Handle multi-word color names (shouldn't happen with standard Tailwind)
	return colorName, nil
}

// Style represents a parsed Tailwind style with transform info
// This mirrors the key fields from lipgloss.Style without importing lipgloss
type Style struct {
	// Text styling
	Foreground    string // Text color
	Background    string // Background color
	Bold          bool   // Bold text
	Faint         bool   // Faint text
	Italic        bool   // Italic text
	Underline     bool   // Underlined text
	Strikethrough bool   // Strikethrough text

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

// IsTextUtilityClass checks if a text- prefixed class is a utility rather than a color
func IsTextUtilityClass(class string) bool {
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

// isTextUtilityClass is a private wrapper for backwards compatibility
func isTextUtilityClass(class string) bool {
	return IsTextUtilityClass(class)
}

// TransformText applies text transformation based on the transform type
func TransformText(text, transform string) string {
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
func ApplyStyle(text, styleStr string) (string, Style) {
	parsedStyle := ParseStyle(styleStr)

	// Apply text transform first
	if parsedStyle.TextTransform != "" {
		text = TransformText(text, parsedStyle.TextTransform)
	}

	return text, parsedStyle
}

// ClassToFgColor converts a Tailwind class to a termenv foreground color
func ClassToFgColor(class string) termenv.Color {
	// Parse the tailwind color to get hex value
	hexColor, err := ParseTailwindColor(class)
	if err != nil {
		// Return default color for invalid classes
		return termenv.ANSIColor(termenv.ANSIRed)
	}

	// Handle special colors
	switch hexColor {
	case "transparent":
		return nil
	case "currentColor":
		return termenv.ANSIColor(termenv.ANSIBrightWhite)
	case "":
		return termenv.ANSIColor(termenv.ANSIRed) // Fallback
	}

	// Convert hex to termenv color
	if strings.HasPrefix(hexColor, "#") {
		return termenv.RGBColor(hexColor)
	}

	// Fallback to red for any invalid color
	return termenv.ANSIColor(termenv.ANSIRed)
}

// ClassToBgColor converts a Tailwind class to a termenv background color
func ClassToBgColor(class string) termenv.Color {
	// Parse the tailwind color to get hex value
	hexColor, err := ParseTailwindColor(class)
	if err != nil {
		// Return default color for invalid classes
		return termenv.ANSIColor(termenv.ANSIWhite)
	}

	// Handle special colors
	switch hexColor {
	case "transparent":
		return nil
	case "currentColor":
		return termenv.ANSIColor(termenv.ANSIBrightBlack)
	case "":
		return termenv.ANSIColor(termenv.ANSIWhite) // Fallback
	}

	// Convert hex to termenv color
	if strings.HasPrefix(hexColor, "#") {
		return termenv.RGBColor(hexColor)
	}

	// Fallback to white background for any invalid color
	return termenv.ANSIColor(termenv.ANSIWhite)
}

// Background detection cache to avoid repeated expensive operations
var (
	backgroundCache     *bool
	backgroundCacheLock sync.RWMutex
)

// isBackgroundDark returns whether the terminal has a dark background (cached)
func isBackgroundDark() bool {
	backgroundCacheLock.RLock()
	if backgroundCache != nil {
		defer backgroundCacheLock.RUnlock()
		return *backgroundCache
	}
	backgroundCacheLock.RUnlock()

	// Compute background detection
	backgroundCacheLock.Lock()
	defer backgroundCacheLock.Unlock()

	// Double-check after acquiring write lock
	if backgroundCache != nil {
		return *backgroundCache
	}

	isDark := termenv.HasDarkBackground()
	backgroundCache = &isDark
	return isDark
}

// hexToRGB converts a hex color to RGB values (0-255)
func hexToRGB(hex string) (r, g, b int, err error) {
	// Remove # if present
	hex = strings.TrimPrefix(hex, "#")

	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color: %s", hex)
	}

	// Parse each component
	rVal, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}
	gVal, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}
	bVal, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	return int(rVal), int(gVal), int(bVal), nil
}

// rgbToHex converts RGB values to hex string
func rgbToHex(r, g, b int) string {
	// Clamp values to 0-255 range
	r = int(math.Max(0, math.Min(255, float64(r))))
	g = int(math.Max(0, math.Min(255, float64(g))))
	b = int(math.Max(0, math.Min(255, float64(b))))

	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// calculateLuminance calculates relative luminance using WCAG formula
func calculateLuminance(r, g, b int) float64 {
	// Convert to 0-1 range
	rNorm := float64(r) / 255.0
	gNorm := float64(g) / 255.0
	bNorm := float64(b) / 255.0

	// Apply gamma correction
	rLin := linearizeColorComponent(rNorm)
	gLin := linearizeColorComponent(gNorm)
	bLin := linearizeColorComponent(bNorm)

	// Calculate luminance using WCAG coefficients
	return 0.2126*rLin + 0.7152*gLin + 0.0722*bLin
}

// linearizeColorComponent applies gamma correction to color component
func linearizeColorComponent(c float64) float64 {
	if c <= 0.03928 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// rgbToHSL converts RGB values to HSL
func rgbToHSL(r, g, b int) (h, s, l float64) {
	// Normalize to 0-1 range
	rNorm := float64(r) / 255.0
	gNorm := float64(g) / 255.0
	bNorm := float64(b) / 255.0

	max := math.Max(rNorm, math.Max(gNorm, bNorm))
	min := math.Min(rNorm, math.Min(gNorm, bNorm))

	// Lightness
	l = (max + min) / 2

	if max == min {
		// Achromatic (gray)
		h, s = 0, 0
	} else {
		d := max - min

		// Saturation
		if l > 0.5 {
			s = d / (2 - max - min)
		} else {
			s = d / (max + min)
		}

		// Hue
		switch max {
		case rNorm:
			h = (gNorm - bNorm) / d
			if gNorm < bNorm {
				h += 6
			}
		case gNorm:
			h = (bNorm-rNorm)/d + 2
		case bNorm:
			h = (rNorm-gNorm)/d + 4
		}
		h /= 6
	}

	return h, s, l
}

// hslToRGB converts HSL values to RGB
func hslToRGB(h, s, l float64) (r, g, b int) {
	var rNorm, gNorm, bNorm float64

	if s == 0 {
		// Achromatic
		rNorm, gNorm, bNorm = l, l, l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q

		rNorm = hueToRGB(p, q, h+1.0/3.0)
		gNorm = hueToRGB(p, q, h)
		bNorm = hueToRGB(p, q, h-1.0/3.0)
	}

	// Convert to 0-255 range
	r = int(math.Round(rNorm * 255))
	g = int(math.Round(gNorm * 255))
	b = int(math.Round(bNorm * 255))

	return r, g, b
}

// hueToRGB helper function for HSL to RGB conversion
func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6.0 {
		return p + (q-p)*6*t
	}
	if t < 1.0/2.0 {
		return q
	}
	if t < 2.0/3.0 {
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

// adaptColorForBackground adapts a hex color based on terminal background
func adaptColorForBackground(hexColor string, isDark bool) string {
	if hexColor == "" || hexColor == "transparent" || hexColor == "currentColor" {
		return hexColor
	}

	// Parse RGB values
	r, g, b, err := hexToRGB(hexColor)
	if err != nil {
		return hexColor // Return original on parse error
	}

	// Calculate luminance
	luminance := calculateLuminance(r, g, b)

	// Don't adapt if color already has good visibility
	const minLuminanceThreshold = 0.15 // Very dark colors need adaptation
	const maxLuminanceThreshold = 0.85 // Very light colors need adaptation

	if isDark {
		// Dark background: make dark colors lighter
		if luminance < minLuminanceThreshold {
			// Convert to HSL for better control
			h, s, l := rgbToHSL(r, g, b)

			// Boost lightness using formula: L' = L + (target - L) * factor
			targetLightness := 0.75 // Target for good visibility on dark background
			factor := 0.8           // How much to move toward target
			newL := l + (targetLightness-l)*factor

			// Ensure minimum lightness
			newL = math.Max(0.6, newL)

			// Slightly reduce saturation to prevent over-brightness
			newS := s * 0.9

			// Convert back to RGB
			r, g, b = hslToRGB(h, newS, newL)
			return rgbToHex(r, g, b)
		}
	} else {
		// Light background: make light colors darker
		if luminance > maxLuminanceThreshold {
			h, s, l := rgbToHSL(r, g, b)

			// Reduce lightness for visibility on light background
			targetLightness := 0.25 // Target for good visibility on light background
			factor := 0.8
			newL := l + (targetLightness-l)*factor

			// Ensure maximum lightness
			newL = math.Min(0.4, newL)

			// Slightly increase saturation for better distinction
			newS := math.Min(1.0, s*1.1)

			r, g, b = hslToRGB(h, newS, newL)
			return rgbToHex(r, g, b)
		}
	}

	// No adaptation needed
	return hexColor
}
