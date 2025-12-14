package api

import (
	"strings"
	"sync"

	"github.com/flanksource/clicky/api/tailwind"
	"github.com/muesli/termenv"
)

// Global cache for ResolveStyles to avoid repeated parsing
var (
	styleCache     = make(map[string]Class)
	styleCacheLock sync.RWMutex
)

func ResolveStyles(styles ...string) Class {
	// Create cache key from all style strings
	cacheKey := strings.Join(styles, "|")

	// Check cache first
	styleCacheLock.RLock()
	if cached, ok := styleCache[cacheKey]; ok {
		styleCacheLock.RUnlock()
		return cached
	}
	styleCacheLock.RUnlock()

	var resolved Class

	// Process each style string
	for _, styleStr := range styles {
		if styleStr == "" {
			continue
		}

		// Split into individual classes
		classes := strings.Fields(styleStr)

		for _, class := range classes {
			// Parse colors
			if strings.HasPrefix(class, "text-") && !tailwind.IsTextUtilityClass(class) {
				color := tailwind.Color(class)
				if color != "" {
					resolved.Foreground = &Color{Hex: color}
				}
			} else if strings.HasPrefix(class, "bg-") {
				color := tailwind.Color(class)
				if color != "" {
					resolved.Background = &Color{Hex: color}
				}
			}

			// Parse font properties
			parsedStyle := tailwind.ParseStyle(class)

			// Initialize Font if needed
			if resolved.Font == nil {
				resolved.Font = &Font{}
			}

			// Apply font family
			if strings.HasPrefix(class, "font-family-") {
				fontName := strings.TrimPrefix(class, "font-family-")
				switch strings.ToLower(fontName) {
				case "arial":
					resolved.Font.Name = "Arial"
				case "times":
					resolved.Font.Name = "Times"
				case "helvetica":
					resolved.Font.Name = "Helvetica"
				case "courier":
					resolved.Font.Name = "Courier"
				case "georgia":
					resolved.Font.Name = "Georgia"
				case "verdana":
					resolved.Font.Name = "Verdana"
				default:
					// Allow custom font names (case-sensitive for exact match)
					resolved.Font.Name = fontName
				}
			}

			// Apply font weight
			switch class {
			case "bold", "font-bold", "font-semibold", "font-medium":
				resolved.Font.Bold = true
			case "font-normal":
				resolved.Font.Bold = false
			}

			// Apply font style
			switch class {
			case "italic", "font-italic":
				resolved.Font.Italic = true
			case "not-italic":
				resolved.Font.Italic = false
			}

			// Apply text decoration
			switch class {
			case "underline":
				resolved.Font.Underline = true
			case "no-underline":
				resolved.Font.Underline = false
			}

			if class == "line-through" || class == "strikethrough" || class == "text-strikethrough" {
				resolved.Font.Strikethrough = true
			}

			// Apply faint/opacity
			switch class {
			case "font-light", "font-thin", "font-extralight", "opacity-50", "opacity-75", "opacity-25":
				resolved.Font.Faint = true
			case "opacity-100":
				resolved.Font.Faint = false
			}

			// Parse font size
			if fontSize := tailwind.ParseFontSize(class); fontSize > 0 {
				resolved.Font.Size = fontSize
			}

			// Parse padding
			top, right, bottom, left := tailwind.ParsePadding(class)
			if top != nil || right != nil || bottom != nil || left != nil {
				if resolved.Padding == nil {
					resolved.Padding = &Padding{}
				}

				// Apply non-nil values, converting to Point type
				if top != nil {
					resolved.Padding.Top = NewPoint(*top)
				}
				if right != nil {
					resolved.Padding.Right = NewPoint(*right)
				}
				if bottom != nil {
					resolved.Padding.Bottom = NewPoint(*bottom)
				}
				if left != nil {
					resolved.Padding.Left = NewPoint(*left)
				}
			}

			// Apply colors from parsed style (as fallback)
			if parsedStyle.Foreground != "" && resolved.Foreground == nil {
				resolved.Foreground = &Color{Hex: parsedStyle.Foreground}
			}
			if parsedStyle.Background != "" && resolved.Background == nil {
				resolved.Background = &Color{Hex: parsedStyle.Background}
			}
		}
	}

	// Store in cache before returning
	styleCacheLock.Lock()
	styleCache[cacheKey] = resolved
	styleCacheLock.Unlock()

	return resolved
}

