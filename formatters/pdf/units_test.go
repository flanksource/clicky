//go:build pdf

package pdf

import (
	"math"
	"testing"
)

func TestPointToMM(t *testing.T) {
	tests := []struct {
		name     string
		points   Point
		expected MM
	}{
		{"12pt standard font", NewPoint(12), MM(12 * 25.4 / 72.0)}, // 4.233333...
		{"8pt small font", NewPoint(8), MM(8 * 25.4 / 72.0)},       // 2.822222...
		{"16pt large font", NewPoint(16), MM(16 * 25.4 / 72.0)},    // 5.644444...
		{"0pt zero", NewPoint(0), MM(0)},
		{"72pt one inch", NewPoint(72), MM(25.4)}, // 1 inch = 72pt = 25.4mm
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.points.ToMM()
			if math.Abs(float64(result-tt.expected)) > 0.000001 {
				t.Errorf("Point(%v).ToMM() = %v, want %v", tt.points, result, tt.expected)
			}
		})
	}
}

func TestMMToPoint(t *testing.T) {
	tests := []struct {
		name     string
		mm       MM
		expected Point
	}{
		{"25.4mm one inch", NewMM(25.4), Point(72)}, // 1 inch = 25.4mm = 72pt
		{"10mm", NewMM(10), Point(28.3465)},
		{"5mm", NewMM(5), Point(14.17325)},
		{"0mm zero", NewMM(0), Point(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mm.ToPoints()
			if math.Abs(float64(result-tt.expected)) > 0.0001 {
				t.Errorf("MM(%v).ToPoints() = %v, want %v", tt.mm, result, tt.expected)
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	tests := []float64{12, 8, 16, 24, 72, 0, 1}

	t.Run("Point->MM->Point", func(t *testing.T) {
		for _, value := range tests {
			original := NewPoint(value)
			converted := original.ToMM().ToPoints()
			if math.Abs(float64(original-converted)) > 0.000001 {
				t.Errorf("Round trip Point(%v) -> MM -> Point = %v, lost precision", original, converted)
			}
		}
	})

	t.Run("MM->Point->MM", func(t *testing.T) {
		for _, value := range tests {
			original := NewMM(value)
			converted := original.ToPoints().ToMM()
			if math.Abs(float64(original-converted)) > 0.000001 {
				t.Errorf("Round trip MM(%v) -> Point -> MM = %v, lost precision", original, converted)
			}
		}
	})
}

func TestFontSizeOperations(t *testing.T) {
	fontSize := NewFontSize(12)

	t.Run("FontSize to MM", func(t *testing.T) {
		result := fontSize.ToMM()
		expected := MM(12 * 25.4 / 72.0) // 12pt = 4.233333...mm
		if math.Abs(float64(result-expected)) > 0.000001 {
			t.Errorf("FontSize(12).ToMM() = %v, want %v", result, expected)
		}
	})

	t.Run("FontSize line height", func(t *testing.T) {
		result := fontSize.LineHeight(1.2)
		expected := fontSize.ToMM().Multiply(1.2)
		if math.Abs(float64(result-expected)) > 0.000001 {
			t.Errorf("FontSize(12).LineHeight(1.2) = %v, want %v", result, expected)
		}
	})

	t.Run("FontSize Float64", func(t *testing.T) {
		result := fontSize.Float64()
		expected := 12.0
		if result != expected {
			t.Errorf("FontSize(12).Float64() = %v, want %v", result, expected)
		}
	})
}

func TestUnitArithmetic(t *testing.T) {
	t.Run("Point arithmetic", func(t *testing.T) {
		p1 := NewPoint(10)
		p2 := NewPoint(5)

		sum := p1.Add(p2)
		if sum != NewPoint(15) {
			t.Errorf("Point(10).Add(Point(5)) = %v, want Point(15)", sum)
		}

		multiplied := p1.Multiply(2)
		if multiplied != NewPoint(20) {
			t.Errorf("Point(10).Multiply(2) = %v, want Point(20)", multiplied)
		}
	})

	t.Run("MM arithmetic", func(t *testing.T) {
		m1 := NewMM(10)
		m2 := NewMM(5)

		sum := m1.Add(m2)
		if sum != NewMM(15) {
			t.Errorf("MM(10).Add(MM(5)) = %v, want MM(15)", sum)
		}

		multiplied := m1.Multiply(2)
		if multiplied != NewMM(20) {
			t.Errorf("MM(10).Multiply(2) = %v, want MM(20)", multiplied)
		}
	})
}

func TestStringRepresentation(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{ String() string }
		expected string
	}{
		{"Point formatting", NewPoint(12.5), "12.50pt"},
		{"MM formatting", NewMM(10.75), "10.75mm"},
		{"FontSize formatting", NewFontSize(14), "14.0pt"},
		{"Rem formatting", NewRem(1.5), "1.50rem"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.value.String()
			if result != tt.expected {
				t.Errorf("%T.String() = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestConversionConstants(t *testing.T) {
	// Test that our constants are correct
	// 1 inch = 72 points = 25.4 mm

	t.Run("PointsToMM constant", func(t *testing.T) {
		// 72 points should equal 25.4 mm
		result := 72 * PointsToMM
		expected := 25.4
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("72 * PointsToMM = %v, want %v", result, expected)
		}
	})

	t.Run("MMToPoints constant", func(t *testing.T) {
		// 25.4 mm should equal 72 points
		result := 25.4 * MMToPoints
		expected := 72.0
		if math.Abs(result-expected) > 0.0001 {
			t.Errorf("25.4 * MMToPoints = %v, want %v", result, expected)
		}
	})

	t.Run("Constants are reciprocals", func(t *testing.T) {
		product := PointsToMM * MMToPoints
		expected := 1.0
		if math.Abs(product-expected) > 0.0001 {
			t.Errorf("PointsToMM * MMToPoints = %v, want %v", product, expected)
		}
	})

	t.Run("Rem conversion constants", func(t *testing.T) {
		// 1 rem = 12 points
		if RemToPoints != 12.0 {
			t.Errorf("RemToPoints = %v, want 12.0", RemToPoints)
		}

		// 1 rem = 4.233336 mm (12pt * 0.352778)
		expected := 12.0 * PointsToMM
		if math.Abs(RemToMM-expected) > 0.0001 {
			t.Errorf("RemToMM = %v, want %v", RemToMM, expected)
		}
	})
}

func TestRemConversions(t *testing.T) {
	tests := []struct {
		name      string
		rem       Rem
		expPoints Point
		expMM     MM
	}{
		{"1rem standard", NewRem(1), Point(12), MM(12 * 25.4 / 72.0)}, // 4.233333...
		{"2rem large", NewRem(2), Point(24), MM(24 * 25.4 / 72.0)},    // 8.466666...
		{"0.5rem small", NewRem(0.5), Point(6), MM(6 * 25.4 / 72.0)},  // 2.116666...
		{"0rem zero", NewRem(0), Point(0), MM(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test rem to points
			resultPoints := tt.rem.ToPoints()
			if math.Abs(float64(resultPoints-tt.expPoints)) > 0.0001 {
				t.Errorf("Rem(%v).ToPoints() = %v, want %v", tt.rem, resultPoints, tt.expPoints)
			}

			// Test rem to mm
			resultMM := tt.rem.ToMM()
			if math.Abs(float64(resultMM-tt.expMM)) > 0.000001 {
				t.Errorf("Rem(%v).ToMM() = %v, want %v", tt.rem, resultMM, tt.expMM)
			}

			// Test rem to font size
			fontSize := tt.rem.ToFontSize()
			expectedFontSize := FontSize(tt.expPoints)
			if fontSize != expectedFontSize {
				t.Errorf("Rem(%v).ToFontSize() = %v, want %v", tt.rem, fontSize, expectedFontSize)
			}
		})
	}
}

func TestRemArithmetic(t *testing.T) {
	r1 := NewRem(1.5)
	r2 := NewRem(0.5)

	t.Run("Rem addition", func(t *testing.T) {
		sum := r1.Add(r2)
		expected := NewRem(2.0)
		if sum != expected {
			t.Errorf("Rem(1.5).Add(Rem(0.5)) = %v, want %v", sum, expected)
		}
	})

	t.Run("Rem multiplication", func(t *testing.T) {
		multiplied := r1.Multiply(2)
		expected := NewRem(3.0)
		if multiplied != expected {
			t.Errorf("Rem(1.5).Multiply(2) = %v, want %v", multiplied, expected)
		}
	})
}
