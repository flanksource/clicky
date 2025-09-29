package tailwind

import (
	"slices"
	"strings"
)

// Property classification for efficient lookup
var (
	// Width/height prefixes
	widthPrefixes = []string{"w-", "min-w-", "max-w-"}
	heightPrefixes = []string{"h-", "min-h-", "max-h-"}

	// Text alignment keywords
	textAlignments = []string{"left", "center", "right", "justify"}

	// Vertical alignment prefixes
	verticalAlignPrefixes = []string{"align-"}

	// Font weight keywords
	fontWeights = []string{"thin", "light", "normal", "medium", "semibold", "bold", "extrabold", "black"}

	// Font size keywords
	fontSizes = []string{"xs", "sm", "base", "lg", "xl"}

	// Font family keywords
	fontFamilies = []string{"sans", "serif", "mono"}

	// Font style keywords
	fontStyleKeywords = []string{"italic", "not-italic"}

	// Padding/margin prefixes
	paddingPrefixes = []string{"p-", "px-", "py-", "pt-", "pr-", "pb-", "pl-"}
	marginPrefixes = []string{"m-", "mx-", "my-", "mt-", "mr-", "mb-", "ml-"}

	// Border width keywords
	borderWidthKeywords = []string{"0", "2", "4", "8"}

	// Display values
	displayValues = []string{"block", "inline", "inline-block", "flex", "inline-flex", "grid", "hidden"}

	// Position values
	positionValues = []string{"static", "fixed", "absolute", "relative", "sticky"}
)

// StyleMerger handles merging of Tailwind class strings with conflict resolution
type StyleMerger struct{}

// MergeStyles combines multiple Tailwind class strings
// Last class wins for conflicting properties (e.g., text-red-500 overrides text-blue-500)
func (sm *StyleMerger) MergeStyles(styles ...string) string {
	if len(styles) == 0 {
		return ""
	}

	// Parse all classes and create property map with order preservation
	propertyMap := make(map[string]string)
	propertyOrder := make([]string, 0)        // Track property insertion order
	propertyFirstSeen := make(map[string]bool) // Track which properties we've seen
	uniqueClassSet := make(map[string]bool)   // For deduplication
	var uniqueClasses []string                // Preserve order

	for _, styleStr := range styles {
		if styleStr == "" {
			continue
		}

		classes := strings.Fields(styleStr)
		for _, class := range classes {
			// Categorize class by property type
			property := sm.getPropertyType(class)

			if property == "unique" {
				// Non-conflicting classes are kept as-is but deduplicated
				if !uniqueClassSet[class] {
					uniqueClassSet[class] = true
					uniqueClasses = append(uniqueClasses, class)
				}
			} else {
				// Conflicting properties: last wins, but preserve first occurrence order
				if !propertyFirstSeen[property] {
					propertyFirstSeen[property] = true
					propertyOrder = append(propertyOrder, property)
				}
				propertyMap[property] = class
			}
		}
	}

	// Rebuild class string from merged properties
	var mergedClasses []string

	// Add unique classes first (preserve input order)
	mergedClasses = append(mergedClasses, uniqueClasses...)

	// Add property-based classes in the order first seen
	for _, property := range propertyOrder {
		mergedClasses = append(mergedClasses, propertyMap[property])
	}

	return strings.Join(mergedClasses, " ")
}

// hasPrefix checks if class starts with any of the given prefixes
func hasPrefix(class string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(class, prefix) {
			return true
		}
	}
	return false
}

// containsAnyKeyword checks if class contains any of the given keywords
func containsAnyKeyword(class string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(class, keyword) {
			return true
		}
	}
	return false
}

// getPropertyType categorizes Tailwind classes by their CSS property
// Returns "unique" for classes that don't conflict with others
func (sm *StyleMerger) getPropertyType(class string) string {
	switch {
	// Width properties
	case hasPrefix(class, widthPrefixes):
		return "width"

	// Height properties
	case hasPrefix(class, heightPrefixes):
		return "height"

	// Text alignment
	case strings.HasPrefix(class, "text-") && containsAnyKeyword(class, textAlignments):
		return "text-align"

	// Vertical alignment
	case hasPrefix(class, verticalAlignPrefixes):
		return "vertical-align"

	// Background color
	case strings.HasPrefix(class, "bg-") && !strings.Contains(class, "gradient"):
		return "background-color"

	// Font weight
	case strings.HasPrefix(class, "font-") && containsAnyKeyword(class, fontWeights):
		return "font-weight"

	// Font size (check before text color to avoid conflicts)
	case strings.HasPrefix(class, "text-") && containsAnyKeyword(class, fontSizes):
		return "font-size"

	// Text color
	case strings.HasPrefix(class, "text-") && !strings.Contains(class, "align") && !strings.Contains(class, "decoration") && !strings.Contains(class, "transform"):
		return "text-color"

	// Font style
	case slices.Contains(fontStyleKeywords, class):
		return "font-style"

	// Font family
	case strings.HasPrefix(class, "font-") && containsAnyKeyword(class, fontFamilies):
		return "font-family"

	// Padding - treat as unique for non-conflicting combinations
	case hasPrefix(class, paddingPrefixes):
		return "unique"

	// Margin - treat as unique for non-conflicting combinations
	case hasPrefix(class, marginPrefixes):
		return "unique"

	// Border width
	case strings.HasPrefix(class, "border-") && containsAnyKeyword(class, borderWidthKeywords) && !strings.Contains(class, "color"):
		return "border-width"

	// Border color
	case strings.HasPrefix(class, "border-") && !strings.Contains(class, "width") && !strings.Contains(class, "style"):
		return "border-color"

	// Display
	case slices.Contains(displayValues, class):
		return "display"

	// Position
	case slices.Contains(positionValues, class):
		return "position"

	default:
		// Non-conflicting classes are treated as unique
		return "unique"
	}
}

// Global merger instance for public API
var DefaultStyleMerger = &StyleMerger{}

// MergeStyles is the public API for merging style strings
func MergeStyles(styles ...string) string {
	return DefaultStyleMerger.MergeStyles(styles...)
}

// MergeClasses is a convenience function for merging base and override classes
func MergeClasses(baseClasses, overrideClasses string) string {
	return DefaultStyleMerger.MergeStyles(baseClasses, overrideClasses)
}
