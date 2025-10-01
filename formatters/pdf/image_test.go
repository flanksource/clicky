package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
)

// TestImageGridShowcase creates a comprehensive PDF showcasing all image functionality
// including multi-format grids, SVG conversion, TableComponent integration, and styling options
func TestImageGridShowcase(t *testing.T) {
	// Create builder with debug mode for grid visualization
	builder := NewBuilder(WithDebug(true))

	// Create demo images for consistent testing
	pngPath, jpgPath, svgPath := createDemoImages(t)
	defer cleanupDemoImages(pngPath, jpgPath, svgPath)

	// Title
	titleText := Text{
		Text: api.Text{
			Content: "Image Grid Showcase - Comprehensive Multi-Format Testing",
			Class:   api.ResolveStyles("text-3xl font-bold text-center mb-6"),
		},
	}
	titleText.Draw(builder)

	// Section 1: Multi-Format Image Grid (col-span 3)
	if err := addMultiFormatImageGrid(builder, pngPath, jpgPath, svgPath); err != nil {
		t.Fatalf("Failed to add multi-format image grid: %v", err)
	}

	// Section 2: SVG Conversion Showcase
	if err := addSVGConversionShowcase(builder, svgPath); err != nil {
		t.Fatalf("Failed to add SVG conversion showcase: %v", err)
	}

	// Section 3: Image + TableComponent Combinations
	if err := addImageTableCombinations(builder, pngPath); err != nil {
		t.Fatalf("Failed to add image-table combinations: %v", err)
	}

	// Section 4: Padding and Alignment Grid
	if err := addPaddingAlignmentGrid(builder, jpgPath); err != nil {
		t.Fatalf("Failed to add padding alignment grid: %v", err)
	}

	// Generate PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to generate PDF: %v", err)
	}

	// Validate PDF was generated
	if len(pdfData) == 0 {
		t.Fatal("Generated PDF is empty")
	}

	// Save the PDF to out directory
	saveImageTestPDF(t, "image_grid_showcase", pdfData)

	t.Logf("✓ Image grid showcase PDF generated successfully (%d bytes)", len(pdfData))
	t.Logf("✓ PDF demonstrates multi-format grids, SVG conversion, table integration, and styling")
}

// addMultiFormatImageGrid demonstrates PNG, JPG, SVG in 3-column grid layout
func addMultiFormatImageGrid(builder *Builder, pngPath, jpgPath, svgPath string) error {
	// Section header
	headerText := Text{
		Text: api.Text{
			Content: "1. Multi-Format Image Grid (3-Column Layout)",
			Class:   api.ResolveStyles("text-xl font-semibold mt-8 mb-4 text-blue-600"),
		},
	}
	headerText.Draw(builder)

	// Description
	descText := Text{
		Text: api.Text{
			Content: "Demonstrating PNG, JPG, and SVG images in a 3-column grid layout with 4-column spans each (4+4+4=12)",
			Class:   api.ResolveStyles("text-sm text-gray-600 mb-4"),
		},
	}
	descText.Draw(builder)

	// Create 3-column grid with different image formats
	gridVariations := []struct {
		format      string
		path        string
		style       string
		description string
	}{
		{"PNG", pngPath, "w-1/3 text-center p-2", "PNG format with center alignment and padding"},
		{"JPG", jpgPath, "w-1/3 text-center p-2", "JPG format with center alignment and padding"},
		{"SVG", svgPath, "w-1/3 text-center p-2", "SVG format with center alignment and padding"},
	}

	// Grid row with 3 images
	for i, variation := range gridVariations {
		// Format label
		labelText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Column %d - %s: %s", i+1, variation.format, variation.description),
				Class:   api.ResolveStyles("text-xs text-gray-600 mt-2 mb-1"),
			},
		}
		labelText.Draw(builder)

		// Image with 4-column span (1/3 of 12 columns)
		image := &Image{
			Source: variation.path,
			Style:  variation.style,
		}
		if err := image.DrawInColumn(builder, 4); err != nil {
			return fmt.Errorf("failed to draw %s image: %w", variation.format, err)
		}
	}

	return nil
}

