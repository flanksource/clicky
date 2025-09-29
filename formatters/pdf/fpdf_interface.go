package pdf

// FpdfInterface defines the interface we need for table rendering
// This interface allows access to the full FPDF functionality through the underlying provider
//
// Unit Conventions:
// - Font sizes: Points (pt) - standard typography unit
// - Layout dimensions: Millimeters (mm) - PDF page layout unit
// - Margins and positioning: Millimeters (mm)
type FpdfInterface interface {
	// Core text and font methods used by table component
	SetFont(familyStr, styleStr string, size float64) // size in points
	SetTextColor(r, g, b int)
	GetFontSize() (ptSize, unitSize float64)                                                                         // ptSize in points, unitSize in current unit (typically mm)
	GetStringWidth(s string) float64                                                                                 // returns width in current unit (typically mm)
	Text(x, y float64, txtStr string)                                                                                // x, y in current unit (typically mm)
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

	// Basic PDF methods
	AddPage()
	SetAutoPageBreak(auto bool, margin float64) // margin in current unit (typically mm)

	// Font management methods that were missing
	AddFont(familyStr, styleStr, fileStr string)
	AddFontFromBytes(familyStr, styleStr string, jsonFileBytes, zFileBytes []byte)
	AddUTF8FontFromBytes(familyStr, styleStr string, bytes []byte)
	SetFontSize(size float64)

	// Extended text methods
	MultiCell(w, h float64, txtStr, borderStr, alignStr string, fill bool)
	Write(h float64, txtStr string)
	WriteLinkString(h float64, displayStr, targetStr string)

	// Page management
	PageCount() int
	PageNo() int
	SetPage(pageNum int)

	// Color and style
	SetFillColorArray(colorArray [3]int)
	SetTextColorArray(colorArray [3]int)
	SetDrawColorArray(colorArray [3]int)

	// Advanced positioning
	GetXY() (float64, float64)
	SetX(x float64)
	SetY(y float64)

	// Line styling
	SetLineWidth(width float64)
	SetLineCapStyle(styleStr string)
	SetLineJoinStyle(styleStr string)

	// Images
	ImageFromURL(urlStr string, x, y, w, h float64, flow bool, tp string, link int, linkStr string)
	ImageFromBytes(imgBuf []byte, x, y, w, h float64, flow bool, tp string, link int, linkStr string)

	// Generic interface to allow access to any other FPDF methods
	GetRawFpdf() interface{}
}

// FpdfWrapper wraps the underlying FPDF instance and implements our FpdfInterface
type FpdfWrapper struct {
	fpdf interface{} // The actual FPDF instance from Maroto
}

// WrapFpdf wraps any gofpdf-compatible interface into our FpdfInterface
func WrapFpdf(fpdf interface{}) FpdfInterface {
	return &FpdfWrapper{fpdf: fpdf}
}

// GetRawFpdf returns the underlying FPDF instance for advanced operations
func (w *FpdfWrapper) GetRawFpdf() interface{} {
	return w.fpdf
}

// Core text and font methods - delegate to underlying FPDF
func (w *FpdfWrapper) SetFont(familyStr, styleStr string, size float64) {
	if f, ok := w.fpdf.(interface{ SetFont(string, string, float64) }); ok {
		f.SetFont(familyStr, styleStr, size)
	}
}

func (w *FpdfWrapper) SetTextColor(r, g, b int) {
	if f, ok := w.fpdf.(interface{ SetTextColor(int, int, int) }); ok {
		f.SetTextColor(r, g, b)
	}
}

func (w *FpdfWrapper) GetFontSize() (ptSize, unitSize float64) {
	if f, ok := w.fpdf.(interface{ GetFontSize() (float64, float64) }); ok {
		return f.GetFontSize()
	}
	return 12.0, 4.23 // Default fallback
}

func (w *FpdfWrapper) GetStringWidth(s string) float64 {
	if f, ok := w.fpdf.(interface{ GetStringWidth(string) float64 }); ok {
		return f.GetStringWidth(s)
	}
	return 0
}

func (w *FpdfWrapper) Text(x, y float64, txtStr string) {
	if f, ok := w.fpdf.(interface {
		Text(float64, float64, string)
	}); ok {
		f.Text(x, y, txtStr)
	}
}

