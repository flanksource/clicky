package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	marotoimages "github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// SVGWidget renders an SVGBox as a widget in the PDF
type SVGWidget struct {
	SVGBox SVGBox   `json:"svg_box"`
	Height *float64 `json:"height,omitempty"`
}

// NewSVGWidget creates a new SVG widget from an SVGBox
func NewSVGWidget(svgBox SVGBox) *SVGWidget {
	return &SVGWidget{
		SVGBox: svgBox,
	}
}

// WithHeight sets the height of the SVG widget in mm
func (w *SVGWidget) WithHeight(height float64) *SVGWidget {
	w.Height = &height
	return w
}

// Draw implements the Widget interface
func (w SVGWidget) Draw(b *Builder) error {
	// Generate SVG content
	svgBytes, err := w.SVGBox.GenerateSVG()
	if err != nil {
		return fmt.Errorf("failed to generate SVG: %w", err)
	}

	// Convert SVG to PNG for embedding in PDF
	// Since maroto doesn't support SVG directly, we need to convert to a raster format
	pngBytes, err := w.convertSVGToPNG(svgBytes)
	if err != nil {
		return fmt.Errorf("failed to convert SVG to PNG: %w", err)
	}

	// Calculate height
	height := 100.0 // Default height in mm
	if w.Height != nil {
		height = *w.Height
	}

	// Create image component from PNG bytes
	imageComponent := marotoimages.NewFromBytes(pngBytes, extension.Png)
	imageCol := col.New(12).Add(imageComponent)
	b.maroto.AddRow(height, imageCol)

	return nil
}

// convertSVGToPNG converts SVG bytes to PNG bytes with aspect ratio preservation
func (w SVGWidget) convertSVGToPNG(svgBytes []byte) ([]byte, error) {
	// Parse SVG using oksvg
	icon, err := oksvg.ReadIconStream(bytes.NewReader(svgBytes), oksvg.StrictErrorMode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SVG: %w", err)
	}

	// Extract viewBox or use default dimensions
	svgWidth, svgHeight, err := w.extractSVGDimensions(svgBytes)
	if err != nil {
		// Fall back to default square aspect ratio
		svgWidth, svgHeight = 100, 100
	}

	// Calculate aspect ratio
	aspectRatio := float64(svgWidth) / float64(svgHeight)

	// Determine target dimensions (default to 400px width)
	var targetWidth, targetHeight int
	defaultSize := 400

	if aspectRatio >= 1.0 {
		// Landscape or square: fix width, calculate height
		targetWidth = defaultSize
		targetHeight = int(float64(defaultSize) / aspectRatio)
	} else {
		// Portrait: fix height, calculate width
		targetHeight = defaultSize
		targetWidth = int(float64(defaultSize) * aspectRatio)
	}

	// Set the target size on the icon
	icon.SetTarget(0, 0, float64(targetWidth), float64(targetHeight))

	// Create RGBA image
	rgba := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	// Create scanner and rasterizer
	scanner := rasterx.NewScannerGV(targetWidth, targetHeight, rgba, rgba.Bounds())
	raster := rasterx.NewDasher(targetWidth, targetHeight, scanner)

	// Render SVG to image
	icon.Draw(raster, 1.0)

	// Encode to PNG
	var pngBuf bytes.Buffer
	err = png.Encode(&pngBuf, rgba)
	if err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}

	return pngBuf.Bytes(), nil
}

// extractSVGDimensions parses SVG content to extract width and height
func (w SVGWidget) extractSVGDimensions(svgBytes []byte) (float64, float64, error) {
	svgContent := string(svgBytes)

	// Look for width and height attributes
	width, widthOk := w.extractAttribute(svgContent, "width")
	height, heightOk := w.extractAttribute(svgContent, "height")

	if widthOk && heightOk {
		return width, height, nil
	}

	// Look for viewBox attribute as fallback
	if viewBox := w.extractViewBox(svgContent); len(viewBox) == 4 {
		return viewBox[2], viewBox[3], nil // width and height from viewBox
	}

	return 0, 0, fmt.Errorf("could not extract SVG dimensions")
}

// extractAttribute extracts a numeric attribute value from SVG content
func (w SVGWidget) extractAttribute(svgContent, attrName string) (float64, bool) {
	// Simple regex-like parsing to find attribute="value"
	attrPattern := attrName + `="`
	start := strings.Index(svgContent, attrPattern)
	if start == -1 {
		return 0, false
	}

	start += len(attrPattern)
	end := strings.Index(svgContent[start:], `"`)
	if end == -1 {
		return 0, false
	}

	valueStr := svgContent[start : start+end]
	// Remove units (px, mm, etc.)
	valueStr = strings.TrimSuffix(valueStr, "px")
	valueStr = strings.TrimSuffix(valueStr, "mm")
	valueStr = strings.TrimSuffix(valueStr, "cm")
	valueStr = strings.TrimSuffix(valueStr, "pt")
	valueStr = strings.TrimSuffix(valueStr, "pc")
	valueStr = strings.TrimSuffix(valueStr, "in")

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, false
	}

	return value, true
}

