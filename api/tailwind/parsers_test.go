package tailwind

import (
	"testing"
)

// TestRegexBasedSpacingParser tests the refactored spacing parser using regex
func TestRegexBasedSpacingParser(t *testing.T) {
	tests := []struct {
		name           string
		class          string
		expectedTop    *float64
		expectedRight  *float64
		expectedBottom *float64
		expectedLeft   *float64
	}{
		{
			name:           "p-4 using regex",
			class:          "p-4",
			expectedTop:    floatPtr(1.0),
			expectedRight:  floatPtr(1.0),
			expectedBottom: floatPtr(1.0),
			expectedLeft:   floatPtr(1.0),
		},
		{
			name:           "px-8 using regex",
			class:          "px-8",
			expectedTop:    nil,
			expectedRight:  floatPtr(2.0),
			expectedBottom: nil,
			expectedLeft:   floatPtr(2.0),
		},
		{
			name:           "py-2 using regex",
			class:          "py-2",
			expectedTop:    floatPtr(0.5),
			expectedRight:  nil,
			expectedBottom: floatPtr(0.5),
			expectedLeft:   nil,
		},
		{
			name:           "invalid class",
			class:          "not-padding",
			expectedTop:    nil,
			expectedRight:  nil,
			expectedBottom: nil,
			expectedLeft:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			top, right, bottom, left := ParsePadding(tt.class)

			if !floatsEqual(top, tt.expectedTop) {
				t.Errorf("ParsePadding() top = %v, want %v", formatFloat(top), formatFloat(tt.expectedTop))
			}
			if !floatsEqual(right, tt.expectedRight) {
				t.Errorf("ParsePadding() right = %v, want %v", formatFloat(right), formatFloat(tt.expectedRight))
			}
			if !floatsEqual(bottom, tt.expectedBottom) {
				t.Errorf("ParsePadding() bottom = %v, want %v", formatFloat(bottom), formatFloat(tt.expectedBottom))
			}
			if !floatsEqual(left, tt.expectedLeft) {
				t.Errorf("ParsePadding() left = %v, want %v", formatFloat(left), formatFloat(tt.expectedLeft))
			}
		})
	}
}

// TestRegexBasedAlignmentParser tests the refactored alignment parser using regex
func TestRegexBasedAlignmentParser(t *testing.T) {
	tests := []struct {
		name               string
		class              string
		expectedHorizontal Alignment
		expectedVertical   VerticalAlign
	}{
		{
			name:               "text-left using regex",
			class:              "text-left",
			expectedHorizontal: Left,
			expectedVertical:   VerticalMiddle,
		},
		{
			name:               "text-center using regex",
			class:              "text-center",
			expectedHorizontal: Center,
			expectedVertical:   VerticalMiddle,
		},
		{
			name:               "text-right-top using regex",
			class:              "text-right-top",
			expectedHorizontal: Right,
			expectedVertical:   VerticalTop,
		},
		{
			name:               "text-center-bottom using regex",
			class:              "text-center-bottom",
			expectedHorizontal: Center,
			expectedVertical:   VerticalBottom,
		},
		{
			name:               "align-middle using regex",
			class:              "align-middle",
			expectedHorizontal: Left,
			expectedVertical:   VerticalMiddle,
		},
	}

	parser := NewAlignmentParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.parseAlignmentClass(tt.class)

			if result == nil {
				t.Fatalf("parseAlignmentClass() returned nil for class %s", tt.class)
			}

			if result.Horizontal != tt.expectedHorizontal {
				t.Errorf("parseAlignmentClass() horizontal = %v, want %v", result.Horizontal, tt.expectedHorizontal)
			}
			if result.Vertical != tt.expectedVertical {
				t.Errorf("parseAlignmentClass() vertical = %v, want %v", result.Vertical, tt.expectedVertical)
			}
		})
	}
}