func (w *FpdfWrapper) CellFormat(w1, h float64, txtStr, borderStr string, ln int, alignStr string, fill bool, link int, linkStr string) {
	if f, ok := w.fpdf.(interface {
		CellFormat(float64, float64, string, string, int, string, bool, int, string)
	}); ok {
		f.CellFormat(w1, h, txtStr, borderStr, ln, alignStr, fill, link, linkStr)
	}
}

// Drawing methods
func (w *FpdfWrapper) SetDrawColor(r, g, b int) {
	if f, ok := w.fpdf.(interface{ SetDrawColor(int, int, int) }); ok {
		f.SetDrawColor(r, g, b)
	}
}

func (w *FpdfWrapper) SetFillColor(r, g, b int) {
	if f, ok := w.fpdf.(interface{ SetFillColor(int, int, int) }); ok {
		f.SetFillColor(r, g, b)
	}
}

func (w *FpdfWrapper) Line(x1, y1, x2, y2 float64) {
	if f, ok := w.fpdf.(interface {
		Line(float64, float64, float64, float64)
	}); ok {
		f.Line(x1, y1, x2, y2)
	}
}

func (w *FpdfWrapper) Rect(x, y, w1, h float64, styleStr string) {
	if f, ok := w.fpdf.(interface {
		Rect(float64, float64, float64, float64, string)
	}); ok {
		f.Rect(x, y, w1, h, styleStr)
	}
}

func (w *FpdfWrapper) SetDashPattern(dashArray []float64, dashPhase float64) {
	if f, ok := w.fpdf.(interface{ SetDashPattern([]float64, float64) }); ok {
		f.SetDashPattern(dashArray, dashPhase)
	}
}

// Position and margin methods
func (w *FpdfWrapper) GetMargins() (left, top, right, bottom float64) {
	if f, ok := w.fpdf.(interface {
		GetMargins() (float64, float64, float64, float64)
	}); ok {
		return f.GetMargins()
	}
	return 10, 10, 10, 10 // Default fallback
}

func (w *FpdfWrapper) GetPageSize() (width, height float64) {
	if f, ok := w.fpdf.(interface{ GetPageSize() (float64, float64) }); ok {
		return f.GetPageSize()
	}
	return 210, 297 // A4 default
}

func (w *FpdfWrapper) SetXY(x, y float64) {
	if f, ok := w.fpdf.(interface{ SetXY(float64, float64) }); ok {
		f.SetXY(x, y)
	}
}

func (w *FpdfWrapper) GetX() float64 {
	if f, ok := w.fpdf.(interface{ GetX() float64 }); ok {
		return f.GetX()
	}
	return 0
}

func (w *FpdfWrapper) GetY() float64 {
	if f, ok := w.fpdf.(interface{ GetY() float64 }); ok {
		return f.GetY()
	}
	return 0
}

// Basic PDF methods
func (w *FpdfWrapper) AddPage() {
	if f, ok := w.fpdf.(interface{ AddPage() }); ok {
		f.AddPage()
	}
}

func (w *FpdfWrapper) SetAutoPageBreak(auto bool, margin float64) {
	if f, ok := w.fpdf.(interface{ SetAutoPageBreak(bool, float64) }); ok {
		f.SetAutoPageBreak(auto, margin)
	}
}

// Font management methods
func (w *FpdfWrapper) AddFont(familyStr, styleStr, fileStr string) {
	if f, ok := w.fpdf.(interface{ AddFont(string, string, string) }); ok {
		f.AddFont(familyStr, styleStr, fileStr)
	}
}

func (w *FpdfWrapper) AddFontFromBytes(familyStr, styleStr string, jsonFileBytes, zFileBytes []byte) {
	if f, ok := w.fpdf.(interface {
		AddFontFromBytes(string, string, []byte, []byte)
	}); ok {
		f.AddFontFromBytes(familyStr, styleStr, jsonFileBytes, zFileBytes)
	}
}

func (w *FpdfWrapper) AddUTF8FontFromBytes(familyStr, styleStr string, bytes []byte) {
	if f, ok := w.fpdf.(interface{ AddUTF8FontFromBytes(string, string, []byte) }); ok {
		f.AddUTF8FontFromBytes(familyStr, styleStr, bytes)
	}
}

func (w *FpdfWrapper) SetFontSize(size float64) {
	if f, ok := w.fpdf.(interface{ SetFontSize(float64) }); ok {
		f.SetFontSize(size)
	}
}

