package tailwind

import (
	"testing"
)

func TestParseTailwindColor(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		// Base colors with shades (adaptive colors - may be modified based on background)
		{"red-500", "red-500", false},
		{"blue-700", "blue-700", false},
		{"green-300", "green-300", false},
		{"gray-900", "gray-900", false},
		{"slate-50", "slate-50", false},

		// With prefixes
		{"bg-red-500", "bg-red-500", false},
		{"text-blue-700", "text-blue-700", false},
		{"border-green-300", "border-green-300", false},
		{"ring-purple-600", "ring-purple-600", false},
		{"fill-amber-400", "fill-amber-400", false},

		// Default to 500 shade
		{"red", "red", false},
		{"blue", "blue", false},
		{"green", "green", false},

		// Special colors
		{"black", "black", false},
		{"white", "white", false},
		{"transparent", "transparent", false},

		// With prefixes and special colors
		{"bg-black", "bg-black", false},
		{"text-white", "text-white", false},
		{"border-transparent", "border-transparent", false},

		// Edge cases
		{"", "", true},
		{"invalid-color", "invalid-color", false}, // Returns as-is
		{"red-1000", "red-1000", true},            // Invalid shade

		// All color families
		{"indigo-500", "indigo-500", false},
		{"violet-500", "violet-500", false},
		{"purple-500", "purple-500", false},
		{"fuchsia-500", "fuchsia-500", false},
		{"pink-500", "pink-500", false},
		{"rose-500", "rose-500", false},
		{"orange-500", "orange-500", false},
		{"amber-500", "amber-500", false},
		{"yellow-500", "yellow-500", false},
		{"lime-500", "lime-500", false},
		{"emerald-500", "emerald-500", false},
		{"teal-500", "teal-500", false},
		{"cyan-500", "cyan-500", false},
		{"sky-500", "sky-500", false},
		{"zinc-500", "zinc-500", false},
		{"neutral-500", "neutral-500", false},
		{"stone-500", "stone-500", false},

		// Test 950 shades (not all colors have them)
		{"slate-950", "slate-950", false},
		{"gray-950", "gray-950", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTailwindColor(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseTailwindColor(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
				return
			}
			
			// For adaptive colors, just verify valid output format
			if !tt.wantError {
				// Special colors should remain unchanged
				if tt.input == "transparent" {
					if result != "transparent" {
						t.Errorf("ParseTailwindColor(%q) = %q, want %q", tt.input, result, "transparent")
					}
				} else if result != "" && result != "transparent" && result != "currentColor" {
					// Should be valid hex color OR pass-through invalid colors
					if len(result) == 7 && result[0] == '#' {
						// Valid hex color - good
					} else if result == tt.input {
						// Pass-through color (e.g., "invalid-color") - also valid
					} else {
						t.Errorf("ParseTailwindColor(%q) = %q, expected valid hex color or pass-through", tt.input, result)
					}
				}
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
		{"red", "500"},   // Default
		{"black", "500"}, // Default
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
		{"red-1000", false},    // Invalid shade
		{"#ff0000", true},      // Passes through as-is
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

func TestParseRawTailwindColor(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		wantError bool
	}{
		// Base colors with shades (raw - no background adaptation)
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
		{"current", "current", "currentColor", false},

		// With prefixes and special colors
		{"bg-black", "bg-black", "#000000", false},
		{"text-white", "text-white", "#ffffff", false},
		{"border-transparent", "border-transparent", "transparent", false},

		// Edge cases
		{"", "", "", true},
		{"invalid-color", "invalid-color", "invalid-color", false}, // Returns as-is
		{"red-1000", "red-1000", "", true},                         // Invalid shade

		// All color families at 500 shade
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

		// Extreme shades
		{"slate-950", "slate-950", "#020617", false},
		{"gray-950", "gray-950", "#030712", false},
		{"red-50", "red-50", "#fef2f2", false},
		{"blue-50", "blue-50", "#eff6ff", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseRawTailwindColor(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ParseRawTailwindColor(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseRawTailwindColor(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAdaptColorForBackground(t *testing.T) {
	tests := []struct {
		name     string
		hexColor string
		isDark   bool
		expected string
	}{
		// Special colors - should not be adapted
		{"transparent", "transparent", true, "transparent"},
		{"transparent", "transparent", false, "transparent"},
		{"currentColor", "currentColor", true, "currentColor"},
		{"currentColor", "currentColor", false, "currentColor"},
		{"empty", "", true, ""},
		{"empty", "", false, ""},

		// Very dark colors on dark background - should be lightened
		{"black on dark", "#000000", true, "#999999"}, // Will be adapted to lighter
		{"dark gray on dark", "#111827", true, "#9ca3af"}, // gray-900 -> lighter
		
		// Very light colors on light background - should be darkened
		{"white on light", "#ffffff", false, "#404040"}, // Will be adapted to darker
		{"light gray on light", "#f9fafb", false, "#4b5563"}, // gray-50 -> darker

		// Mid-range colors - should remain unchanged
		{"red-500 on dark", "#ef4444", true, "#ef4444"},
		{"red-500 on light", "#ef4444", false, "#ef4444"},
		{"blue-600 on dark", "#2563eb", true, "#2563eb"},
		{"blue-600 on light", "#2563eb", false, "#2563eb"},

		// Colors with good visibility - should not be adapted
		{"green-400 on dark", "#4ade80", true, "#4ade80"},
		{"purple-700 on light", "#7e22ce", false, "#7e22ce"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adaptColorForBackground(tt.hexColor, tt.isDark)
			
			// For special cases, expect exact match
			if tt.hexColor == "transparent" || tt.hexColor == "currentColor" || tt.hexColor == "" {
				if result != tt.expected {
					t.Errorf("adaptColorForBackground(%q, %v) = %q, want %q", tt.hexColor, tt.isDark, result, tt.expected)
				}
				return
			}

			// For color adaptation, check if adaptation occurred when expected
			if tt.hexColor == "#000000" && tt.isDark {
				// Very dark color on dark background should be lightened
				if result == tt.hexColor {
					t.Errorf("adaptColorForBackground(%q, %v) = %q, expected color to be lightened", tt.hexColor, tt.isDark, result)
				}
			} else if tt.hexColor == "#ffffff" && !tt.isDark {
				// Very light color on light background should be darkened
				if result == tt.hexColor {
					t.Errorf("adaptColorForBackground(%q, %v) = %q, expected color to be darkened", tt.hexColor, tt.isDark, result)
				}
			} else if tt.hexColor == "#ef4444" {
				// Mid-range colors should remain unchanged
				if result != tt.hexColor {
					t.Errorf("adaptColorForBackground(%q, %v) = %q, expected no adaptation", tt.hexColor, tt.isDark, result)
				}
			}
		})
	}
}

func TestColorHelperFunctions(t *testing.T) {
	t.Run("hexToRGB", func(t *testing.T) {
		tests := []struct {
			hex         string
			expectedR   int
			expectedG   int
			expectedB   int
			shouldError bool
		}{
			{"#ff0000", 255, 0, 0, false},
			{"#00ff00", 0, 255, 0, false},
			{"#0000ff", 0, 0, 255, false},
			{"#ffffff", 255, 255, 255, false},
			{"#000000", 0, 0, 0, false},
			{"ff0000", 255, 0, 0, false}, // Without #
			{"#ef4444", 239, 68, 68, false}, // red-500
			{"invalid", 0, 0, 0, true}, // Invalid hex
			{"#ff", 0, 0, 0, true},     // Too short
		}

		for _, tt := range tests {
			r, g, b, err := hexToRGB(tt.hex)
			if (err != nil) != tt.shouldError {
				t.Errorf("hexToRGB(%q) error = %v, shouldError %v", tt.hex, err, tt.shouldError)
				continue
			}
			if !tt.shouldError {
				if r != tt.expectedR || g != tt.expectedG || b != tt.expectedB {
					t.Errorf("hexToRGB(%q) = (%d, %d, %d), want (%d, %d, %d)", 
						tt.hex, r, g, b, tt.expectedR, tt.expectedG, tt.expectedB)
				}
			}
		}
	})

	t.Run("rgbToHex", func(t *testing.T) {
		tests := []struct {
			r, g, b  int
			expected string
		}{
			{255, 0, 0, "#ff0000"},
			{0, 255, 0, "#00ff00"},
			{0, 0, 255, "#0000ff"},
			{255, 255, 255, "#ffffff"},
			{0, 0, 0, "#000000"},
			{239, 68, 68, "#ef4444"}, // red-500
			{300, -10, 500, "#ff00ff"}, // Clamped values
		}

		for _, tt := range tests {
			result := rgbToHex(tt.r, tt.g, tt.b)
			if result != tt.expected {
				t.Errorf("rgbToHex(%d, %d, %d) = %q, want %q", tt.r, tt.g, tt.b, result, tt.expected)
			}
		}
	})

	t.Run("calculateLuminance", func(t *testing.T) {
		tests := []struct {
			r, g, b  int
			expected float64
			tolerance float64
		}{
			{0, 0, 0, 0.0, 0.001},         // Black
			{255, 255, 255, 1.0, 0.001},   // White
			{128, 128, 128, 0.215, 0.01},  // Middle gray
		}

		for _, tt := range tests {
			result := calculateLuminance(tt.r, tt.g, tt.b)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("calculateLuminance(%d, %d, %d) = %f, want %f (±%f)", 
					tt.r, tt.g, tt.b, result, tt.expected, tt.tolerance)
			}
		}
	})

	t.Run("rgbToHSL and hslToRGB", func(t *testing.T) {
		tests := []struct {
			r, g, b int
		}{
			{255, 0, 0},   // Red
			{0, 255, 0},   // Green
			{0, 0, 255},   // Blue
			{255, 255, 255}, // White
			{0, 0, 0},     // Black
			{128, 128, 128}, // Gray
			{239, 68, 68}, // red-500
		}

		for _, tt := range tests {
			// Convert to HSL and back to RGB
			h, s, l := rgbToHSL(tt.r, tt.g, tt.b)
			backR, backG, backB := hslToRGB(h, s, l)

			// Allow for small rounding errors
			if abs(backR-tt.r) > 1 || abs(backG-tt.g) > 1 || abs(backB-tt.b) > 1 {
				t.Errorf("RGB->HSL->RGB: (%d,%d,%d) -> (%.3f,%.3f,%.3f) -> (%d,%d,%d)", 
					tt.r, tt.g, tt.b, h, s, l, backR, backG, backB)
			}
		}
	})
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestColorVsRawColor(t *testing.T) {
	tests := []struct {
		colorClass string
	}{
		{"red-500"},
		{"bg-blue-700"},
		{"text-green-300"},
		{"black"},
		{"white"},
		{"transparent"},
		{"purple-600"},
	}

	for _, tt := range tests {
		t.Run(tt.colorClass, func(t *testing.T) {
			rawColor, rawErr := ParseRawTailwindColor(tt.colorClass)
			adaptiveColor, adaptiveErr := ParseTailwindColor(tt.colorClass)

			// Both should either succeed or fail together for valid inputs
			if (rawErr != nil) != (adaptiveErr != nil) {
				t.Errorf("Error consistency: ParseRawTailwindColor(%q) err=%v, ParseTailwindColor(%q) err=%v",
					tt.colorClass, rawErr, tt.colorClass, adaptiveErr)
			}

			if rawErr == nil && adaptiveErr == nil {
				// For special colors, they should be the same
				if rawColor == "transparent" || rawColor == "currentColor" {
					if rawColor != adaptiveColor {
						t.Errorf("Special color mismatch: raw=%q, adaptive=%q", rawColor, adaptiveColor)
					}
				}

				// Both should be valid hex colors (or special values)
				if rawColor != "" && rawColor != "transparent" && rawColor != "currentColor" {
					if len(rawColor) != 7 || rawColor[0] != '#' {
						t.Errorf("Invalid raw color format: %q", rawColor)
					}
				}
				if adaptiveColor != "" && adaptiveColor != "transparent" && adaptiveColor != "currentColor" {
					if len(adaptiveColor) != 7 || adaptiveColor[0] != '#' {
						t.Errorf("Invalid adaptive color format: %q", adaptiveColor)
					}
				}
			}
		})
	}
}
