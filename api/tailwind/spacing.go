package tailwind

import (
	"strconv"
	"strings"
)

// TailwindSpacing defines the Tailwind CSS spacing scale in rem units
var TailwindSpacing = map[string]float64{
	"0":   0,
	"px":  0.0625, // 1px
	"0.5": 0.125,  // 2px
	"1":   0.25,   // 4px
	"1.5": 0.375,  // 6px
	"2":   0.5,    // 8px
	"2.5": 0.625,  // 10px
	"3":   0.75,   // 12px
	"3.5": 0.875,  // 14px
	"4":   1,      // 16px
	"5":   1.25,   // 20px
	"6":   1.5,    // 24px
	"7":   1.75,   // 28px
	"8":   2,      // 32px
	"9":   2.25,   // 36px
	"10":  2.5,    // 40px
	"11":  2.75,   // 44px
	"12":  3,      // 48px
	"14":  3.5,    // 56px
	"16":  4,      // 64px
	"20":  5,      // 80px
	"24":  6,      // 96px
	"28":  7,      // 112px
	"32":  8,      // 128px
	"36":  9,      // 144px
	"40":  10,     // 160px
	"44":  11,     // 176px
	"48":  12,     // 192px
	"52":  13,     // 208px
	"56":  14,     // 224px
	"60":  15,     // 240px
	"64":  16,     // 256px
	"72":  18,     // 288px
	"80":  20,     // 320px
	"96":  24,     // 384px
}

// TailwindFontSizes defines Tailwind font size scale in rem units
var TailwindFontSizes = map[string]float64{
	"xs":   0.75,  // 12px
	"sm":   0.875, // 14px
	"base": 1,     // 16px
	"lg":   1.125, // 18px
	"xl":   1.25,  // 20px
	"2xl":  1.5,   // 24px
	"3xl":  1.875, // 30px
	"4xl":  2.25,  // 36px
	"5xl":  3,     // 48px
	"6xl":  3.75,  // 60px
	"7xl":  4.5,   // 72px
	"8xl":  6,     // 96px
	"9xl":  8,     // 128px
}

// PaddingValue represents padding for a single side
type PaddingValue struct {
	Value float64
	Unit  string
}

// ParsePadding parses a Tailwind padding utility class
// Returns padding values in rem units for each side (top, right, bottom, left)
// Returns nil values for sides that are not set
func ParsePadding(class string) (top, right, bottom, left *float64) {
	if !strings.Contains(class, "p-") && !strings.Contains(class, "px-") &&
		!strings.Contains(class, "py-") && !strings.Contains(class, "pt-") &&
		!strings.Contains(class, "pr-") && !strings.Contains(class, "pb-") &&
		!strings.Contains(class, "pl-") {
		return nil, nil, nil, nil
	}

	// Extract the value part
	var prefix string
	var valueStr string

	if strings.HasPrefix(class, "p-") {
		prefix = "p"
		valueStr = strings.TrimPrefix(class, "p-")
	} else if strings.HasPrefix(class, "px-") {
		prefix = "px"
		valueStr = strings.TrimPrefix(class, "px-")
	} else if strings.HasPrefix(class, "py-") {
		prefix = "py"
		valueStr = strings.TrimPrefix(class, "py-")
	} else if strings.HasPrefix(class, "pt-") {
		prefix = "pt"
		valueStr = strings.TrimPrefix(class, "pt-")
	} else if strings.HasPrefix(class, "pr-") {
		prefix = "pr"
		valueStr = strings.TrimPrefix(class, "pr-")
	} else if strings.HasPrefix(class, "pb-") {
		prefix = "pb"
		valueStr = strings.TrimPrefix(class, "pb-")
	} else if strings.HasPrefix(class, "pl-") {
		prefix = "pl"
		valueStr = strings.TrimPrefix(class, "pl-")
	} else {
		return nil, nil, nil, nil
	}

	// Parse the value
	value, exists := TailwindSpacing[valueStr]
	if !exists {
		// Try parsing as a custom value (e.g., p-[10px])
		if strings.HasPrefix(valueStr, "[") && strings.HasSuffix(valueStr, "]") {
			customValue := strings.TrimPrefix(strings.TrimSuffix(valueStr, "]"), "[")
			if v, err := parseCustomSpacing(customValue); err == nil {
				value = v
			} else {
				return nil, nil, nil, nil
			}
		} else {
			return nil, nil, nil, nil
		}
	}

	// Apply to appropriate sides
	switch prefix {
	case "p":
		// All sides
		return &value, &value, &value, &value
	case "px":
		// Horizontal (left and right)
		return nil, &value, nil, &value
	case "py":
		// Vertical (top and bottom)
		return &value, nil, &value, nil
	case "pt":
		// Top only
		return &value, nil, nil, nil
	case "pr":
		// Right only
		return nil, &value, nil, nil
	case "pb":
		// Bottom only
		return nil, nil, &value, nil
	case "pl":
		// Left only
		return nil, nil, nil, &value
	}

	return nil, nil, nil, nil
}

// ParseFontSize parses a Tailwind font size utility class
// Returns the font size in rem units, or 0 if not a font size class
func ParseFontSize(class string) float64 {
	if !strings.HasPrefix(class, "text-") {
		return 0
	}

	sizeStr := strings.TrimPrefix(class, "text-")

	// Check if it's a standard font size
	if size, exists := TailwindFontSizes[sizeStr]; exists {
		return size
	}

	// Check for custom value (e.g., text-[14px])
	if strings.HasPrefix(sizeStr, "[") && strings.HasSuffix(sizeStr, "]") {
		customValue := strings.TrimPrefix(strings.TrimSuffix(sizeStr, "]"), "[")
		if v, err := parseCustomSpacing(customValue); err == nil {
			return v
		}
	}

	return 0
}

// parseCustomSpacing parses custom spacing values like "10px", "1.5rem", "24"
func parseCustomSpacing(value string) (float64, error) {
	value = strings.TrimSpace(value)

	// Handle px values
	if strings.HasSuffix(value, "px") {
		pxStr := strings.TrimSuffix(value, "px")
		if px, err := strconv.ParseFloat(pxStr, 64); err == nil {
			// Convert px to rem (assuming 16px = 1rem)
			return px / 16, nil
		}
	}

	// Handle rem values
	if strings.HasSuffix(value, "rem") {
		remStr := strings.TrimSuffix(value, "rem")
		if rem, err := strconv.ParseFloat(remStr, 64); err == nil {
			return rem, nil
		}
	}

	// Handle em values (treat as rem)
	if strings.HasSuffix(value, "em") {
		emStr := strings.TrimSuffix(value, "em")
		if em, err := strconv.ParseFloat(emStr, 64); err == nil {
			return em, nil
		}
	}

	// Handle unitless values (assume rem)
	if v, err := strconv.ParseFloat(value, 64); err == nil {
		return v, nil
	}

	return 0, strconv.ErrSyntax
}

// MergePadding merges padding values, with later values overriding earlier ones
// nil values don't override existing values
func MergePadding(existing, new struct{ Top, Right, Bottom, Left *float64 }) struct{ Top, Right, Bottom, Left *float64 } {
	result := existing

	if new.Top != nil {
		result.Top = new.Top
	}
	if new.Right != nil {
		result.Right = new.Right
	}
	if new.Bottom != nil {
		result.Bottom = new.Bottom
	}
	if new.Left != nil {
		result.Left = new.Left
	}

	return result
}

