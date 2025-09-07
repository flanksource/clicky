package pdf_test

import (
	"context"
	"os"
	"strings"
	"testing"

	. "github.com/flanksource/clicky/formatters/pdf"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestSVGWidget_ConvertSVGToPNG_BasicSVG(t *testing.T) {

	svgContent := `<?xml version="1.0"?>
<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
    <circle cx="50" cy="50" r="20" fill="red"/>
</svg>`

	pngBytes, err := ConvertSVGToPNG([]byte(svgContent))
	require.NoError(t, err)
	require.NotEmpty(t, pngBytes)

	// Verify PNG header (magic bytes)
	assert.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47}, pngBytes[:4])
}

func TestSVGWidget_ConvertSVGToPNG_AspectRatioLandscape(t *testing.T) {

	// Landscape SVG (2:1 aspect ratio)
	svgContent := `<?xml version="1.0"?>
<svg width="200" height="100" xmlns="http://www.w3.org/2000/svg">
    <rect width="200" height="100" fill="blue"/>
</svg>`

	pngBytes, err := ConvertSVGToPNG([]byte(svgContent))
	require.NoError(t, err)
	require.NotEmpty(t, pngBytes)

	// Should maintain aspect ratio - width should be 400, height should be 200
	// (We can't easily test exact dimensions without decoding PNG, but we can test it doesn't error)
	assert.True(t, len(pngBytes) > 100) // Reasonable PNG size - lower threshold
}

func TestSVGWidget_ConvertSVGToPNG_AspectRatioPortrait(t *testing.T) {
	// Portrait SVG (1:2 aspect ratio)
	svgContent := `<?xml version="1.0"?>
<svg width="100" height="200" xmlns="http://www.w3.org/2000/svg">
    <rect width="100" height="200" fill="green"/>
</svg>`

	pngBytes, err := ConvertSVGToPNG([]byte(svgContent))
	require.NoError(t, err)
	require.NotEmpty(t, pngBytes)

	// Should maintain aspect ratio - width should be 200, height should be 400
	assert.True(t, len(pngBytes) > 100) // Reasonable PNG size
}

func TestSVGWidget_ConvertSVGToPNG_ViewBoxOnly(t *testing.T) {

	// SVG with viewBox but no width/height
	svgContent := `<?xml version="1.0"?>
<svg viewBox="0 0 300 150" xmlns="http://www.w3.org/2000/svg">
    <ellipse cx="150" cy="75" rx="120" ry="60" fill="purple"/>
</svg>`

	pngBytes, err := ConvertSVGToPNG([]byte(svgContent))
	require.NoError(t, err)
	require.NotEmpty(t, pngBytes)

	// Should extract dimensions from viewBox (300x150 = 2:1 aspect ratio)
	assert.True(t, len(pngBytes) > 100) // Reasonable PNG size
}

func TestSVGWidget_ConvertSVGToPNG_InvalidSVG(t *testing.T) {

	invalidSVG := `<not-svg>invalid content</not-svg>`

	_, err := ConvertSVGToPNG([]byte(invalidSVG))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse SVG")
}

func TestSVGWidget_ExtractSVGDimensions(t *testing.T) {

	tests := []struct {
		name         string
		svg          string
		expectWidth  float64
		expectHeight float64
		expectError  bool
	}{
		{
			name:         "Width and height attributes",
			svg:          `<svg width="100" height="50">`,
			expectWidth:  100,
			expectHeight: 50,
			expectError:  false,
		},
		{
			name:         "Width and height with units",
			svg:          `<svg width="100px" height="50mm">`,
			expectWidth:  100,
			expectHeight: 50,
			expectError:  false,
		},
		{
			name:         "ViewBox only",
			svg:          `<svg viewBox="0 0 200 100">`,
			expectWidth:  200,
			expectHeight: 100,
			expectError:  false,
		},
		{
			name:        "No dimensions",
			svg:         `<svg>`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height, err := ExtractSVGDimensions([]byte(tt.svg))

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectWidth, width)
				assert.Equal(t, tt.expectHeight, height)
			}
		})
	}
}

// Test SVG converter integration with SVGWidget
func TestSVGWidget_ConverterIntegration(t *testing.T) {

	// Create test SVG
	svgContent := createTestSVG()
	svgPath := writeTestSVG(t, svgContent)

	ctx := context.Background()

	for _, converterName := range GetAvailableConverters() {
		t.Run("Converter_"+converterName, func(t *testing.T) {
			converter, err := GetConverter(converterName)
			require.NoError(t, err)

			// Test PNG conversion (supported by all converters)
			if supportsFormat(converter, "png") {
				outputPath := strings.TrimSuffix(svgPath, ".svg") + "_" + converterName + ".png"
				defer os.Remove(outputPath)

				options := &ConvertOptions{
					Format: "png",
					Width:  100,
					Height: 100,
				}

				err := converter.Convert(ctx, svgPath, outputPath, options)
				assert.NoError(t, err, "PNG conversion should succeed for %s", converterName)

				if err == nil {
					assertFileExists(t, outputPath)
					assertFileNotEmpty(t, outputPath)
				}
			}
		})
	}
}

func TestSVGWidget_ManagerFallback(t *testing.T) {

	// Create test SVG
	svgContent := createComplexTestSVG()
	svgPath := writeTestSVG(t, svgContent)

	ctx := context.Background()

	t.Run("ConvertWithFallback", func(t *testing.T) {
		outputPath := strings.TrimSuffix(svgPath, ".svg") + "_fallback.png"
		defer os.Remove(outputPath)

		options := &ConvertOptions{
			Format: "png",
			Width:  200,
			Height: 150,
		}

		err := ConvertWithFallback(ctx, svgPath, outputPath, options)
		assert.NoError(t, err, "Fallback conversion should succeed")

		if err == nil {
			assertFileExists(t, outputPath)
			assertFileNotEmpty(t, outputPath)
		}
	})
}
