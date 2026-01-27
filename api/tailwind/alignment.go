package tailwind

import (
	"regexp"
	"strings"
)

// Type is a representation of a column align.
type Alignment string

const (
	// Left represents a left horizontal align.
	Left Alignment = "L"
	// Right represents a right horizontal align.
	Right Alignment = "R"
	// Center represents a center horizontal and/or vertical align.
	Center Alignment = "C"
	// Top represents a top vertical align.
	Top Alignment = "T"
	// Bottom represents a bottom vertical align.
	Bottom Alignment = "B"
	// Middle represents a middle align (from gofpdf).
	Middle Alignment = "M"
	// Justify represents a horizontal alignment that distributes
	// the text evenly between the left and right margins.
	Justify Alignment = "J"
)

// Compiled regex patterns for alignment parsing
var (
	// textAlignRegex matches text alignment classes: text-left, text-center, text-right, text-justify
	// with optional vertical alignment: text-left-top, text-center-middle, etc.
	textAlignRegex = regexp.MustCompile(`^text-(left|center|right|justify)(?:-(top|middle|bottom))?$`)

	// verticalAlignRegex matches standalone vertical alignment: align-top, align-middle, align-bottom
	verticalAlignRegex = regexp.MustCompile(`^align-(top|middle|bottom)$`)
)

// VerticalAlign represents vertical alignment options
type VerticalAlign int

const (
	VerticalTop VerticalAlign = iota
	VerticalMiddle
	VerticalBottom
)

// ParsedAlignment contains both horizontal and vertical alignment information
type ParsedAlignment struct {
	Horizontal Alignment
	Vertical   VerticalAlign
}

// AlignmentParser handles parsing of Tailwind alignment classes
type AlignmentParser struct{}

// NewAlignmentParser creates a new alignment parser
func NewAlignmentParser() *AlignmentParser {
	return &AlignmentParser{}
}

// ParseAlignment extracts alignment information from a style string
func (ap *AlignmentParser) ParseAlignment(style string) ParsedAlignment {
	classes := strings.Fields(style)
	alignment := ParsedAlignment{
		Horizontal: Left,           // Default horizontal alignment
		Vertical:   VerticalMiddle, // Default vertical alignment
	}

	// Parse alignment classes - last one wins for conflicts
	for _, class := range classes {
		if parsed := ap.parseAlignmentClass(class); parsed != nil {
			if parsed.Horizontal != Left || class == "text-left" {
				alignment.Horizontal = parsed.Horizontal
			}
			if parsed.Vertical != VerticalMiddle || strings.Contains(class, "align-") {
				alignment.Vertical = parsed.Vertical
			}
		}
	}

	return alignment
}

// stringToHorizontalAlign converts string to horizontal alignment type
func stringToHorizontalAlign(s string) Alignment {
	switch s {
	case "left":
		return Left
	case "center":
		return Center
	case "right":
		return Right
	case "justify":
		return Justify
	default:
		return Left
	}
}

// stringToVerticalAlign converts string to vertical alignment type
func stringToVerticalAlign(s string) VerticalAlign {
	switch s {
	case "top":
		return VerticalTop
	case "middle":
		return VerticalMiddle
	case "bottom":
		return VerticalBottom
	default:
		return VerticalMiddle
	}
}

// parseAlignmentClass parses a single alignment class using regex patterns
func (ap *AlignmentParser) parseAlignmentClass(class string) *ParsedAlignment {
	// Try text alignment pattern first (covers most cases including combined alignment)
	if matches := textAlignRegex.FindStringSubmatch(class); matches != nil {
		horizontal := stringToHorizontalAlign(matches[1])
		vertical := VerticalMiddle // Default

		// Check if vertical alignment is specified
		if len(matches) > 2 && matches[2] != "" {
			vertical = stringToVerticalAlign(matches[2])
		}

		return &ParsedAlignment{Horizontal: horizontal, Vertical: vertical}
	}

	// Try standalone vertical alignment pattern
	if matches := verticalAlignRegex.FindStringSubmatch(class); matches != nil {
		vertical := stringToVerticalAlign(matches[1])
		return &ParsedAlignment{Horizontal: Left, Vertical: vertical}
	}

	return nil
}

// Global parser instance for public API
var DefaultAlignmentParser = NewAlignmentParser()

// ParseAlignment is the public API for parsing alignment from style strings
func ParseAlignment(style string) ParsedAlignment {
	return DefaultAlignmentParser.ParseAlignment(style)
}
