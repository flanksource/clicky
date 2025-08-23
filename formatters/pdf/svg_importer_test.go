package pdf

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSVGImporter_ImportSVG_BasicElements(t *testing.T) {
	importer := NewSVGImporter()
	
	svgContent := `<?xml version="1.0"?>
<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
    <circle cx="50" cy="50" r="20" id="test-circle"/>
    <rect width="30" height="5" id="test-cut"/>
</svg>`
	
	elements, err := importer.ImportSVG(svgContent)
	require.NoError(t, err)
	require.NotNil(t, elements)
	
	// Check circles
	assert.Len(t, elements.Circles, 1)
	circle := elements.Circles[0]
	assert.Equal(t, 50.0, circle.X)
	assert.Equal(t, 50.0, circle.Y)
	assert.Equal(t, 40.0, circle.Diameter) // r=20 -> diameter=40
	assert.Equal(t, "test-circle", circle.Label)
	
	// Check cuts
	assert.Len(t, elements.Cuts, 1)
	cut := elements.Cuts[0]
	assert.Equal(t, "horizontal", cut.Orientation) // height < width
	assert.Equal(t, 50.0, cut.Position)           // default position
	assert.Equal(t, 5.0, cut.Width)               // min(width, height)
	assert.Equal(t, "test-cut", cut.Label)
	
	// No labels since rustyoz/svg doesn't support text elements
	assert.Len(t, elements.Labels, 0)
}

func TestSVGImporter_ImportSVG_Rectangles(t *testing.T) {
	importer := NewSVGImporter()
	
	svgContent := `<?xml version="1.0"?>
<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
    <rect width="5" height="80" id="vertical-cut"/>
    <rect width="80" height="5" id="horizontal-cut"/>
</svg>`
	
	elements, err := importer.ImportSVG(svgContent)
	require.NoError(t, err)
	
	// Should have 2 cuts (rustyoz/svg doesn't provide position info for edge cut detection)
	assert.Len(t, elements.Cuts, 2)
	
	// Check vertical cut (height > width)
	verticalCut := elements.Cuts[0]
	assert.Equal(t, "vertical", verticalCut.Orientation)
	assert.Equal(t, 5.0, verticalCut.Width) // min(width, height)
	assert.Equal(t, "vertical-cut", verticalCut.Label)
	
	// Check horizontal cut (width > height)
	horizontalCut := elements.Cuts[1]
	assert.Equal(t, "horizontal", horizontalCut.Orientation)
	assert.Equal(t, 5.0, horizontalCut.Width) // min(width, height)
	assert.Equal(t, "horizontal-cut", horizontalCut.Label)
}

func TestSVGImporter_ImportSVG_NestedGroups(t *testing.T) {
	importer := NewSVGImporter()
	
	svgContent := `<?xml version="1.0"?>
<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
    <g id="outer-group">
        <circle cx="25" cy="25" r="10" id="group-circle-1"/>
        <g id="inner-group">
            <circle cx="75" cy="75" r="15" id="group-circle-2"/>
        </g>
    </g>
</svg>`
	
	elements, err := importer.ImportSVG(svgContent)
	require.NoError(t, err)
	
	// Should extract elements from nested groups
	assert.Len(t, elements.Circles, 2)
	assert.Len(t, elements.Labels, 0) // No text support
	
	// Check circles
	circle1 := elements.Circles[0]
	assert.Equal(t, 25.0, circle1.X)
	assert.Equal(t, 25.0, circle1.Y)
	assert.Equal(t, 20.0, circle1.Diameter)
	assert.Equal(t, "group-circle-1", circle1.Label)
	
	circle2 := elements.Circles[1]
	assert.Equal(t, 75.0, circle2.X)
	assert.Equal(t, 75.0, circle2.Y)
	assert.Equal(t, 30.0, circle2.Diameter)
	assert.Equal(t, "group-circle-2", circle2.Label)
}

func TestSVGImporter_ImportSVG_InvalidSVG(t *testing.T) {
	importer := NewSVGImporter()
	
	invalidSVG := `<not-svg>invalid</not-svg>`
	
	_, err := importer.ImportSVG(invalidSVG)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse SVG")
}

func TestSVGImporter_ImportSVG_InvalidRectWidth(t *testing.T) {
	importer := NewSVGImporter()
	
	// Test invalid width
	invalidSVG := `<?xml version="1.0"?>
<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
    <rect width="invalid" height="20"/>
</svg>`
	
	_, err := importer.ImportSVG(invalidSVG)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid rect width")
}