// Extended text methods
func (w *FpdfWrapper) MultiCell(w1, h float64, txtStr, borderStr, alignStr string, fill bool) {
	if f, ok := w.fpdf.(interface {
		MultiCell(float64, float64, string, string, string, bool)
	}); ok {
		f.MultiCell(w1, h, txtStr, borderStr, alignStr, fill)
	}
}

func (w *FpdfWrapper) Write(h float64, txtStr string) {
	if f, ok := w.fpdf.(interface{ Write(float64, string) }); ok {
		f.Write(h, txtStr)
	}
}

func (w *FpdfWrapper) WriteLinkString(h float64, displayStr, targetStr string) {
	if f, ok := w.fpdf.(interface{ WriteLinkString(float64, string, string) }); ok {
		f.WriteLinkString(h, displayStr, targetStr)
	}
}

// Page management
func (w *FpdfWrapper) PageCount() int {
	if f, ok := w.fpdf.(interface{ PageCount() int }); ok {
		return f.PageCount()
	}
	return 1
}

func (w *FpdfWrapper) PageNo() int {
	if f, ok := w.fpdf.(interface{ PageNo() int }); ok {
		return f.PageNo()
	}
	return 1
}

func (w *FpdfWrapper) SetPage(pageNum int) {
	if f, ok := w.fpdf.(interface{ SetPage(int) }); ok {
		f.SetPage(pageNum)
	}
}

// Color and style arrays
func (w *FpdfWrapper) SetFillColorArray(colorArray [3]int) {
	if f, ok := w.fpdf.(interface{ SetFillColorArray([3]int) }); ok {
		f.SetFillColorArray(colorArray)
	}
}

func (w *FpdfWrapper) SetTextColorArray(colorArray [3]int) {
	if f, ok := w.fpdf.(interface{ SetTextColorArray([3]int) }); ok {
		f.SetTextColorArray(colorArray)
	}
}

func (w *FpdfWrapper) SetDrawColorArray(colorArray [3]int) {
	if f, ok := w.fpdf.(interface{ SetDrawColorArray([3]int) }); ok {
		f.SetDrawColorArray(colorArray)
	}
}

// Advanced positioning
func (w *FpdfWrapper) GetXY() (float64, float64) {
	if f, ok := w.fpdf.(interface{ GetXY() (float64, float64) }); ok {
		return f.GetXY()
	}
	return 0, 0
}

func (w *FpdfWrapper) SetX(x float64) {
	if f, ok := w.fpdf.(interface{ SetX(float64) }); ok {
		f.SetX(x)
	}
}

func (w *FpdfWrapper) SetY(y float64) {
	if f, ok := w.fpdf.(interface{ SetY(float64) }); ok {
		f.SetY(y)
	}
}

// Line styling
func (w *FpdfWrapper) SetLineWidth(width float64) {
	if f, ok := w.fpdf.(interface{ SetLineWidth(float64) }); ok {
		f.SetLineWidth(width)
	}
}

func (w *FpdfWrapper) SetLineCapStyle(styleStr string) {
	if f, ok := w.fpdf.(interface{ SetLineCapStyle(string) }); ok {
		f.SetLineCapStyle(styleStr)
	}
}

func (w *FpdfWrapper) SetLineJoinStyle(styleStr string) {
	if f, ok := w.fpdf.(interface{ SetLineJoinStyle(string) }); ok {
		f.SetLineJoinStyle(styleStr)
	}
}

// Images
func (w *FpdfWrapper) ImageFromURL(urlStr string, x, y, w1, h float64, flow bool, tp string, link int, linkStr string) {
	if f, ok := w.fpdf.(interface {
		ImageFromURL(string, float64, float64, float64, float64, bool, string, int, string)
	}); ok {
		f.ImageFromURL(urlStr, x, y, w1, h, flow, tp, link, linkStr)
	}
}

func (w *FpdfWrapper) ImageFromBytes(imgBuf []byte, x, y, w1, h float64, flow bool, tp string, link int, linkStr string) {
	if f, ok := w.fpdf.(interface {
		ImageFromBytes([]byte, float64, float64, float64, float64, bool, string, int, string)
	}); ok {
		f.ImageFromBytes(imgBuf, x, y, w1, h, flow, tp, link, linkStr)
	}
}
