package api

import "fmt"

// Unit conversion constants
const (
	// 1 point = 25.4/72 mm (exact: 1/72 inch = 25.4mm/72)
	PointsToMM = 25.4 / 72.0
	// 1 mm = 72/25.4 points (exact reciprocal)
	MMToPoints = 72.0 / 25.4
)

// Point represents a measurement in typographic points
type Point float64

// ToMM converts points to millimeters
func (p Point) ToMM() float64 {
	return float64(p) * PointsToMM
}

// Float64 returns the point value as a float64
func (p Point) Float64() float64 {
	return float64(p)
}

// String returns a formatted string representation
func (p Point) String() string {
	return fmt.Sprintf("%.2fpt", float64(p))
}

// NewPoint creates a new Point value
func NewPoint(value float64) Point {
	return Point(value)
}