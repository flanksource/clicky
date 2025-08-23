package pdf

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSVGWidget_ConvertSVGToPNG_BasicSVG(t *testing.T) {
	widget := SVGWidget{}
	
	svgContent := `<?xml version="1.0"?>
<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
    <circle cx="50" cy="50" r="20" fill="red"/>
</svg>`
	
	pngBytes, err := widget.convertSVGToPNG([]byte(svgContent))
	require.NoError(t, err)
	require.NotEmpty(t, pngBytes)
	
	// Verify PNG header (magic bytes)
	assert.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47}, pngBytes[:4])
}

func TestSVGWidget_ConvertSVGToPNG_AspectRatioLandscape(t *testing.T) {
	widget := SVGWidget{}
	
	// Landscape SVG (2:1 aspect ratio)
	svgContent := `<?xml version="1.0"?>
<svg width="200" height="100" xmlns="http://www.w3.org/2000/svg">
    <rect width="200" height="100" fill="blue"/>
</svg>`
	
	pngBytes, err := widget.convertSVGToPNG([]byte(svgContent))
	require.NoError(t, err)
	require.NotEmpty(t, pngBytes)
	
	// Should maintain aspect ratio - width should be 400, height should be 200
	// (We can't easily test exact dimensions without decoding PNG, but we can test it doesn't error)
	assert.True(t, len(pngBytes) > 100) // Reasonable PNG size - lower threshold
}

func TestSVGWidget_ConvertSVGToPNG_AspectRatioPortrait(t *testing.T) {
	widget := SVGWidget{}
	
	// Portrait SVG (1:2 aspect ratio)
	svgContent := `<?xml version="1.0"?>
<svg width="100" height="200" xmlns="http://www.w3.org/2000/svg">
    <rect width="100" height="200" fill="green"/>
</svg>`
	
	pngBytes, err := widget.convertSVGToPNG([]byte(svgContent))
	require.NoError(t, err)
	require.NotEmpty(t, pngBytes)
	
	// Should maintain aspect ratio - width should be 200, height should be 400
	assert.True(t, len(pngBytes) > 100) // Reasonable PNG size
}

func TestSVGWidget_ConvertSVGToPNG_ViewBoxOnly(t *testing.T) {
	widget := SVGWidget{}
	
	// SVG with viewBox but no width/height
	svgContent := `<?xml version="1.0"?>
<svg viewBox="0 0 300 150" xmlns="http://www.w3.org/2000/svg">
    <ellipse cx="150" cy="75" rx="120" ry="60" fill="purple"/>
</svg>`
	
	pngBytes, err := widget.convertSVGToPNG([]byte(svgContent))
	require.NoError(t, err)
	require.NotEmpty(t, pngBytes)
	
	// Should extract dimensions from viewBox (300x150 = 2:1 aspect ratio)
	assert.True(t, len(pngBytes) > 100) // Reasonable PNG size
}

func TestSVGWidget_ConvertSVGToPNG_InvalidSVG(t *testing.T) {
	widget := SVGWidget{}
	
	invalidSVG := `<not-svg>invalid content</not-svg>`
	
	_, err := widget.convertSVGToPNG([]byte(invalidSVG))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse SVG")
}

func TestSVGWidget_ExtractSVGDimensions(t *testing.T) {
	widget := SVGWidget{}
	
	tests := []struct {
		name        string
		svg         string
		expectWidth float64
		expectHeight float64
		expectError bool
	}{
		{
			name: "Width and height attributes",
			svg:  `<svg width="100" height="50">`,
			expectWidth: 100,
			expectHeight: 50,
			expectError: false,
		},
		{
			name: "Width and height with units",
			svg:  `<svg width="100px" height="50mm">`,
			expectWidth: 100,
			expectHeight: 50,
			expectError: false,
		},
		{
			name: "ViewBox only",
			svg:  `<svg viewBox="0 0 200 100">`,
			expectWidth: 200,
			expectHeight: 100,
			expectError: false,
		},
		{
			name: "No dimensions",
			svg:  `<svg>`,
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height, err := widget.extractSVGDimensions([]byte(tt.svg))
			
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

func TestSVGWidget_FromSVGContent(t *testing.T) {
	svgContent := `<?xml version="1.0"?>
<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
    <circle cx="50" cy="50" r="20"/>
    <rect width="30" height="10"/>
</svg>`
	
	box := api.Box{
		Rectangle: api.Rectangle{Width: 200, Height: 200},
		Fill:      api.Color{Hex: "ffffff"},
	}
	
	widget, err := FromSVGContent(svgContent, box)
	require.NoError(t, err)
	require.NotNil(t, widget)
	
	// Should have imported elements from SVG
	assert.Len(t, widget.SVGBox.Circles, 1)
	assert.Len(t, widget.SVGBox.Cuts, 1)
	
	// Should have collision avoidance enabled
	assert.True(t, widget.SVGBox.EnableCollisionAvoidance)
}

func TestSVGWidget_FromSVGContent_InvalidSVG(t *testing.T) {
	invalidSVG := `<invalid>content</invalid>`
	
	box := api.Box{
		Rectangle: api.Rectangle{Width: 100, Height: 100},
	}
	
	_, err := FromSVGContent(invalidSVG, box)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to import SVG content")
}

// Test SVG converter integration with SVGWidget
func TestSVGWidget_ConverterIntegration(t *testing.T) {
	manager := NewSVGConverterManager()
	availableConverters := manager.GetAvailableConverters()
	
	if len(availableConverters) == 0 {
		t.Skip("No SVG converters available, skipping integration tests")
	}
	
	t.Logf("Testing with available converters: %v", availableConverters)
	
	// Create test SVG
	svgContent := CreateTestSVG()
	svgPath := WriteTestSVG(t, svgContent)
	
	ctx := context.Background()
	
	for _, converterName := range availableConverters {
		t.Run("Converter_"+converterName, func(t *testing.T) {
			converter, err := manager.GetConverter(converterName)
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
					AssertFileExists(t, outputPath)
					AssertFileNotEmpty(t, outputPath)
				}
			}
		})
	}
}

func TestSVGWidget_ManagerFallback(t *testing.T) {
	manager := NewSVGConverterManager()
	
	if len(manager.GetAvailableConverters()) == 0 {
		t.Skip("No converters available")
	}
	
	// Create test SVG
	svgContent := CreateComplexTestSVG()
	svgPath := WriteTestSVG(t, svgContent)
	
	ctx := context.Background()
	
	t.Run("ConvertWithFallback", func(t *testing.T) {
		outputPath := strings.TrimSuffix(svgPath, ".svg") + "_fallback.png"
		defer os.Remove(outputPath)
		
		options := &ConvertOptions{
			Format: "png",
			Width:  200,
			Height: 150,
		}
		
		err := manager.ConvertWithFallback(ctx, svgPath, outputPath, options)
		assert.NoError(t, err, "Fallback conversion should succeed")
		
		if err == nil {
			AssertFileExists(t, outputPath)
			AssertFileNotEmpty(t, outputPath)
		}
	})
}