// ApplyTailwindStyle processes Tailwind CSS classes and applies text transformations,
// returning both the transformed text and parsed style information.
func ApplyTailwindStyle(text, styleStr string) (string, TailwindStyle) {
	transformedText, twStyle := tailwind.ApplyStyle(text, styleStr)

	// Convert to our TailwindStyle struct
	style := TailwindStyle{
		Foreground:    twStyle.Foreground,
		Background:    twStyle.Background,
		Bold:          twStyle.Bold,
		Faint:         twStyle.Faint,
		Italic:        twStyle.Italic,
		Underline:     twStyle.Underline,
		Strikethrough: twStyle.Strikethrough,
		TextTransform: twStyle.TextTransform,
	}

	return transformedText, style
}

func classToTailwindStyle(class Class) TailwindStyle {
	style := TailwindStyle{}

	// Apply colors
	if class.Foreground != nil {
		style.Foreground = class.Foreground.Hex
	}
	if class.Background != nil {
		style.Background = class.Background.Hex
	}

	// Apply font properties
	if class.Font != nil {
		style.Bold = class.Font.Bold
		style.Faint = class.Font.Faint
		style.Italic = class.Font.Italic
		style.Underline = class.Font.Underline
		style.Strikethrough = class.Font.Strikethrough
	}

	return style
}

// TailwindStyle contains parsed CSS styling information extracted from Tailwind classes.
type TailwindStyle struct {
	Foreground    string
	Background    string
	Font          Font
	Bold          bool
	Faint         bool
	Italic        bool
	Underline     bool
	Strikethrough bool
	TextTransform string
}

func formatANSI(text string, style TailwindStyle) string {
	if text == "" {
		return ""
	}
	output := termenv.NewOutput(termenv.DefaultOutput().Writer(), termenv.WithProfile(termenv.ANSI))
	termStyle := output.String(text)

	// Apply text decorations
	if style.Bold {
		termStyle = termStyle.Bold()
	}
	if style.Faint {
		termStyle = termStyle.Faint()
	}
	if style.Italic {
		termStyle = termStyle.Italic()
	}
	if style.Underline {
		termStyle = termStyle.Underline()
	}

	// Apply foreground color using termenv
	if style.Foreground != "" {
		if color := hexToTermenvColor(style.Foreground); color != nil {
			termStyle = termStyle.Foreground(color)
		}
	}

	// Apply background color using termenv
	if style.Background != "" {
		if color := hexToTermenvColor(style.Background); color != nil {
			termStyle = termStyle.Background(color)
		}
	}

	// Handle strikethrough manually since termenv doesn't support it
	result := termStyle.String()
	if style.Strikethrough {
		// Remove any existing reset codes and add strikethrough
		if strings.HasSuffix(result, "\x1b[0m") {
			result = strings.TrimSuffix(result, "\x1b[0m")
			result = "\x1b[9m" + result + "\x1b[0m"
		} else {
			result = "\x1b[9m" + result + "\x1b[29m"
		}
	}

	return result
}

func hexToTermenvColor(hex string) termenv.Color {
	if hex == "" {
		return nil
	}

	// Handle special colors
	switch hex {
	case "transparent":
		return nil
	case "currentColor":
		return termenv.ANSIColor(termenv.ANSIBrightWhite)
	}

	// Convert hex to termenv color
	if strings.HasPrefix(hex, "#") {
		return termenv.RGBColor(hex)
	}

	return nil
}