func TestSVGImporter_ExtractLabelFromID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty ID", "", ""},
		{"Circle prefix", "circle-test", "test"},
		{"Rect prefix", "rect-cut1", "cut1"},
		{"Text prefix", "text-label", "label"},
		{"No prefix", "simple", "simple"},
		{"Multiple prefixes", "circle-rect-test", "test"}, // Removes all matching prefixes
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLabelFromID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSVGImporter_ParseFontSize(t *testing.T) {
	importer := NewSVGImporter()
	
	tests := []struct {
		name     string
		style    string
		expected float64
	}{
		{"Empty style", "", 12.0},
		{"Font size in pixels", "font-size:16px", 16.0},
		{"Font size without px", "font-size:14", 14.0},
		{"Font size with other styles", "color:red;font-size:18px;font-weight:bold", 18.0},
		{"Invalid font size", "font-size:invalid", 12.0},
		{"No font size", "color:blue;font-weight:normal", 12.0},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := importer.parseFontSize(tt.style)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSVGBox_ImportFromSVG(t *testing.T) {
	svgBox := &SVGBox{
		Circles:  []CircleShape{{X: 10, Y: 10, Diameter: 5}}, // Pre-existing circle
		Labels:   []Label{},
		Cuts:     []Cut{},
		EdgeCuts: []EdgeCut{},
	}
	
	svgContent := `<?xml version="1.0"?>
<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
    <circle cx="50" cy="50" r="20"/>
    <rect width="20" height="5" id="test-cut"/>
</svg>`
	
	err := svgBox.ImportFromSVG(svgContent)
	require.NoError(t, err)
	
	// Should append to existing elements
	assert.Len(t, svgBox.Circles, 2) // 1 existing + 1 imported
	assert.Len(t, svgBox.Cuts, 1)    // 0 existing + 1 imported
	assert.Len(t, svgBox.Labels, 0)  // No text support
	
	// Check the imported circle
	importedCircle := svgBox.Circles[1] // Second circle
	assert.Equal(t, 50.0, importedCircle.X)
	assert.Equal(t, 50.0, importedCircle.Y)
	assert.Equal(t, 40.0, importedCircle.Diameter)
	
	// Check the imported cut
	importedCut := svgBox.Cuts[0]
	assert.Equal(t, "horizontal", importedCut.Orientation) // width > height
	assert.Equal(t, "test-cut", importedCut.Label)
}

func TestSVGBox_ImportFromSVG_InvalidContent(t *testing.T) {
	svgBox := &SVGBox{}
	
	invalidSVG := `<invalid>content</invalid>`
	
	err := svgBox.ImportFromSVG(invalidSVG)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to import SVG")
}

// Test SVG importer with converter integration
func TestSVGImporter_WithConverterExport(t *testing.T) {
	manager := NewSVGConverterManager()
	
	if len(manager.GetAvailableConverters()) == 0 {
		t.Skip("No converters available for integration testing")
	}
	
	importer := NewSVGImporter()
	
	// Create complex test SVG
	svgContent := CreateComplexTestSVG()
	
	t.Run("ImportAndConvert", func(t *testing.T) {
		// First test importing the SVG content
		elements, err := importer.ImportSVG(svgContent)
		require.NoError(t, err)
		require.NotNil(t, elements)
		
		// Should have imported various elements
		t.Logf("Imported elements: %d circles, %d cuts, %d labels", 
			len(elements.Circles), len(elements.Cuts), len(elements.Labels))
		
		// Now test converting the SVG to different formats
		svgPath := WriteTestSVG(t, svgContent)
		ctx := context.Background()
		
		for _, format := range []string{"png", "pdf"} {
			outputPath := strings.TrimSuffix(svgPath, ".svg") + "_imported." + format
			defer os.Remove(outputPath)
			
			options := &ConvertOptions{
				Format: format,
				Width:  200,
				Height: 150,
			}
			
			err := manager.ConvertWithFallback(ctx, svgPath, outputPath, options)
			if err != nil {
				t.Logf("Conversion to %s failed (this may be expected): %v", format, err)
				continue
			}
			
			AssertFileExists(t, outputPath)
			AssertFileNotEmpty(t, outputPath)
			t.Logf("Successfully converted imported SVG to %s", format)
		}
	})
}

func TestSVGImporter_ProcessingPipeline(t *testing.T) {
	manager := NewSVGConverterManager()
	importer := NewSVGImporter()
	
	if len(manager.GetAvailableConverters()) == 0 {
		t.Skip("No converters available")
	}
	
	// Test the full pipeline: Create SVG -> Import Elements -> Convert to PNG
	t.Run("FullPipeline", func(t *testing.T) {
		// Create test SVG with specific elements
		svgContent := `<svg width="150" height="100" xmlns="http://www.w3.org/2000/svg">
  <circle cx="30" cy="30" r="20" fill="blue" id="hole-1"/>
  <circle cx="120" cy="30" r="15" fill="red" id="hole-2"/>
  <rect x="10" y="70" width="130" height="5" fill="brown" id="cut-1"/>
  <rect x="65" y="10" width="5" height="80" fill="brown" id="cut-2"/>
</svg>`
		
		// Step 1: Import SVG elements
		elements, err := importer.ImportSVG(svgContent)
		require.NoError(t, err)
		
		// Verify imported elements
		assert.Len(t, elements.Circles, 2, "Should import 2 circles")
		assert.Len(t, elements.Cuts, 2, "Should import 2 cuts")
		
		// Step 2: Convert SVG to PNG
		svgPath := WriteTestSVG(t, svgContent)
		outputPath := strings.TrimSuffix(svgPath, ".svg") + "_pipeline.png"
		defer os.Remove(outputPath)
		
		ctx := context.Background()
		options := &ConvertOptions{
			Format: "png",
			Width:  150,
			Height: 100,
		}
		
		err = manager.Convert(ctx, svgPath, outputPath, options)
		assert.NoError(t, err, "Pipeline conversion should succeed")
		
		if err == nil {
			AssertFileExists(t, outputPath)
			AssertFileNotEmpty(t, outputPath)
		}
		
		t.Logf("Pipeline test completed: imported %d elements and converted to PNG", 
			len(elements.Circles)+len(elements.Cuts))
	})
}