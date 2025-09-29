package pdf

import "fmt"

// Unit conversion constants
const (
	// 1 point = 25.4/72 mm (exact: 1/72 inch = 25.4mm/72)
	PointsToMM = 25.4 / 72.0
	// 1 mm = 72/25.4 points (exact reciprocal)
	MMToPoints = 72.0 / 25.4
	// 1 rem = 12 points (common web default)
	RemToPoints = 12.0
	// 1 rem = 4.233333... mm (12pt converted to mm)
	RemToMM = RemToPoints * PointsToMM
)

// Point represents a measurement in typographic points
type Point float64

// MM represents a measurement in millimeters
type MM float64

// ToMM converts points to millimeters
func (p Point) ToMM() MM {
	return MM(float64(p) * PointsToMM)
}

// ToPoints converts millimeters to points
func (m MM) ToPoints() Point {
	return Point(float64(m) * MMToPoints)
}

// Float64 returns the point value as a float64
func (p Point) Float64() float64 {
	return float64(p)
}

// Float64 returns the mm value as a float64
func (m MM) Float64() float64 {
	return float64(m)
}

// String returns a formatted string representation
func (p Point) String() string {
	return fmt.Sprintf("%.2fpt", float64(p))
}

// String returns a formatted string representation
func (m MM) String() string {
	return fmt.Sprintf("%.2fmm", float64(m))
}

// Add adds two Point values
func (p Point) Add(other Point) Point {
	return p + other
}

// Add adds two MM values
func (m MM) Add(other MM) MM {
	return m + other
}

// Multiply multiplies a Point by a scalar
func (p Point) Multiply(factor float64) Point {
	return Point(float64(p) * factor)
}

// Multiply multiplies an MM by a scalar
func (m MM) Multiply(factor float64) MM {
	return MM(float64(m) * factor)
}

// NewPoint creates a new Point value
func NewPoint(value float64) Point {
	return Point(value)
}

// NewMM creates a new MM value
func NewMM(value float64) MM {
	return MM(value)
}

// FontSize represents a font size in points (standard unit for typography)
type FontSize Point

// ToMM converts font size to millimeters for layout calculations
func (f FontSize) ToMM() MM {
	return Point(f).ToMM()
}

// Float64 returns the font size as a float64 in points
func (f FontSize) Float64() float64 {
	return float64(f)
}

// String returns a formatted string representation
func (f FontSize) String() string {
	return fmt.Sprintf("%.1fpt", float64(f))
}

// LineHeight calculates line height with a given multiplier
func (f FontSize) LineHeight(multiplier float64) MM {
	return f.ToMM().Multiply(multiplier)
}

// NewFontSize creates a new FontSize value
func NewFontSize(value float64) FontSize {
	return FontSize(value)
}

// Rem represents a measurement in CSS rem units
type Rem float64

// ToPoints converts rem to points
func (r Rem) ToPoints() Point {
	return Point(float64(r) * RemToPoints)
}

// ToMM converts rem to millimeters
func (r Rem) ToMM() MM {
	return MM(float64(r) * RemToMM)
}

// ToFontSize converts rem to FontSize (assuming rem is used for font sizing)
func (r Rem) ToFontSize() FontSize {
	return FontSize(r.ToPoints())
}

// Float64 returns the rem value as a float64
func (r Rem) Float64() float64 {
	return float64(r)
}

// String returns a formatted string representation
func (r Rem) String() string {
	return fmt.Sprintf("%.2frem", float64(r))
}

// Add adds two Rem values
func (r Rem) Add(other Rem) Rem {
	return r + other
}

// Multiply multiplies a Rem by a scalar
func (r Rem) Multiply(factor float64) Rem {
	return Rem(float64(r) * factor)
}

// NewRem creates a new Rem value
func NewRem(value float64) Rem {
	return Rem(value)
}
