package tailwind

import (
	"testing"
)

func TestMergeStyles(t *testing.T) {
	tests := []struct {
		name     string
		styles   []string
		expected string
	}{
		{
			name:     "single style",
			styles:   []string{"text-center font-bold"},
			expected: "text-center font-bold",
		},
		{
			name:     "two styles no conflicts",
			styles:   []string{"text-center", "font-bold"},
			expected: "text-center font-bold",
		},
		{
			name:     "text alignment conflict - last wins",
			styles:   []string{"text-left", "text-center"},
			expected: "text-center",
		},
		{
			name:     "font weight conflict - last wins",
			styles:   []string{"font-normal", "font-bold"},
			expected: "font-bold",
		},
		{
			name:     "color conflict - last wins",
			styles:   []string{"text-red-500", "text-blue-600"},
			expected: "text-blue-600",
		},
		{
			name:     "background conflict - last wins",
			styles:   []string{"bg-gray-100", "bg-white"},
			expected: "bg-white",
		},
		{
			name:     "complex merge with multiple conflicts",
			styles:   []string{"text-left font-normal bg-gray-100", "text-center font-bold", "text-right bg-white"},
			expected: "text-right font-bold bg-white",
		},
		{
			name:     "margin and padding - no conflicts",
			styles:   []string{"m-4 p-2", "mx-2 py-4"},
			expected: "m-4 p-2 mx-2 py-4",
		},
		{
			name:     "width specifications - last wins",
			styles:   []string{"w-1/2", "w-[200px]"},
			expected: "w-[200px]",
		},
		{
			name:     "empty strings ignored",
			styles:   []string{"", "text-center", "", "font-bold", ""},
			expected: "text-center font-bold",
		},
		{
			name:     "duplicate classes removed",
			styles:   []string{"text-center font-bold", "text-center bg-gray-100"},
			expected: "text-center font-bold bg-gray-100",
		},
		{
			name:     "alignment with vertical positioning",
			styles:   []string{"align-top", "align-middle"},
			expected: "align-middle",
		},
		{
			name:     "mixed alignment and text alignment",
			styles:   []string{"text-left align-top", "text-center align-middle"},
			expected: "text-center align-middle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeStyles(tt.styles...)
			if result != tt.expected {
				t.Errorf("MergeStyles() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestStyleMergerCategorizeClass(t *testing.T) {
	merger := &StyleMerger{}

	tests := []struct {
		name     string
		class    string
		expected string
	}{
		// Text alignment
		{"left align", "text-left", "text-align"},
		{"center align", "text-center", "text-align"},
		{"right align", "text-right", "text-align"},
		{"justify align", "text-justify", "text-align"},

		// Font weight
		{"normal weight", "font-normal", "font-weight"},
		{"bold weight", "font-bold", "font-weight"},
		{"light weight", "font-light", "font-weight"},
		{"semibold weight", "font-semibold", "font-weight"},

		// Font style
		{"italic", "italic", "font-style"},
		{"not italic", "not-italic", "font-style"},

		// Text color
		{"red text", "text-red-500", "text-color"},
		{"blue text", "text-blue-600", "text-color"},
		{"gray text", "text-gray-700", "text-color"},

		// Background color
		{"gray background", "bg-gray-100", "background-color"},
		{"white background", "bg-white", "background-color"},
		{"blue background", "bg-blue-500", "background-color"},

		// Font size
		{"text xs", "text-xs", "font-size"},
		{"text sm", "text-sm", "font-size"},
		{"text base", "text-base", "font-size"},
		{"text lg", "text-lg", "font-size"},
		{"text xl", "text-xl", "font-size"},

		// Width
		{"width 1/2", "w-1/2", "width"},
		{"width full", "w-full", "width"},
		{"width custom", "w-[200px]", "width"},
		{"min width", "min-w-[100px]", "width"},
		{"max width", "max-w-[300px]", "width"},

		// Vertical alignment
		{"align top", "align-top", "vertical-align"},
		{"align middle", "align-middle", "vertical-align"},
		{"align bottom", "align-bottom", "vertical-align"},

		// Uncategorized (treated as unique/non-conflicting)
		{"margin", "m-4", "unique"},
		{"padding", "p-2", "unique"},
		{"border", "border", "unique"},
		{"custom class", "my-custom-class", "unique"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := merger.getPropertyType(tt.class)
			if result != tt.expected {
				t.Errorf("categorizeClass(%q) = %q, want %q", tt.class, result, tt.expected)
			}
		})
	}
}

func TestStyleMergerMergeStyles(t *testing.T) {
	merger := &StyleMerger{}

	tests := []struct {
		name     string
		styles   []string
		expected string
	}{
		{
			name:     "preserves order for non-conflicting classes",
			styles:   []string{"p-4 m-2", "border rounded"},
			expected: "p-4 m-2 border rounded",
		},
		{
			name:     "resolves text alignment conflicts",
			styles:   []string{"text-left p-4", "text-center m-2"},
			expected: "p-4 m-2 text-center",
		},
		{
			name:     "resolves font weight conflicts",
			styles:   []string{"font-normal text-lg", "font-bold text-sm"},
			expected: "font-bold text-sm",
		},
		{
			name:     "handles multiple property conflicts",
			styles:   []string{"text-left font-normal bg-gray-100", "text-center font-bold bg-white"},
			expected: "text-center font-bold bg-white",
		},
		{
			name:     "preserves uncategorized classes",
			styles:   []string{"custom-class-1", "custom-class-2"},
			expected: "custom-class-1 custom-class-2",
		},
		{
			name:     "removes duplicate classes",
			styles:   []string{"p-4 text-center", "p-4 font-bold"},
			expected: "p-4 text-center font-bold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := merger.MergeStyles(tt.styles...)
			if result != tt.expected {
				t.Errorf("MergeStyles() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDefaultStyleMerger(t *testing.T) {
	// Test that the default style merger is properly initialized
	result := MergeStyles("text-left", "text-center")
	expected := "text-center"

	if result != expected {
		t.Errorf("Default MergeStyles() = %q, want %q", result, expected)
	}
}

func TestEmptyAndWhitespaceHandling(t *testing.T) {
	tests := []struct {
		name     string
		styles   []string
		expected string
	}{
		{
			name:     "all empty strings",
			styles:   []string{"", "", ""},
			expected: "",
		},
		{
			name:     "whitespace only",
			styles:   []string{"   ", "\t", "\n"},
			expected: "",
		},
		{
			name:     "mixed empty and valid",
			styles:   []string{"", "text-center", "   ", "font-bold", ""},
			expected: "text-center font-bold",
		},
		{
			name:     "extra whitespace in classes",
			styles:   []string{"  text-center  font-bold  ", "  bg-gray-100  "},
			expected: "text-center font-bold bg-gray-100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeStyles(tt.styles...)
			if result != tt.expected {
				t.Errorf("MergeStyles() = %q, want %q", result, tt.expected)
			}
		})
	}
}