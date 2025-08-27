package tailwind

import (
	"testing"
)

func TestParsePadding(t *testing.T) {
	tests := []struct {
		name   string
		class  string
		top    *float64
		right  *float64
		bottom *float64
		left   *float64
	}{
		{
			name:   "p-4 all sides",
			class:  "p-4",
			top:    ptr(1.0),
			right:  ptr(1.0),
			bottom: ptr(1.0),
			left:   ptr(1.0),
		},
		{
			name:  "px-8 horizontal",
			class: "px-8",
			right: ptr(2.0),
			left:  ptr(2.0),
		},
		{
			name:   "py-2 vertical",
			class:  "py-2",
			top:    ptr(0.5),
			bottom: ptr(0.5),
		},
		{
			name:  "pt-1 top only",
			class: "pt-1",
			top:   ptr(0.25),
		},
		{
			name:  "pr-2 right only",
			class: "pr-2",
			right: ptr(0.5),
		},
		{
			name:   "pb-3 bottom only",
			class:  "pb-3",
			bottom: ptr(0.75),
		},
		{
			name:  "pl-4 left only",
			class: "pl-4",
			left:  ptr(1.0),
		},
		{
			name:   "p-0 zero padding",
			class:  "p-0",
			top:    ptr(0.0),
			right:  ptr(0.0),
			bottom: ptr(0.0),
			left:   ptr(0.0),
		},
		{
			name:   "p-px single pixel",
			class:  "p-px",
			top:    ptr(0.0625),
			right:  ptr(0.0625),
			bottom: ptr(0.0625),
			left:   ptr(0.0625),
		},
		{
			name:  "invalid class",
			class: "text-red-500",
		},
		{
			name:  "p-invalid",
			class: "p-invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			top, right, bottom, left := ParsePadding(tt.class)

			if !equalPtr(top, tt.top) {
				t.Errorf("Expected top %v, got %v", tt.top, top)
			}
			if !equalPtr(right, tt.right) {
				t.Errorf("Expected right %v, got %v", tt.right, right)
			}
			if !equalPtr(bottom, tt.bottom) {
				t.Errorf("Expected bottom %v, got %v", tt.bottom, bottom)
			}
			if !equalPtr(left, tt.left) {
				t.Errorf("Expected left %v, got %v", tt.left, left)
			}
		})
	}
}

func TestParseFontSize(t *testing.T) {
	tests := []struct {
		name     string
		class    string
		expected float64
	}{
		{name: "text-xs", class: "text-xs", expected: 0.75},
		{name: "text-sm", class: "text-sm", expected: 0.875},
		{name: "text-base", class: "text-base", expected: 1},
		{name: "text-lg", class: "text-lg", expected: 1.125},
		{name: "text-xl", class: "text-xl", expected: 1.25},
		{name: "text-2xl", class: "text-2xl", expected: 1.5},
		{name: "text-3xl", class: "text-3xl", expected: 1.875},
		{name: "text-4xl", class: "text-4xl", expected: 2.25},
		{name: "text-5xl", class: "text-5xl", expected: 3},
		{name: "text-6xl", class: "text-6xl", expected: 3.75},
		{name: "text-7xl", class: "text-7xl", expected: 4.5},
		{name: "text-8xl", class: "text-8xl", expected: 6},
		{name: "text-9xl", class: "text-9xl", expected: 8},
		{name: "not a font size", class: "text-red-500", expected: 0},
		{name: "invalid", class: "invalid", expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseFontSize(tt.class)
			if result != tt.expected {
				t.Errorf("Expected font size %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseCustomSpacing(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected float64
		hasError bool
	}{
		{name: "10px", value: "10px", expected: 0.625}, // 10/16
		{name: "24px", value: "24px", expected: 1.5},   // 24/16
		{name: "1.5rem", value: "1.5rem", expected: 1.5},
		{name: "2rem", value: "2rem", expected: 2},
		{name: "1em", value: "1em", expected: 1},
		{name: "0.5em", value: "0.5em", expected: 0.5},
		{name: "unitless 3", value: "3", expected: 3},
		{name: "unitless 0.25", value: "0.25", expected: 0.25},
		{name: "invalid", value: "invalid", hasError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCustomSpacing(tt.value)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for %s, got none", tt.value)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.value, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestParsePaddingCustom(t *testing.T) {
	tests := []struct {
		name   string
		class  string
		top    *float64
		right  *float64
		bottom *float64
		left   *float64
	}{
		{
			name:   "p-[10px] custom pixel",
			class:  "p-[10px]",
			top:    ptr(0.625),
			right:  ptr(0.625),
			bottom: ptr(0.625),
			left:   ptr(0.625),
		},
		{
			name:  "px-[2rem] custom rem",
			class: "px-[2rem]",
			right: ptr(2.0),
			left:  ptr(2.0),
		},
		{
			name:   "py-[1.5] unitless",
			class:  "py-[1.5]",
			top:    ptr(1.5),
			bottom: ptr(1.5),
		},
		{
			name:  "pl-[24px]",
			class: "pl-[24px]",
			left:  ptr(1.5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			top, right, bottom, left := ParsePadding(tt.class)

			if !equalPtr(top, tt.top) {
				t.Errorf("Expected top %v, got %v", tt.top, top)
			}
			if !equalPtr(right, tt.right) {
				t.Errorf("Expected right %v, got %v", tt.right, right)
			}
			if !equalPtr(bottom, tt.bottom) {
				t.Errorf("Expected bottom %v, got %v", tt.bottom, bottom)
			}
			if !equalPtr(left, tt.left) {
				t.Errorf("Expected left %v, got %v", tt.left, left)
			}
		})
	}
}

func TestParseFontSizeCustom(t *testing.T) {
	tests := []struct {
		name     string
		class    string
		expected float64
	}{
		{name: "text-[14px]", class: "text-[14px]", expected: 0.875}, // 14/16
		{name: "text-[1.5rem]", class: "text-[1.5rem]", expected: 1.5},
		{name: "text-[2em]", class: "text-[2em]", expected: 2},
		{name: "text-[0.75]", class: "text-[0.75]", expected: 0.75},
		{name: "text-[invalid]", class: "text-[invalid]", expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseFontSize(tt.class)
			if result != tt.expected {
				t.Errorf("Expected font size %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper functions
func ptr(v float64) *float64 {
	return &v
}

func equalPtr(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
