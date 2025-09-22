package pdf

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
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

	// Convert SVG to PDF for embedding in PDF
	// PDF format provides better quality and smaller file sizes than raster formats
	pdfBytes, err := ConvertSVGToPDF(svgBytes)
	if err != nil {
		return fmt.Errorf("failed to convert SVG to PDF: %w", err)
	}

	// Calculate height for the widget
	height := 100.0 // Default height in mm
	if w.Height != nil {
		height = *w.Height
	}

	// Embed the converted PDF directly
	// Create a temporary file for the PDF bytes
	tempFile, err := os.CreateTemp("", "svg_converted_*.pdf")
	if err != nil {
		return fmt.Errorf("failed to create temp file for PDF: %w", err)
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Write PDF bytes to temp file
	_, err = tempFile.Write(pdfBytes)
	if err != nil {
		return fmt.Errorf("failed to write PDF bytes: %w", err)
	}
	tempFile.Close()

	// Use PDF embed widget to embed the converted SVG
	embedWidget := NewPDFEmbedWidget(tempFile.Name())
	if w.Height != nil {
		embedWidget = embedWidget.WithSize(float64(w.SVGBox.Width)*0.264583, height) // Convert pixels to mm (96 DPI)
	}
	return embedWidget.Draw(b)
}

// ConvertSVGToPNG converts SVG bytes to PNG bytes with aspect ratio preservation
func ConvertSVGToPNG(svgBytes []byte) ([]byte, error) {
	// Basic SVG validation
	svgContent := string(svgBytes)
	if !strings.Contains(svgContent, "<svg") {
		return nil, fmt.Errorf("failed to parse SVG: content does not appear to be valid SVG")
	}

	tmp, err := os.CreateTemp("", "svg_*.svg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmp.Name()) // Clean up

	// Write SVG bytes to temp file
	if _, err := tmp.Write(svgBytes); err != nil {
		return nil, fmt.Errorf("failed to write SVG to temp file: %w", err)
	}
	tmp.Close()

	options := DefaultConvertOptions()
	options.Format = "png"
	if err := Convert(context.Background(), tmp.Name(), tmp.Name()+".png", options); err != nil {
		return nil, fmt.Errorf("failed to convert SVG to PNG: %w", err)
	}
	pngFile, err := os.Open(tmp.Name() + ".png")
	if err != nil {
		return nil, fmt.Errorf("failed to open PNG file: %w", err)
	}
	defer pngFile.Close()

	return os.ReadFile(tmp.Name() + ".png")
}

// ConvertSVGToPDF converts SVG bytes to PDF bytes
func ConvertSVGToPDF(svgBytes []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "svg_*.svg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmp.Name()) // Clean up

	// Write SVG bytes to temp file
	if _, err := tmp.Write(svgBytes); err != nil {
		return nil, fmt.Errorf("failed to write SVG to temp file: %w", err)
	}
	tmp.Close()

	// Create options for PDF conversion
	options := &ConvertOptions{
		Format: "pdf",
	}

	outputPath := tmp.Name() + ".pdf"
	defer os.Remove(outputPath)

	if err := ConvertWithFallback(context.Background(), tmp.Name(), outputPath, options); err != nil {
		return nil, fmt.Errorf("failed to convert SVG to PDF: %w", err)
	}

	return os.ReadFile(outputPath)
}

// ExtractSVGDimensions parses SVG content to extract width and height
func ExtractSVGDimensions(svgBytes []byte) (float64, float64, error) {
	svgContent := string(svgBytes)

	// Look for width and height attributes
	width, widthOk := extractAttribute(svgContent, "width")
	height, heightOk := extractAttribute(svgContent, "height")

	if widthOk && heightOk {
		return width, height, nil
	}

	// Look for viewBox attribute as fallback
	if viewBox := extractViewBox(svgContent); len(viewBox) == 4 {
		return viewBox[2], viewBox[3], nil // width and height from viewBox
	}

	return 0, 0, fmt.Errorf("could not extract SVG dimensions")
}

// extractAttribute extracts a numeric attribute value from SVG content
func extractAttribute(svgContent, attrName string) (float64, bool) {
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
func extractViewBox(svgContent string) []float64 {
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

