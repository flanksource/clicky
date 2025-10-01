package tailwind

import (
	"testing"

	"github.com/flanksource/maroto/v2/pkg/consts/align"
)

func TestParseAlignment(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedHoriz align.Type
		expectedVert  VerticalAlign
	}{
		// Basic horizontal alignments
		{
			name:          "text-left",
			input:         "text-left",
			expectedHoriz: align.Left,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "text-center",
			input:         "text-center",
			expectedHoriz: align.Center,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "text-right",
			input:         "text-right",
			expectedHoriz: align.Right,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "text-justify",
			input:         "text-justify",
			expectedHoriz: align.Justify,
			expectedVert:  VerticalMiddle,
		},

		// Basic vertical alignments
		{
			name:          "align-top",
			input:         "align-top",
			expectedHoriz: align.Left,
			expectedVert:  VerticalTop,
		},
		{
			name:          "align-middle",
			input:         "align-middle",
			expectedHoriz: align.Left,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "align-bottom",
			input:         "align-bottom",
			expectedHoriz: align.Left,
			expectedVert:  VerticalBottom,
		},

		// Extended syntax combinations
		{
			name:          "text-left-top",
			input:         "text-left-top",
			expectedHoriz: align.Left,
			expectedVert:  VerticalTop,
		},
		{
			name:          "text-center-middle",
			input:         "text-center-middle",
			expectedHoriz: align.Center,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "text-right-bottom",
			input:         "text-right-bottom",
			expectedHoriz: align.Right,
			expectedVert:  VerticalBottom,
		},

		// Complex style strings (should extract alignment)
		{
			name:          "style with text-center",
			input:         "font-bold text-center bg-gray-100",
			expectedHoriz: align.Center,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "style with align-top",
			input:         "w-[200px] align-top font-mono",
			expectedHoriz: align.Left,
			expectedVert:  VerticalTop,
		},
		{
			name:          "style with both alignments",
			input:         "text-right align-bottom font-bold",
			expectedHoriz: align.Right,
			expectedVert:  VerticalBottom,
		},

		// No alignment specified (defaults)
		{
			name:          "no alignment",
			input:         "font-bold bg-gray-100",
			expectedHoriz: align.Left,
			expectedVert:  VerticalMiddle,
		},

		// Empty string
		{
			name:          "empty string",
			input:         "",
			expectedHoriz: align.Left,
			expectedVert:  VerticalMiddle,
		},

		// Multiple alignment classes (last one should win for conflicting properties)
		{
			name:          "multiple horizontal alignments",
			input:         "text-left text-center text-right",
			expectedHoriz: align.Right,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "multiple vertical alignments",
			input:         "align-top align-middle align-bottom",
			expectedHoriz: align.Left,
			expectedVert:  VerticalBottom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAlignment(tt.input)

			if result.Horizontal != tt.expectedHoriz {
				t.Errorf("ParseAlignment(%q).Horizontal = %v, want %v", tt.input, result.Horizontal, tt.expectedHoriz)
			}

			if result.Vertical != tt.expectedVert {
				t.Errorf("ParseAlignment(%q).Vertical = %v, want %v", tt.input, result.Vertical, tt.expectedVert)
			}
		})
	}
}

func TestRealWorldStyleStrings(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedHoriz align.Type
		expectedVert  VerticalAlign
	}{
		{
			name:          "table header style",
			input:         "font-bold bg-gray-800 text-white text-center align-middle",
			expectedHoriz: align.Center,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "table cell style",
			input:         "w-[20ch] text-left align-middle font-medium",
			expectedHoriz: align.Left,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "price column style",
			input:         "w-[10ch] text-right align-middle font-mono",
			expectedHoriz: align.Right,
			expectedVert:  VerticalMiddle,
		},
		{
			name:          "status column style",
			input:         "w-[12ch] text-center align-top font-bold text-green-600",
			expectedHoriz: align.Center,
			expectedVert:  VerticalTop,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAlignment(tt.input)

			if result.Horizontal != tt.expectedHoriz {
				t.Errorf("ParseAlignment(%q).Horizontal = %v, want %v", tt.input, result.Horizontal, tt.expectedHoriz)
			}

			if result.Vertical != tt.expectedVert {
				t.Errorf("ParseAlignment(%q).Vertical = %v, want %v", tt.input, result.Vertical, tt.expectedVert)
			}
		})
	}
}
