package pdf

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/rustyoz/svg"
)

// SVGImporter handles parsing SVG content and converting to SVGBox elements
type SVGImporter struct{}

// NewSVGImporter creates a new SVG importer
func NewSVGImporter() *SVGImporter {
	return &SVGImporter{}
}

// ImportSVG parses SVG content and extracts elements that can be converted to SVGBox
func (importer *SVGImporter) ImportSVG(svgContent string) (*ImportedSVGElements, error) {
	// Parse the SVG content
	parsedSVG, err := svg.ParseSvg(svgContent, "imported", 1.0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SVG: %w", err)
	}

	elements := &ImportedSVGElements{
		Circles:  []CircleShape{},
		Cuts:     []Cut{},
		EdgeCuts: []EdgeCut{},
		Labels:   []Label{},
	}

	// Extract elements from the parsed SVG
	err = importer.extractElements(parsedSVG, elements)
	if err != nil {
		return nil, fmt.Errorf("failed to extract elements: %w", err)
	}

	return elements, nil
}

// ImportedSVGElements contains the elements extracted from an SVG file
type ImportedSVGElements struct {
	Circles  []CircleShape
	Cuts     []Cut
	EdgeCuts []EdgeCut
	Labels   []Label
}

// extractElements recursively extracts supported elements from the SVG
func (importer *SVGImporter) extractElements(svgElement *svg.Svg, elements *ImportedSVGElements) error {
	// Extract elements from top-level groups
	for _, group := range svgElement.Groups {
		err := importer.extractFromGroup(&group, elements)
		if err != nil {
			return err
		}
	}

	// Extract elements from the top-level Elements slice
	err := importer.extractFromElements(svgElement.Elements, elements)
	if err != nil {
		return err
	}

	return nil
}

// extractFromGroup extracts elements from SVG groups
func (importer *SVGImporter) extractFromGroup(group *svg.Group, elements *ImportedSVGElements) error {
	// Extract elements from the group's Elements slice
	err := importer.extractFromElements(group.Elements, elements)
	if err != nil {
		return err
	}

	return nil
}

// extractFromElements extracts elements from a slice of DrawingInstructionParser
func (importer *SVGImporter) extractFromElements(elementList []svg.DrawingInstructionParser, elements *ImportedSVGElements) error {
	for _, element := range elementList {
		switch elem := element.(type) {
		case *svg.Circle:
			circleShape, err := importer.convertCircle(elem)
			if err != nil {
				return err
			}
			elements.Circles = append(elements.Circles, *circleShape)
		case *svg.Rect:
			cut, edgeCut, err := importer.convertRectangle(elem)
			if err != nil {
				return err
			}
			if cut != nil {
				elements.Cuts = append(elements.Cuts, *cut)
			}
			if edgeCut != nil {
				elements.EdgeCuts = append(elements.EdgeCuts, *edgeCut)
			}
		case *svg.Group:
			// Recursively process nested groups
			err := importer.extractFromGroup(elem, elements)
			if err != nil {
				return err
			}
			// Note: The rustyoz/svg library doesn't seem to have a Text type,
			// so we'll skip text elements for now
		}
	}
	return nil
}

// convertCircle converts an SVG circle to a CircleShape
func (importer *SVGImporter) convertCircle(svgCircle *svg.Circle) (*CircleShape, error) {
	return &CircleShape{
		X:        svgCircle.Cx,
		Y:        svgCircle.Cy,
		Diameter: svgCircle.Radius * 2, // Convert radius to diameter
		Depth:    0,                    // SVG circles don't have depth
		Label:    extractLabelFromID(svgCircle.ID),
	}, nil
}

// convertRectangle converts an SVG rectangle to either a Cut or EdgeCut
func (importer *SVGImporter) convertRectangle(svgRect *svg.Rect) (*Cut, *EdgeCut, error) {
	// Parse width and height from string fields
	width, err := strconv.ParseFloat(svgRect.Width, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid rect width: %w", err)
	}

	height, err := strconv.ParseFloat(svgRect.Height, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid rect height: %w", err)
	}

	label := extractLabelFromID(svgRect.ID)

	// Since rustyoz/svg doesn't expose X,Y coordinates easily,
	// we'll make some assumptions for now
	// For a more complete implementation, we'd need to parse the transform
	// or use a different SVG library

	// For now, assume it's a regular cut and determine orientation by aspect ratio
	var orientation string
	var position float64 = 50.0 // Default center position

	if height > width {
		orientation = "vertical"
	} else {
		orientation = "horizontal"
	}

	return &Cut{
		Orientation: orientation,
		Position:    position,
		Width:       min(width, height), // Use the smaller dimension as the cut width
		Depth:       0,                  // SVG rectangles don't have depth
		Label:       label,
	}, nil, nil
}

// convertText converts an SVG text element to a Label
// Note: rustyoz/svg doesn't have a Text type, so this is a placeholder
func (importer *SVGImporter) convertText(textContent, id string, x, y float64, style string) *Label {
	if textContent == "" {
		return nil // Skip empty text
	}

	return &Label{
		Positionable: Positionable{
			Absolute: &api.Position{
				X: int(x),
				Y: int(y),
			},
		},
		Text: api.Text{
			Content: textContent,
			Class: api.Class{
				Font: &api.Font{
					Size: importer.parseFontSize(style),
				},
			},
		},
	}
}

// extractLabelFromID extracts a label from SVG element ID
func extractLabelFromID(id string) string {
	if id == "" {
		return ""
	}
	// Remove common prefixes and return a clean label
	id = strings.TrimPrefix(id, "circle-")
	id = strings.TrimPrefix(id, "rect-")
	id = strings.TrimPrefix(id, "text-")
	return id
}

// parseFontSize parses font size from SVG style string
func (importer *SVGImporter) parseFontSize(style string) float64 {
	if style == "" {
		return 12.0 // Default font size
	}

	// Look for font-size in style
	parts := strings.Split(style, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "font-size:") {
			sizeStr := strings.TrimPrefix(part, "font-size:")
			sizeStr = strings.TrimSuffix(strings.TrimSpace(sizeStr), "px")
			if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
				return size
			}
		}
	}

	return 12.0 // Default font size
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
