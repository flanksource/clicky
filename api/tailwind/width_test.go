package tailwind

import (
	"testing"
)

func TestParseWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *WidthSpec
		wantErr  bool
	}{
		// Character-based widths
		{
			name:  "character width",
			input: "w-[10ch]",
			expected: &WidthSpec{
				Type:     WidthCharacter,
				Value:    10,
				Unit:     "ch",
				Original: "w-[10ch]",
			},
		},
		{
			name:  "min character width",
			input: "min-w-[5ch]",
			expected: &WidthSpec{
				Type:     WidthCharacter,
				Value:    5,
				Unit:     "ch",
				IsMin:    true,
				Original: "min-w-[5ch]",
			},
		},

		// Percentage widths
		{
			name:  "percentage width",
			input: "w-[25%]",
			expected: &WidthSpec{
				Type:     WidthPercentage,
				Value:    25,
				Unit:     "%",
				Original: "w-[25%]",
			},
		},

		// Fractional widths
		{
			name:  "half width",
			input: "w-1/2",
			expected: &WidthSpec{
				Type:     WidthFraction,
				Value:    0.5,
				Unit:     "fraction",
				Original: "w-1/2",
			},
		},

		// Auto width
		{
			name:  "auto width",
			input: "w-auto",
			expected: &WidthSpec{
				Type:     WidthAuto,
				Value:    0,
				Unit:     "auto",
				Original: "w-auto",
			},
		},

		// Error cases
		{
			name:    "invalid format",
			input:   "invalid-width",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ParseWidth(tt.input)

			if tt.wantErr {
				if ok {
					t.Errorf("ParseWidth() expected no match, got %v", result)
				}
				return
			}

			if !ok {
				t.Errorf("ParseWidth() expected match, got false")
				return
			}

			if result == nil {
				t.Errorf("ParseWidth() returned nil result")
				return
			}

			if result.Type != tt.expected.Type {
				t.Errorf("ParseWidth() Type = %v, want %v", result.Type, tt.expected.Type)
			}

			if result.Value != tt.expected.Value {
				t.Errorf("ParseWidth() Value = %v, want %v", result.Value, tt.expected.Value)
			}

			if result.Unit != tt.expected.Unit {
				t.Errorf("ParseWidth() Unit = %v, want %v", result.Unit, tt.expected.Unit)
			}

			if result.IsMin != tt.expected.IsMin {
				t.Errorf("ParseWidth() IsMin = %v, want %v", result.IsMin, tt.expected.IsMin)
			}

			if result.IsMax != tt.expected.IsMax {
				t.Errorf("ParseWidth() IsMax = %v, want %v", result.IsMax, tt.expected.IsMax)
			}

			if result.Original != tt.expected.Original {
				t.Errorf("ParseWidth() Original = %v, want %v", result.Original, tt.expected.Original)
			}
		})
	}
}

func TestParseWidthFromStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *WidthSpec
	}{
		{
			name:  "width in middle of style",
			input: "text-center w-[20ch] font-bold",
			expected: &WidthSpec{
				Type:     WidthCharacter,
				Value:    20,
				Unit:     "ch",
				Original: "w-[20ch]",
			},
		},
		{
			name:  "min width at start",
			input: "min-w-[30%] text-left",
			expected: &WidthSpec{
				Type:     WidthPercentage,
				Value:    30,
				Unit:     "%",
				IsMin:    true,
				Original: "min-w-[30%]",
			},
		},
		{
			name:     "no width specification",
			input:    "text-center font-bold bg-blue-500",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseWidthFromStyle(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("ParseWidthFromStyle() = %v, want nil", result)
				}
				return
			}

			if result == nil {
				t.Errorf("ParseWidthFromStyle() = nil, want %v", tt.expected)
				return
			}

			if result.Type != tt.expected.Type {
				t.Errorf("ParseWidthFromStyle() Type = %v, want %v", result.Type, tt.expected.Type)
			}

			if result.Value != tt.expected.Value {
				t.Errorf("ParseWidthFromStyle() Value = %v, want %v", result.Value, tt.expected.Value)
			}

			if result.Unit != tt.expected.Unit {
				t.Errorf("ParseWidthFromStyle() Unit = %v, want %v", result.Unit, tt.expected.Unit)
			}

			if result.Original != tt.expected.Original {
				t.Errorf("ParseWidthFromStyle() Original = %v, want %v", result.Original, tt.expected.Original)
			}
		})
	}
}
