package pdf

import (
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/go-pdf/fpdf"
)

// StyleConverter handles converting api.Class and Tailwind styles to PDF properties
type StyleConverter struct {
	pdf *fpdf.Fpdf
}

// NewStyleConverter creates a new style converter for the given PDF instance
func NewStyleConverter(pdf *fpdf.Fpdf) *StyleConverter {
	return &StyleConverter{pdf: pdf}
}

// ApplyClassToPDF applies api.Class properties to the PDF instance
func (s *StyleConverter) ApplyClassToPDF(class api.Class) {
	// Apply font properties
	if class.Font != nil {
		s.applyFontProperties(*class.Font)
	}

	// Apply colors
	if class.Foreground != nil {
		s.applyTextColor(*class.Foreground)
	}
	
	if class.Background != nil {
		s.applyFillColor(*class.Background)
	}
}

// applyFontProperties applies font properties from api.Font to PDF
func (s *StyleConverter) applyFontProperties(font api.Font) {
	// Build font style string
	var style string
	if font.Bold {
		style += "B"
	}
	if font.Italic {
		style += "I"
	}
	if font.Underline {
		style += "U"
	}

	// Convert font size from rem to points (1rem = 16px = 12pt default)
	fontSize := 12.0 // Default font size in points
	if font.Size > 0 {
		fontSize = font.Size * 12 // Convert rem to points (assuming 1rem = 12pt)
	}

	// Set font
	s.pdf.SetFont("Arial", style, fontSize)

	// Apply additional styling
	if font.Faint {
		// Set text color to a lighter gray for faint effect
		s.pdf.SetTextColor(128, 128, 128)
	}
}

// applyTextColor applies foreground color to PDF
func (s *StyleConverter) applyTextColor(color api.Color) {
	r, g, b := s.hexToRGB(color.Hex)
	s.pdf.SetTextColor(r, g, b)
}

// applyFillColor applies background color to PDF
func (s *StyleConverter) applyFillColor(color api.Color) {
	r, g, b := s.hexToRGB(color.Hex)
	s.pdf.SetFillColor(r, g, b)
}

// hexToRGB converts hex color to RGB values
func (s *StyleConverter) hexToRGB(hex string) (int, int, int) {
	// Remove # if present
	hex = strings.TrimPrefix(hex, "#")
	
	if len(hex) != 6 {
		return 0, 0, 0 // Default to black for invalid colors
	}

	// Parse RGB components
	r, err := strconv.ParseInt(hex[0:2], 16, 0)
	if err != nil {
		r = 0
	}
	g, err := strconv.ParseInt(hex[2:4], 16, 0)
	if err != nil {
		g = 0
	}
	b, err := strconv.ParseInt(hex[4:6], 16, 0)
	if err != nil {
		b = 0
	}

	return int(r), int(g), int(b)
}

// ParseTailwindToPDF parses Tailwind style strings and applies them to PDF
func (s *StyleConverter) ParseTailwindToPDF(styles string) {
	if styles == "" {
		return
	}

	// Use existing ResolveStyles to convert Tailwind to Class
	class := api.ResolveStyles(styles)
	
	// Apply the resolved class to PDF
	s.ApplyClassToPDF(class)
}

// GetTextWidth calculates the width of text with current font settings
func (s *StyleConverter) GetTextWidth(text string) float64 {
	return s.pdf.GetStringWidth(text)
}

// GetTextHeight calculates the height of text with current font settings
func (s *StyleConverter) GetTextHeight() float64 {
	_, fontSize := s.pdf.GetFontSize()
	return fontSize * 0.352778 // Convert points to mm (1pt = 0.352778mm)
}

// ApplyPadding applies padding from api.Padding by adjusting current position
func (s *StyleConverter) ApplyPadding(padding api.Padding) (leftPadding, topPadding float64) {
	// Convert rem to mm (assuming 1rem = 4.2333mm, which is 16px at 96dpi)
	const remToMM = 4.2333
	
	leftPadding = padding.Left * remToMM
	topPadding = padding.Top * remToMM
	
	return leftPadding, topPadding
}

// DrawBorders draws borders from api.Borders around a rectangle
func (s *StyleConverter) DrawBorders(x, y, width, height float64, borders *api.Borders) {
	if borders == nil {
		return
	}

	// Draw each border if defined
	if borders.Top.Width > 0 {
		s.drawLine(x, y, x+width, y, borders.Top)
	}
	if borders.Right.Width > 0 {
		s.drawLine(x+width, y, x+width, y+height, borders.Right)
	}
	if borders.Bottom.Width > 0 {
		s.drawLine(x, y+height, x+width, y+height, borders.Bottom)
	}
	if borders.Left.Width > 0 {
		s.drawLine(x, y, x, y+height, borders.Left)
	}
}

// drawLine draws a single line with the specified properties
func (s *StyleConverter) drawLine(x1, y1, x2, y2 float64, line api.Line) {
	// Set line width
	s.pdf.SetLineWidth(line.Width)

	// Set line color
	r, g, b := s.hexToRGB(line.Color.Hex)
	s.pdf.SetDrawColor(r, g, b)

	// TODO: Handle line styles (solid, dashed, dotted, etc.)
	// fpdf doesn't have built-in dash patterns, would need custom implementation

	// Draw the line
	s.pdf.Line(x1, y1, x2, y2)
}

// SaveCurrentState saves the current PDF styling state
func (s *StyleConverter) SaveCurrentState() PDFState {
	r, g, b := s.pdf.GetTextColor()
	fr, fg, fb := s.pdf.GetFillColor()
	dr, dg, db := s.pdf.GetDrawColor()
	_, fontSize := s.pdf.GetFontSize()
	lineWidth := s.pdf.GetLineWidth()

	return PDFState{
		TextColor:  [3]int{r, g, b},
		FillColor:  [3]int{fr, fg, fb},
		DrawColor:  [3]int{dr, dg, db},
		FontSize:   fontSize,
		LineWidth:  lineWidth,
	}
}

// RestoreState restores PDF styling state
func (s *StyleConverter) RestoreState(state PDFState) {
	s.pdf.SetTextColor(state.TextColor[0], state.TextColor[1], state.TextColor[2])
	s.pdf.SetFillColor(state.FillColor[0], state.FillColor[1], state.FillColor[2])
	s.pdf.SetDrawColor(state.DrawColor[0], state.DrawColor[1], state.DrawColor[2])
	s.pdf.SetFont("Arial", "", state.FontSize)
	s.pdf.SetLineWidth(state.LineWidth)
}

// PDFState represents the state of PDF styling properties
type PDFState struct {
	TextColor [3]int
	FillColor [3]int
	DrawColor [3]int
	FontSize  float64
	LineWidth float64
}