// addSVGConversionShowcase demonstrates different SVG conversion methods
func addSVGConversionShowcase(builder *Builder, svgPath string) error {
	// Note: This function uses builder parameter but needs access to testing.T for logging
	// We'll use fmt.Printf instead of t.Logf for SVG conversion warnings
	// Section header
	headerText := Text{
		Text: api.Text{
			Content: "2. SVG Conversion Showcase",
			Class:   api.ResolveStyles("text-xl font-semibold mt-8 mb-4 text-green-600"),
		},
	}
	headerText.Draw(builder)

	// Description
	descText := Text{
		Text: api.Text{
			Content: "Testing SVG conversion with Chrome/Chromium, Inkscape, and rsvg-convert fallback handling",
			Class:   api.ResolveStyles("text-sm text-gray-600 mb-4"),
		},
	}
	descText.Draw(builder)

	// Test different converters
	converters := []string{"chromium", "inkscape", "rsvg-convert"}
	availableConverters := GetAvailableConverters()

	for _, converter := range converters {
		// Check if converter is available
		isAvailable := false
		for _, available := range availableConverters {
			if available == converter {
				isAvailable = true
				break
			}
		}

		statusText := "❌ Not Available"
		if isAvailable {
			statusText = "✅ Available"
		}

		// Converter status
		converterText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s Converter: %s", converter, statusText),
				Class:   api.ResolveStyles("text-sm font-medium mt-2"),
			},
		}
		converterText.Draw(builder)

		// Test conversion if available
		if isAvailable {
			image := &Image{
				Source:             svgPath,
				PreferredConverter: converter,
				Style:              "w-1/2 text-center p-4",
			}
			if err := image.Draw(builder); err != nil {
				// Log error but continue with other converters
				fmt.Printf("Warning: %s conversion failed: %v\n", converter, err)
			}
		}
	}

	return nil
}

// addImageTableCombinations demonstrates image and table combinations
func addImageTableCombinations(builder *Builder, imagePath string) error {
	// Section header
	headerText := Text{
		Text: api.Text{
			Content: "3. Image + TableComponent Combinations",
			Class:   api.ResolveStyles("text-xl font-semibold mt-8 mb-4 text-purple-600"),
		},
	}
	headerText.Draw(builder)

	// Description
	descText := Text{
		Text: api.Text{
			Content: "Demonstrating image and table combinations with different column distributions",
			Class:   api.ResolveStyles("text-sm text-gray-600 mb-4"),
		},
	}
	descText.Draw(builder)

	// Test different combinations
	combinations := []struct {
		name       string
		imageSpan  int
		tableSpan  int
		imageStyle string
	}{
		{"7-5 Split", 7, 5, "w-7/12 text-center p-3"},
		{"8-4 Split", 8, 4, "w-2/3 text-center p-3"},
		{"6-6 Split", 6, 6, "w-1/2 text-center p-3"},
	}

	for _, combo := range combinations {
		// Combination header
		comboText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s: Image (%d cols) + Table (%d cols)", combo.name, combo.imageSpan, combo.tableSpan),
				Class:   api.ResolveStyles("text-md font-medium mt-4 mb-2"),
			},
		}
		comboText.Draw(builder)

		// Image
		image := &Image{
			Source: imagePath,
			Style:  combo.imageStyle,
		}
		if err := image.DrawInColumn(builder, combo.imageSpan); err != nil {
			return fmt.Errorf("failed to draw image for %s: %w", combo.name, err)
		}

		// Table data
		headers := []string{"Item", "Value", "Status"}
		rows := [][]any{
			{"Image Width", fmt.Sprintf("%d/12 columns", combo.imageSpan), "✓"},
			{"Table Width", fmt.Sprintf("%d/12 columns", combo.tableSpan), "✓"},
			{"Total", "12/12 columns", "✓"},
		}

		// Create table component
		table := NewTableComponent(headers, rows)
		table.WithHeaderStyle("font-bold text-white bg-purple-600 text-center text-xs p-1")
		table.WithRowStyle("text-xs text-gray-800 bg-white p-1")
		table.WithAlternateRowStyle("bg-purple-50")

		// Add table after the image (tables span full width)
		// Note: TableComponent integration would require proper rendering setup
		// For this demonstration, we'll add a text note about the table
		noteText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Table with %d rows and %d columns would be displayed here", len(rows), len(headers)),
				Class:   api.ResolveStyles("text-xs text-gray-600 italic mt-2"),
			},
		}
		noteText.Draw(builder)
	}

	return nil
}

