package pdf

// FpdfInterface defines the interface we need for table rendering
// This is our own copy focused on the methods we actually use
//
// Unit Conventions:
// - Font sizes: Points (pt) - standard typography unit
// - Layout dimensions: Millimeters (mm) - PDF page layout unit
// - Margins and positioning: Millimeters (mm)
type FpdfInterface interface {
	// Font and text methods
	SetFont(familyStr, styleStr string, size float64) // size in points
	SetTextColor(r, g, b int)
	GetFontSize() (ptSize, unitSize float64) // ptSize in points, unitSize in current unit (typically mm)
	GetStringWidth(s string) float64         // returns width in current unit (typically mm)
	Text(x, y float64, txtStr string)        // x, y in current unit (typically mm)
	CellFormat(w, h float64, txtStr, borderStr string, ln int, alignStr string, fill bool, link int, linkStr string) // w, h in current unit (typically mm)

	// Drawing methods
	SetDrawColor(r, g, b int)
	SetFillColor(r, g, b int)
	Line(x1, y1, x2, y2 float64)
	Rect(x, y, w, h float64, styleStr string)
	SetDashPattern(dashArray []float64, dashPhase float64)

	// Position and margin methods
	GetMargins() (left, top, right, bottom float64) // returns margins in current unit (typically mm)
	GetPageSize() (width, height float64)           // returns page size in current unit (typically mm)
	SetXY(x, y float64)                             // x, y in current unit (typically mm)
	GetX() float64                                  // returns X position in current unit (typically mm)
	GetY() float64                                  // returns Y position in current unit (typically mm)

	// Basic PDF methods we might need
	AddPage()
	SetAutoPageBreak(auto bool, margin float64) // margin in current unit (typically mm)
}

// WrapFpdf wraps any gofpdf-compatible interface into our FpdfInterface
func WrapFpdf(fpdf interface{}) FpdfInterface {
	// Type assert to our interface - this will panic if the interface doesn't match
	// but that's acceptable since it indicates a programming error
	return fpdf.(FpdfInterface)
}
