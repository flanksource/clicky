package tailwind

import (
	"testing"
)

func TestParseTailwindColor(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError bool
	}{
		// Base colors with shades
		{"red-500", "red-500", "#ef4444", false},
		{"blue-700", "blue-700", "#1d4ed8", false},
		{"green-300", "green-300", "#86efac", false},
		{"gray-900", "gray-900", "#111827", false},
		{"slate-50", "slate-50", "#f8fafc", false},
		
		// With prefixes
		{"bg-red-500", "bg-red-500", "#ef4444", false},
		{"text-blue-700", "text-blue-700", "#1d4ed8", false},
		{"border-green-300", "border-green-300", "#86efac", false},
		{"ring-purple-600", "ring-purple-600", "#9333ea", false},
		{"fill-amber-400", "fill-amber-400", "#fbbf24", false},
		
		// Default to 500 shade
		{"red", "red", "#ef4444", false},
		{"blue", "blue", "#3b82f6", false},
		{"green", "green", "#22c55e", false},
		
		// Special colors
		{"black", "black", "#000000", false},
		{"white", "white", "#ffffff", false},
		{"transparent", "transparent", "transparent", false},
		
		// With prefixes and special colors
		{"bg-black", "bg-black", "#000000", false},
		{"text-white", "text-white", "#ffffff", false},
		{"border-transparent", "border-transparent", "transparent", false},
		
		// Edge cases
		{"", "", "", true},
		{"invalid-color", "invalid-color", "invalid-color", false}, // Returns as-is
		{"red-1000", "red-1000", "", true}, // Invalid shade
		
		// All color families
		{"indigo-500", "indigo-500", "#6366f1", false},
		{"violet-500", "violet-500", "#8b5cf6", false},
		{"purple-500", "purple-500", "#a855f7", false},
		{"fuchsia-500", "fuchsia-500", "#d946ef", false},
		{"pink-500", "pink-500", "#ec4899", false},
		{"rose-500", "rose-500", "#f43f5e", false},
		{"orange-500", "orange-500", "#f97316", false},
		{"amber-500", "amber-500", "#f59e0b", false},
		{"yellow-500", "yellow-500", "#eab308", false},
		{"lime-500", "lime-500", "#84cc16", false},
		{"emerald-500", "emerald-500", "#10b981", false},
		{"teal-500", "teal-500", "#14b8a6", false},
		{"cyan-500", "cyan-500", "#06b6d4", false},
		{"sky-500", "sky-500", "#0ea5e9", false},
		{"zinc-500", "zinc-500", "#71717a", false},
		{"neutral-500", "neutral-500", "#737373", false},
		{"stone-500", "stone-500", "#78716c", false},
		
		// Test 950 shades (not all colors have them)
		{"slate-950", "slate-950", "#020617", false},
		{"gray-950", "gray-950", "#030712", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTailwindColor(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseTailwindColor(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseTailwindColor(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetTailwindColorName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"red-500", "red"},
		{"bg-blue-700", "blue"},
		{"text-green-300", "green"},
		{"red", "red"},
		{"black", "black"},
		{"border-purple-600", "purple"},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := GetTailwindColorName(tt.input)
			if result != tt.expected {
				t.Errorf("GetTailwindColorName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetTailwindShade(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"red-500", "500"},
		{"bg-blue-700", "700"},
		{"text-green-300", "300"},
		{"red", "500"},      // Default
		{"black", "500"},    // Default
		{"slate-950", "950"},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := GetTailwindShade(tt.input)
			if result != tt.expected {
				t.Errorf("GetTailwindShade(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsTailwindColor(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"red-500", true},
		{"bg-blue-700", true},
		{"black", true},
		{"transparent", true},
		{"", false},
		{"red-1000", false}, // Invalid shade
		{"#ff0000", true},   // Passes through as-is
		{"rgb(255,0,0)", true}, // Passes through as-is
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsTailwindColor(tt.input)
			if result != tt.expected {
				t.Errorf("IsTailwindColor(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestColor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Tailwind colors
		{"red-500", "red-500", "#ef4444"},
		{"bg-blue-700", "bg-blue-700", "#1d4ed8"},
		{"text-green-300", "text-green-300", "#86efac"},
		
		// Default to 500 shade
		{"red", "red", "#ef4444"},
		{"blue", "blue", "#3b82f6"},
		
		// Special colors
		{"black", "black", "#000000"},
		{"white", "white", "#ffffff"},
		
		// Hex colors pass through
		{"#ff0000", "#ff0000", "#ff0000"},
		
		// Invalid colors pass through as-is
		{"invalid", "invalid", "invalid"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Color(tt.input)
			
			// Convert lipgloss.Color to string for comparison
			resultStr := string(result)
			if tt.input == "transparent" {
				// Special case for transparent which returns empty string
				if resultStr != "" {
					t.Errorf("Color(%q) expected empty string for transparent, got %q", tt.input, resultStr)
				}
			} else if resultStr != tt.expected {
				t.Errorf("Color(%q) = %q, want %q", tt.input, resultStr, tt.expected)
			}
		})
	}
}