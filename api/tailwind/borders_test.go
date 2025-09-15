package tailwind

import (
	"testing"
)

func TestParseBorders(t *testing.T) {
	tests := []struct {
		name     string
		styles   []string
		expected Borders
	}{
		{
			name:   "no border",
			styles: []string{},
			expected: Borders{
				Top:    Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Right:  Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Left:   Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
			},
		},
		{
			name:   "global border default",
			styles: []string{"border"},
			expected: Borders{
				Top:    Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Right:  Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Left:   Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
			},
		},
		{
			name:   "global border with width",
			styles: []string{"border-2"},
			expected: Borders{
				Top:    Line{Width: 2.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Right:  Line{Width: 2.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 2.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Left:   Line{Width: 2.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
			},
		},
		{
			name:   "global border with style",
			styles: []string{"border border-dashed"},
			expected: Borders{
				Top:    Line{Width: 1.0, Style: Dashed, Color: BorderColor{Hex: "#000000"}},
				Right:  Line{Width: 1.0, Style: Dashed, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 1.0, Style: Dashed, Color: BorderColor{Hex: "#000000"}},
				Left:   Line{Width: 1.0, Style: Dashed, Color: BorderColor{Hex: "#000000"}},
			},
		},
		{
			name:   "global border with color",
			styles: []string{"border border-red-500"},
			expected: Borders{
				Top:    Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#ef4444"}},
				Right:  Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#ef4444"}},
				Bottom: Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#ef4444"}},
				Left:   Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#ef4444"}},
			},
		},
		{
			name:   "combined global border",
			styles: []string{"border-2 border-solid border-gray-700"},
			expected: Borders{
				Top:    Line{Width: 2.0, Style: Solid, Color: BorderColor{Hex: "#374151"}},
				Right:  Line{Width: 2.0, Style: Solid, Color: BorderColor{Hex: "#374151"}},
				Bottom: Line{Width: 2.0, Style: Solid, Color: BorderColor{Hex: "#374151"}},
				Left:   Line{Width: 2.0, Style: Solid, Color: BorderColor{Hex: "#374151"}},
			},
		},
		{
			name:   "top border only",
			styles: []string{"border-t"},
			expected: Borders{
				Top:    Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Right:  Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Left:   Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
			},
		},
		{
			name:   "top border with width",
			styles: []string{"border-t-4"},
			expected: Borders{
				Top:    Line{Width: 4.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Right:  Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Left:   Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
			},
		},
		{
			name:   "top border with color",
			styles: []string{"border-t border-t-blue-500"},
			expected: Borders{
				Top:    Line{Width: 1.0, Style: Solid, Color: BorderColor{Hex: "#3b82f6"}},
				Right:  Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Left:   Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
			},
		},
		{
			name:   "mixed side borders",
			styles: []string{"border-t-2 border-t-red-500 border-b-4 border-b-blue-500"},
			expected: Borders{
				Top:    Line{Width: 2.0, Style: Solid, Color: BorderColor{Hex: "#ef4444"}},
				Right:  Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 4.0, Style: Solid, Color: BorderColor{Hex: "#3b82f6"}},
				Left:   Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
			},
		},
		{
			name:   "all sides different",
			styles: []string{"border-t-2 border-t-dashed border-t-red-500", "border-r-0", "border-b-4 border-b-solid border-b-blue-500", "border-l-1 border-l-dotted border-l-green-500"},
			expected: Borders{
				Top:    Line{Width: 2.0, Style: Dashed, Color: BorderColor{Hex: "#ef4444"}},
				Right:  Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 4.0, Style: Solid, Color: BorderColor{Hex: "#3b82f6"}},
				Left:   Line{Width: 1.0, Style: Dotted, Color: BorderColor{Hex: "#22c55e"}},
			},
		},
		{
			name:   "arbitrary width values",
			styles: []string{"border-[3px]"},
			expected: Borders{
				Top:    Line{Width: 3.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Right:  Line{Width: 3.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 3.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Left:   Line{Width: 3.0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
			},
		},
		{
			name:   "border none",
			styles: []string{"border-0"},
			expected: Borders{
				Top:    Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Right:  Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Bottom: Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
				Left:   Line{Width: 0, Style: Solid, Color: BorderColor{Hex: "#000000"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseBorders(tt.styles...)

			// Check each side
			checkLine(t, "Top", result.Top, tt.expected.Top)
			checkLine(t, "Right", result.Right, tt.expected.Right)
			checkLine(t, "Bottom", result.Bottom, tt.expected.Bottom)
			checkLine(t, "Left", result.Left, tt.expected.Left)
		})
	}
}

func checkLine(t *testing.T, side string, actual, expected Line) {
	if actual.Width != expected.Width {
		t.Errorf("%s width: expected %.1f, got %.1f", side, expected.Width, actual.Width)
	}
	if actual.Style != expected.Style {
		t.Errorf("%s style: expected %s, got %s", side, expected.Style, actual.Style)
	}
	if actual.Color.Hex != expected.Color.Hex {
		t.Errorf("%s color: expected %s, got %s", side, expected.Color.Hex, actual.Color.Hex)
	}
}

func TestParseBorderWidth(t *testing.T) {
	tests := []struct {
		suffix   string
		expected float64
		valid    bool
	}{
		{"0", 0, true},
		{"2", 2, true},
		{"4", 4, true},
		{"8", 8, true},
		{"[3px]", 3, true},
		{"[5]", 5, true},
		{"invalid", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.suffix, func(t *testing.T) {
			width, valid := parseBorderWidth(tt.suffix)
			if valid != tt.valid {
				t.Errorf("expected valid=%v, got valid=%v", tt.valid, valid)
			}
			if valid && width != tt.expected {
				t.Errorf("expected width=%.1f, got width=%.1f", tt.expected, width)
			}
		})
	}
}

func TestParseBorderStyle(t *testing.T) {
	tests := []struct {
		suffix   string
		expected LineStyle
		valid    bool
	}{
		{"solid", Solid, true},
		{"dashed", Dashed, true},
		{"dotted", Dotted, true},
		{"double", Double, true},
		{"none", None, true},
		{"invalid", Solid, false},
		{"", Solid, false},
	}

	for _, tt := range tests {
		t.Run(tt.suffix, func(t *testing.T) {
			style, valid := parseBorderStyle(tt.suffix)
			if valid != tt.valid {
				t.Errorf("expected valid=%v, got valid=%v", tt.valid, valid)
			}
			if valid && style != tt.expected {
				t.Errorf("expected style=%s, got style=%s", tt.expected, style)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("NewSolidBorder", func(t *testing.T) {
		color := BorderColor{Hex: "#ff0000"}
		borders := NewSolidBorder(2.0, color)
		
		expected := Line{Width: 2.0, Style: Solid, Color: color}
		checkLine(t, "Top", borders.Top, expected)
		checkLine(t, "Right", borders.Right, expected)
		checkLine(t, "Bottom", borders.Bottom, expected)
		checkLine(t, "Left", borders.Left, expected)
	})

	t.Run("NewDashedBorder", func(t *testing.T) {
		color := BorderColor{Hex: "#00ff00"}
		borders := NewDashedBorder(1.0, color)
		
		expected := Line{Width: 1.0, Style: Dashed, Color: color}
		checkLine(t, "Top", borders.Top, expected)
		checkLine(t, "Right", borders.Right, expected)
		checkLine(t, "Bottom", borders.Bottom, expected)
		checkLine(t, "Left", borders.Left, expected)
	})

	t.Run("NewTopBottomBorder", func(t *testing.T) {
		color := BorderColor{Hex: "#0000ff"}
		borders := NewTopBottomBorder(3.0, color, Dotted)
		
		expectedActive := Line{Width: 3.0, Style: Dotted, Color: color}
		expectedInactive := Line{Width: 0}
		
		checkLine(t, "Top", borders.Top, expectedActive)
		checkLine(t, "Right", borders.Right, expectedInactive)
		checkLine(t, "Bottom", borders.Bottom, expectedActive)
		checkLine(t, "Left", borders.Left, expectedInactive)
	})

	t.Run("NewLeftRightBorder", func(t *testing.T) {
		color := BorderColor{Hex: "#ff00ff"}
		borders := NewLeftRightBorder(2.5, color, Double)
		
		expectedActive := Line{Width: 2.5, Style: Double, Color: color}
		expectedInactive := Line{Width: 0}
		
		checkLine(t, "Top", borders.Top, expectedInactive)
		checkLine(t, "Right", borders.Right, expectedActive)
		checkLine(t, "Bottom", borders.Bottom, expectedInactive)
		checkLine(t, "Left", borders.Left, expectedActive)
	})
}