// TestSliceBasedFontWeightDetection tests the refactored font weight detection using slices
func TestSliceBasedFontWeightDetection(t *testing.T) {
	tests := []struct {
		name          string
		styleString   string
		expectedBold  bool
		expectedFaint bool
	}{
		{
			name:          "bold detection using slices",
			styleString:   "bold text-red-500",
			expectedBold:  true,
			expectedFaint: false,
		},
		{
			name:          "font-semibold detection using slices",
			styleString:   "font-semibold bg-blue-200",
			expectedBold:  true,
			expectedFaint: false,
		},
		{
			name:          "font-light detection using slices",
			styleString:   "font-light text-gray-600",
			expectedBold:  false,
			expectedFaint: true,
		},
		{
			name:          "font-thin detection using slices",
			styleString:   "font-thin underline",
			expectedBold:  false,
			expectedFaint: true,
		},
		{
			name:          "no font weight",
			styleString:   "text-center bg-white",
			expectedBold:  false,
			expectedFaint: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := ParseStyle(tt.styleString)

			if style.Bold != tt.expectedBold {
				t.Errorf("ParseStyle() bold = %v, want %v", style.Bold, tt.expectedBold)
			}
			if style.Faint != tt.expectedFaint {
				t.Errorf("ParseStyle() faint = %v, want %v", style.Faint, tt.expectedFaint)
			}
		})
	}
}

// TestMapBasedBorderParsing tests the refactored border parsing using maps
func TestMapBasedBorderParsing(t *testing.T) {
	tests := []struct {
		name          string
		suffix        string
		expectedWidth float64
		expectedValid bool
	}{
		{
			name:          "border width 0 using map",
			suffix:        "0",
			expectedWidth: 0,
			expectedValid: true,
		},
		{
			name:          "border width 2 using map",
			suffix:        "2",
			expectedWidth: 2,
			expectedValid: true,
		},
		{
			name:          "border width 8 using map",
			suffix:        "8",
			expectedWidth: 8,
			expectedValid: true,
		},
		{
			name:          "invalid border width",
			suffix:        "invalid",
			expectedWidth: 0,
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, valid := parseBorderWidth(tt.suffix)

			if valid != tt.expectedValid {
				t.Errorf("parseBorderWidth() valid = %v, want %v", valid, tt.expectedValid)
			}
			if tt.expectedValid && width != tt.expectedWidth {
				t.Errorf("parseBorderWidth() width = %v, want %v", width, tt.expectedWidth)
			}
		})
	}

	// Test border styles
	styleTests := []struct {
		name          string
		suffix        string
		expectedStyle LineStyle
		expectedValid bool
	}{
		{
			name:          "solid style using map",
			suffix:        "solid",
			expectedStyle: Solid,
			expectedValid: true,
		},
		{
			name:          "dashed style using map",
			suffix:        "dashed",
			expectedStyle: Dashed,
			expectedValid: true,
		},
		{
			name:          "invalid style",
			suffix:        "invalid",
			expectedStyle: Solid,
			expectedValid: false,
		},
	}

	for _, tt := range styleTests {
		t.Run(tt.name, func(t *testing.T) {
			style, valid := parseBorderStyle(tt.suffix)

			if valid != tt.expectedValid {
				t.Errorf("parseBorderStyle() valid = %v, want %v", valid, tt.expectedValid)
			}
			if tt.expectedValid && style != tt.expectedStyle {
				t.Errorf("parseBorderStyle() style = %v, want %v", style, tt.expectedStyle)
			}
		})
	}
}

// TestOptimizedPropertyDetection tests the refactored property type detection
func TestOptimizedPropertyDetection(t *testing.T) {
	tests := []struct {
		name             string
		class            string
		expectedProperty string
	}{
		{
			name:             "width detection using helper",
			class:            "w-full",
			expectedProperty: "width",
		},
		{
			name:             "min-width detection using helper",
			class:            "min-w-0",
			expectedProperty: "width",
		},
		{
			name:             "font-weight detection using slice",
			class:            "font-bold",
			expectedProperty: "font-weight",
		},
		{
			name:             "font-size detection using slice",
			class:            "text-xl",
			expectedProperty: "font-size",
		},
		{
			name:             "display detection using slice",
			class:            "flex",
			expectedProperty: "display",
		},
		{
			name:             "position detection using slice",
			class:            "absolute",
			expectedProperty: "position",
		},
		{
			name:             "padding detection using helper",
			class:            "p-4",
			expectedProperty: "unique",
		},
	}

	merger := &StyleMerger{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			property := merger.getPropertyType(tt.class)

			if property != tt.expectedProperty {
				t.Errorf("getPropertyType() = %v, want %v", property, tt.expectedProperty)
			}
		})
	}
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func floatsEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func formatFloat(f *float64) string {
	if f == nil {
		return "nil"
	}
	return "float64(" + string(rune(int(*f))) + ")"
}