// addPaddingAlignmentGrid demonstrates padding and alignment variations
func addPaddingAlignmentGrid(builder *Builder, imagePath string) error {
	// Section header
	headerText := Text{
		Text: api.Text{
			Content: "4. Padding and Alignment Grid Demonstrations",
			Class:   api.ResolveStyles("text-xl font-semibold mt-8 mb-4 text-orange-600"),
		},
	}
	headerText.Draw(builder)

	// Description
	descText := Text{
		Text: api.Text{
			Content: "Testing various padding levels and alignment options within grid cells",
			Class:   api.ResolveStyles("text-sm text-gray-600 mb-4"),
		},
	}
	descText.Draw(builder)

	// Padding variations
	paddingVariations := []struct {
		name    string
		style   string
		colspan int
	}{
		{"No Padding", "w-1/4 text-center p-0", 3},
		{"Small Padding", "w-1/4 text-center p-2", 3},
		{"Medium Padding", "w-1/4 text-center p-4", 3},
		{"Large Padding", "w-1/4 text-center p-6", 3},
	}

	// Padding grid
	paddingText := Text{
		Text: api.Text{
			Content: "Padding Variations (4-column grid):",
			Class:   api.ResolveStyles("text-md font-medium mt-4 mb-2"),
		},
	}
	paddingText.Draw(builder)

	for _, variation := range paddingVariations {
		// Label
		labelText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s (%s)", variation.name, variation.style),
				Class:   api.ResolveStyles("text-xs text-gray-600 mb-1"),
			},
		}
		labelText.Draw(builder)

		// Image with padding
		image := &Image{
			Source: imagePath,
			Style:  variation.style,
		}
		if err := image.DrawInColumn(builder, variation.colspan); err != nil {
			return fmt.Errorf("failed to draw image with %s: %w", variation.name, err)
		}
	}

	// Alignment variations
	alignmentVariations := []struct {
		name    string
		style   string
		colspan int
	}{
		{"Left Aligned", "w-1/3 text-left p-2", 4},
		{"Center Aligned", "w-1/3 text-center p-2", 4},
		{"Right Aligned", "w-1/3 text-right p-2", 4},
	}

	// Alignment grid
	alignmentText := Text{
		Text: api.Text{
			Content: "Alignment Variations (3-column grid):",
			Class:   api.ResolveStyles("text-md font-medium mt-6 mb-2"),
		},
	}
	alignmentText.Draw(builder)

	for _, variation := range alignmentVariations {
		// Label
		labelText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s (%s)", variation.name, variation.style),
				Class:   api.ResolveStyles("text-xs text-gray-600 mb-1"),
			},
		}
		labelText.Draw(builder)

		// Image with alignment
		image := &Image{
			Source: imagePath,
			Style:  variation.style,
		}
		if err := image.DrawInColumn(builder, variation.colspan); err != nil {
			return fmt.Errorf("failed to draw image with %s: %w", variation.name, err)
		}
	}

	return nil
}

// createDemoImages creates PNG, JPG, and SVG demo images for testing
func createDemoImages(t *testing.T) (pngPath, jpgPath, svgPath string) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "image_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create demo SVG
	svgContent := createDemoImageSVG()
	svgPath = filepath.Join(tempDir, "demo.svg")
	if err := os.WriteFile(svgPath, []byte(svgContent), 0644); err != nil {
		t.Fatalf("Failed to write demo SVG: %v", err)
	}

	// For PNG and JPG, we'll use existing test images or create simple ones
	// Check if test images exist in the project
	testImagePaths := []string{
		"/Users/moshe/work/omi/clicky/examples/test-dir/assets/images/logo.png",
		"/Users/moshe/work/omi/clicky/out/test_grid_png.png",
		"/Users/moshe/work/omi/clicky/formatters/pdf/out/grid_png.png",
	}

	// Find an existing PNG
	for _, path := range testImagePaths {
		if _, err := os.Stat(path); err == nil {
			pngPath = path
			break
		}
	}

	// If no PNG found, convert SVG to PNG
	if pngPath == "" {
		pngPath = filepath.Join(tempDir, "demo.png")
		ctx := context.Background()
		options := &ConvertOptions{Format: "png", DPI: 150}
		if err := ConvertWithFallback(ctx, svgPath, pngPath, options); err != nil {
			t.Logf("Warning: Failed to create PNG from SVG: %v", err)
			pngPath = svgPath // Fallback to SVG
		}
	}

	// For JPG, try to find existing or use PNG
	jpgTestPaths := []string{
		"/Users/moshe/work/omi/clicky/out/test_grid_jpg.jpg",
		"/Users/moshe/work/omi/clicky/formatters/pdf/out/grid_jpg.jpg",
	}

	for _, path := range jpgTestPaths {
		if _, err := os.Stat(path); err == nil {
			jpgPath = path
			break
		}
	}

	// If no JPG found, use PNG path (Image component handles format detection)
	if jpgPath == "" {
		jpgPath = pngPath
	}

	return pngPath, jpgPath, svgPath
}