// extractViewBox extracts viewBox values [x, y, width, height] from SVG content
func (w SVGWidget) extractViewBox(svgContent string) []float64 {
	viewBoxPattern := `viewBox="`
	start := strings.Index(svgContent, viewBoxPattern)
	if start == -1 {
		return nil
	}

	start += len(viewBoxPattern)
	end := strings.Index(svgContent[start:], `"`)
	if end == -1 {
		return nil
	}

	viewBoxStr := svgContent[start : start+end]
	parts := strings.Fields(viewBoxStr)
	if len(parts) != 4 {
		return nil
	}

	values := make([]float64, 4)
	for i, part := range parts {
		value, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil
		}
		values[i] = value
	}

	return values
}

// Removed drawError method - errors should be returned, not drawn in the PDF
// Errors are now properly returned to the caller for handling

// drawSVGDescription draws a text description of the SVG contents
func (w SVGWidget) drawSVGDescription(b *Builder) error {
	var description bytes.Buffer

	// Describe the box
	description.WriteString(fmt.Sprintf("SVG Box: %dx%d",
		w.SVGBox.Rectangle.Width, w.SVGBox.Rectangle.Height))

	// Describe circles
	if len(w.SVGBox.Circles) > 0 {
		description.WriteString(fmt.Sprintf("\nCircles: %d", len(w.SVGBox.Circles)))
		for i, circle := range w.SVGBox.Circles {
			if i < 3 { // Show first 3
				description.WriteString(fmt.Sprintf("\n  - Circle at (%.1f,%.1f), diameter %.1f",
					circle.X, circle.Y, circle.Diameter))
				if circle.Label != "" {
					description.WriteString(fmt.Sprintf(" [%s]", circle.Label))
				}
			} else if i == 3 {
				description.WriteString(fmt.Sprintf("\n  ... and %d more", len(w.SVGBox.Circles)-3))
				break
			}
		}
	}

	// Describe cuts
	if len(w.SVGBox.Cuts) > 0 {
		description.WriteString(fmt.Sprintf("\nCuts: %d", len(w.SVGBox.Cuts)))
		for i, cut := range w.SVGBox.Cuts {
			if i < 3 { // Show first 3
				description.WriteString(fmt.Sprintf("\n  - %s cut at %.1f, width %.1f",
					cut.Orientation, cut.Position, cut.Width))
				if cut.Label != "" {
					description.WriteString(fmt.Sprintf(" [%s]", cut.Label))
				}
			} else if i == 3 {
				description.WriteString(fmt.Sprintf("\n  ... and %d more", len(w.SVGBox.Cuts)-3))
				break
			}
		}
	}

	// Describe edge cuts
	if len(w.SVGBox.EdgeCuts) > 0 {
		description.WriteString(fmt.Sprintf("\nEdge Cuts: %d", len(w.SVGBox.EdgeCuts)))
		for i, edgeCut := range w.SVGBox.EdgeCuts {
			if i < 3 { // Show first 3
				description.WriteString(fmt.Sprintf("\n  - %s edge cut, width %.1f",
					edgeCut.Edge, edgeCut.Width))
				if edgeCut.Label != "" {
					description.WriteString(fmt.Sprintf(" [%s]", edgeCut.Label))
				}
			} else if i == 3 {
				description.WriteString(fmt.Sprintf("\n  ... and %d more", len(w.SVGBox.EdgeCuts)-3))
				break
			}
		}
	}

	// Describe labels
	if len(w.SVGBox.Labels) > 0 {
		description.WriteString(fmt.Sprintf("\nLabels: %d", len(w.SVGBox.Labels)))
		for i, label := range w.SVGBox.Labels {
			if i < 3 { // Show first 3
				description.WriteString(fmt.Sprintf("\n  - \"%s\"", label.Text.Content))
			} else if i == 3 {
				description.WriteString(fmt.Sprintf("\n  ... and %d more", len(w.SVGBox.Labels)-3))
				break
			}
		}
	}

	// Create description text component
	descProps := props.Text{
		Size:  9,
		Style: fontstyle.Normal,
		Align: align.Left,
		Color: &props.Color{Red: 100, Green: 100, Blue: 100},
	}

	descComponent := text.New(description.String(), descProps)
	descCol := col.New(12).Add(descComponent)
	b.maroto.AddRow(30, descCol) // 30mm height for description

	return nil
}

// FromSVGContent creates an SVGWidget from raw SVG content string
func FromSVGContent(svgContent string, box api.Box) (*SVGWidget, error) {
	// Create base SVGBox
	svgBox := SVGBox{
		Box:                      box,
		EnableCollisionAvoidance: true,
	}

	// Import elements from SVG content
	err := svgBox.ImportFromSVG(svgContent)
	if err != nil {
		return nil, fmt.Errorf("failed to import SVG content: %w", err)
	}

	return NewSVGWidget(svgBox), nil
}
