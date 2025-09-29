package tailwind

import (
	"regexp"
	"strconv"
	"strings"
)

// WidthType represents different width specification types
type WidthType int

const (
	WidthAuto WidthType = iota
	WidthPercentage
	WidthCharacter
	WidthPixel
	WidthRem
	WidthFraction
	WidthFixed
)

// WidthSpec represents a parsed width specification
type WidthSpec struct {
	Type     WidthType
	Value    float64
	Unit     string
	IsMin    bool   // min-w-[...]
	IsMax    bool   // max-w-[...]
	Original string // Original class for debugging
}

// WidthParser handles parsing of Tailwind width classes
type WidthParser struct {
	// Compiled regex patterns for performance
	arbitraryWidthRegex *regexp.Regexp // w-[10ch], min-w-[20px], max-w-[50%]
	fractionWidthRegex  *regexp.Regexp // w-1/2, w-1/3, w-2/5
	namedWidthRegex     *regexp.Regexp // w-auto, w-full, w-fit
	numericWidthRegex   *regexp.Regexp // w-12, w-64 (Tailwind spacing scale)
}

// NewWidthParser creates a new width parser with compiled regex patterns
func NewWidthParser() *WidthParser {
	return &WidthParser{
		// Arbitrary values: w-[10ch], min-w-[20px], max-w-[50%]
		arbitraryWidthRegex: regexp.MustCompile(`^(min-w|max-w|w)-\[([0-9.]+)(ch|px|rem|%|em|vh|vw)\]$`),

		// Fractions: w-1/2, w-1/3, w-2/5, etc.
		fractionWidthRegex: regexp.MustCompile(`^(min-w|max-w|w)-([0-9]+)\/([0-9]+)$`),

		// Named widths: w-auto, w-full, w-fit, w-screen
		namedWidthRegex: regexp.MustCompile(`^(min-w|max-w|w)-(auto|full|fit|screen|min|max)$`),

		// Numeric scale: w-0, w-1, w-12, w-64, etc. (Tailwind spacing)
		numericWidthRegex: regexp.MustCompile(`^(min-w|max-w|w)-([0-9]+(?:\.[0-9]+)?)$`),
	}
}

// ParseWidth parses a single width class and returns the specification
func (wp *WidthParser) ParseWidth(class string) (*WidthSpec, bool) {
	// Try arbitrary values first: w-[10ch], min-w-[20px], max-w-[50%]
	if matches := wp.arbitraryWidthRegex.FindStringSubmatch(class); matches != nil {
		prefix := matches[1] // "w", "min-w", "max-w"
		value := matches[2]  // "10", "20", "50"
		unit := matches[3]   // "ch", "px", "%"

		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, false
		}

		widthType := wp.getWidthTypeFromUnit(unit)
		return &WidthSpec{
			Type:     widthType,
			Value:    val,
			Unit:     unit,
			IsMin:    prefix == "min-w",
			IsMax:    prefix == "max-w",
			Original: class,
		}, true
	}

	// Try fractions: w-1/2, w-1/3, w-2/5
	if matches := wp.fractionWidthRegex.FindStringSubmatch(class); matches != nil {
		prefix := matches[1]      // "w", "min-w", "max-w"
		numerator := matches[2]   // "1", "2"
		denominator := matches[3] // "2", "3", "5"

		num, err1 := strconv.ParseFloat(numerator, 64)
		den, err2 := strconv.ParseFloat(denominator, 64)
		if err1 != nil || err2 != nil || den == 0 {
			return nil, false
		}

		fraction := num / den
		return &WidthSpec{
			Type:     WidthFraction,
			Value:    fraction,
			Unit:     "fraction",
			IsMin:    prefix == "min-w",
			IsMax:    prefix == "max-w",
			Original: class,
		}, true
	}

	// Try named widths: w-auto, w-full, w-fit
	if matches := wp.namedWidthRegex.FindStringSubmatch(class); matches != nil {
		prefix := matches[1] // "w", "min-w", "max-w"
		name := matches[2]   // "auto", "full", "fit"

		var widthType WidthType
		var value float64

		switch name {
		case "auto", "fit":
			widthType = WidthAuto
			value = 0
		case "full":
			widthType = WidthPercentage
			value = 100
		case "screen":
			widthType = WidthPercentage
			value = 100 // 100vw equivalent
		default:
			return nil, false
		}

		return &WidthSpec{
			Type:     widthType,
			Value:    value,
			Unit:     name,
			IsMin:    prefix == "min-w",
			IsMax:    prefix == "max-w",
			Original: class,
		}, true
	}

	// Try numeric scale: w-12, w-64 (Tailwind spacing scale)
	if matches := wp.numericWidthRegex.FindStringSubmatch(class); matches != nil {
		prefix := matches[1] // "w", "min-w", "max-w"
		value := matches[2]  // "12", "64"

		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, false
		}

		// Convert Tailwind spacing scale to rem (scale * 0.25rem)
		remValue := val * 0.25
		return &WidthSpec{
			Type:     WidthRem,
			Value:    remValue,
			Unit:     "rem",
			IsMin:    prefix == "min-w",
			IsMax:    prefix == "max-w",
			Original: class,
		}, true
	}

	return nil, false
}

// ParseWidthFromStyle extracts the last width specification from a style string
func (wp *WidthParser) ParseWidthFromStyle(style string) *WidthSpec {
	classes := strings.Fields(style)
	var lastWidth *WidthSpec

	// Parse all width classes, last one wins
	for _, class := range classes {
		if widthSpec, found := wp.ParseWidth(class); found {
			lastWidth = widthSpec
		}
	}

	return lastWidth
}

// getWidthTypeFromUnit determines the width type from the unit
func (wp *WidthParser) getWidthTypeFromUnit(unit string) WidthType {
	switch unit {
	case "ch":
		return WidthCharacter
	case "px":
		return WidthPixel
	case "rem", "em":
		return WidthRem
	case "%":
		return WidthPercentage
	case "vh", "vw":
		return WidthPercentage // Treat as viewport percentage
	default:
		return WidthFixed
	}
}

// Global parser instance for public API
var DefaultWidthParser = NewWidthParser()

// ParseWidth is the public API for parsing width classes
func ParseWidth(class string) (*WidthSpec, bool) {
	return DefaultWidthParser.ParseWidth(class)
}

// ParseWidthFromStyle is the public API for extracting width from style strings
func ParseWidthFromStyle(style string) *WidthSpec {
	return DefaultWidthParser.ParseWidthFromStyle(style)
}