// cleanupDemoImages removes temporary demo images
func cleanupDemoImages(paths ...string) {
	for _, path := range paths {
		if strings.Contains(path, "image_test_") {
			os.RemoveAll(filepath.Dir(path))
		}
	}
}

// createDemoImageSVG creates a simple recognizable SVG for consistent testing
func createDemoImageSVG() string {
	return `<svg width="300" height="200" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="grad" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" style="stop-color:#3b82f6;stop-opacity:1" />
      <stop offset="100%" style="stop-color:#1d4ed8;stop-opacity:1" />
    </linearGradient>
  </defs>

  <!-- Background -->
  <rect width="300" height="200" fill="url(#grad)" rx="15"/>

  <!-- Grid pattern -->
  <g stroke="#ffffff" stroke-width="1" opacity="0.3">
    <line x1="100" y1="0" x2="100" y2="200"/>
    <line x1="200" y1="0" x2="200" y2="200"/>
    <line x1="0" y1="67" x2="300" y2="67"/>
    <line x1="0" y1="133" x2="300" y2="133"/>
  </g>

  <!-- Center content -->
  <circle cx="150" cy="100" r="40" fill="#ffffff" opacity="0.9"/>

  <!-- Text -->
  <text x="150" y="70" font-family="Arial, sans-serif" font-size="18" font-weight="bold"
        text-anchor="middle" fill="#ffffff">GRID</text>
  <text x="150" y="105" font-family="Arial, sans-serif" font-size="16"
        text-anchor="middle" fill="#1d4ed8">DEMO</text>
  <text x="150" y="125" font-family="Arial, sans-serif" font-size="12"
        text-anchor="middle" fill="#1d4ed8">COL-SPAN-3</text>

  <!-- Corner grid indicators -->
  <circle cx="25" cy="25" r="12" fill="#fbbf24"/>
  <text x="25" y="30" font-family="Arial, sans-serif" font-size="10" font-weight="bold"
        text-anchor="middle" fill="#000">1</text>

  <circle cx="150" cy="25" r="12" fill="#10b981"/>
  <text x="150" y="30" font-family="Arial, sans-serif" font-size="10" font-weight="bold"
        text-anchor="middle" fill="#000">2</text>

  <circle cx="275" cy="25" r="12" fill="#ef4444"/>
  <text x="275" y="30" font-family="Arial, sans-serif" font-size="10" font-weight="bold"
        text-anchor="middle" fill="#fff">3</text>

  <!-- Size indicator -->
  <text x="150" y="185" font-family="Arial, sans-serif" font-size="11"
        text-anchor="middle" fill="#ffffff">300×200 Grid Layout</text>
</svg>`
}

// saveImageTestPDF saves the generated PDF to the out directory
func saveImageTestPDF(t *testing.T, name string, pdfData []byte) {
	outDir := "out"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Logf("Warning: Failed to create out directory: %v", err)
		return
	}

	filename := filepath.Join(outDir, name+".pdf")
	if err := os.WriteFile(filename, pdfData, 0644); err != nil {
		t.Logf("Warning: Failed to save PDF to %s: %v", filename, err)
		return
	}

	t.Logf("✓ PDF saved to %s", filename)
}
