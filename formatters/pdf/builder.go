package pdf

import (
	"bytes"
	"fmt"

	"github.com/flanksource/clicky/api"
	"github.com/go-pdf/fpdf"
)

type PageSize struct {
	api.Rectangle `json:"rectangle,omitempty"`
	Margins       api.Padding `json:"margins,omitempty"`
}

type Widget interface {
	// GetWidth returns the final width of the widget, used for centering against other widgets
	GetWidth() float64
	// GetHeight returns the final height of the widget, used for centering against other widgets
	GetHeight() float64
	// Draw draws the widget at the current position/page unless overridden by option
	Draw(b *Builder, opts ...DrawOptions) error
}

var Widgets []Widget = []Widget{Text{}, Table{}, Image{}}

type Builder struct {
	PageSize
	PageNumbers bool
	Header      api.Text
	Footer      api.Text
	currentPage int
	currentPos  api.Position
	pdf         *fpdf.Fpdf
	style       *StyleConverter
}

// NewBuilder creates a new PDF builder
func NewBuilder() *Builder {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	
	return &Builder{
		PageSize: PageSize{
			Rectangle: api.Rectangle{Width: 210, Height: 297}, // A4 in mm
			Margins:   api.Padding{Top: 10, Right: 10, Bottom: 10, Left: 10},
		},
		PageNumbers: false,
		currentPage: 0,
		currentPos:  api.Position{X: 10, Y: 10}, // Start with left margin
		pdf:         pdf,
		style:       NewStyleConverter(pdf),
	}
}

// AddPage adds a new page to the PDF
func (b *Builder) AddPage() {
	b.pdf.AddPage()
	b.currentPage++
	// Reset position to top-left with margins
	b.currentPos = api.Position{
		X: int(b.Margins.Left * 4.2333), // Convert rem to mm
		Y: int(b.Margins.Top * 4.2333),
	}
	
	// Add header if defined
	if !b.Header.IsEmpty() {
		b.writeHeader()
	}
}

// writeHeader writes the header text
func (b *Builder) writeHeader() {
	if b.Header.IsEmpty() {
		return
	}
	
	// Save current position and styling
	savedPos := b.currentPos
	savedState := b.style.SaveCurrentState()
	
	// Position header at top
	b.currentPos = api.Position{X: int(b.Margins.Left * 4.2333), Y: int(b.Margins.Top * 4.2333)}
	
	// Apply header styling and write text
	if b.Header.Class != (api.Class{}) {
		b.style.ApplyClassToPDF(b.Header.Class)
	} else if b.Header.Style != "" {
		b.style.ParseTailwindToPDF(b.Header.Style)
	}
	
	b.writeText(b.Header)
	
	// Restore position and styling
	b.currentPos = savedPos
	b.style.RestoreState(savedState)
	
	// Move current position below header
	b.currentPos.Y += int(b.style.GetTextHeight() * 2) // Add some spacing
}

// writeFooter writes the footer text (called by PDF library)
func (b *Builder) writeFooter() {
	if b.Footer.IsEmpty() {
		return
	}
	
	// Save current styling
	savedState := b.style.SaveCurrentState()
	
	// Position footer at bottom
	footerY := float64(b.PageSize.Height) - b.Margins.Bottom*4.2333
	b.pdf.SetY(footerY)
	b.pdf.SetX(b.Margins.Left * 4.2333)
	
	// Apply footer styling and write text
	if b.Footer.Class != (api.Class{}) {
		b.style.ApplyClassToPDF(b.Footer.Class)
	} else if b.Footer.Style != "" {
		b.style.ParseTailwindToPDF(b.Footer.Style)
	}
	
	b.writeText(b.Footer)
	
	// Add page numbers if enabled
	if b.PageNumbers {
		pageText := fmt.Sprintf("Page %d", b.currentPage)
		pageWidth := b.style.GetTextWidth(pageText)
		// Position at right margin
		b.pdf.SetX(float64(b.PageSize.Width) - b.Margins.Right*4.2333 - pageWidth)
		b.pdf.Cell(pageWidth, b.style.GetTextHeight(), pageText)
	}
	
	// Restore styling
	b.style.RestoreState(savedState)
}

// Write writes text at the current position
func (b *Builder) Write(text api.Text) *Builder {
	b.writeText(text)
	return b
}

// writeText writes api.Text with proper styling
func (b *Builder) writeText(text api.Text) {
	// Save current state
	savedState := b.style.SaveCurrentState()
	
	// Apply styling from Class or Style string
	if text.Class != (api.Class{}) {
		b.style.ApplyClassToPDF(text.Class)
	} else if text.Style != "" {
		b.style.ParseTailwindToPDF(text.Style)
	}
	
	// Position the text
	b.pdf.SetXY(float64(b.currentPos.X), float64(b.currentPos.Y))
	
	// Write the main content
	if text.Content != "" {
		textHeight := b.style.GetTextHeight()
		b.pdf.Cell(0, textHeight, text.Content)
		// Move to next line
		b.currentPos.Y += int(textHeight)
	}
	
	// Write children
	for _, child := range text.Children {
		b.writeText(child)
	}
	
	// Restore state
	b.style.RestoreState(savedState)
}

// MoveTo moves the current position
func (b *Builder) MoveTo(pos api.Position) *Builder {
	b.currentPos = pos
	return b
}

// MoveBy moves the current position by the given offset
func (b *Builder) MoveBy(dx, dy int) *Builder {
	b.currentPos.X += dx
	b.currentPos.Y += dy
	return b
}

// GetCurrentPosition returns the current drawing position
func (b *Builder) GetCurrentPosition() api.Position {
	return b.currentPos
}

// GetPDF returns the underlying fpdf instance for direct manipulation
func (b *Builder) GetPDF() *fpdf.Fpdf {
	return b.pdf
}

// GetStyleConverter returns the style converter for direct styling operations
func (b *Builder) GetStyleConverter() *StyleConverter {
	return b.style
}

// Output generates the final PDF content
func (b *Builder) Output() ([]byte, error) {
	// Ensure we have at least one page
	if b.currentPage == 0 {
		b.AddPage()
	}
	
	// Set up footer callback
	b.pdf.SetFooterFunc(func() {
		b.writeFooter()
	})
	
	var buf bytes.Buffer
	err := b.pdf.Output(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}
	
	return buf.Bytes(), nil
}

// DrawWidget draws a widget at the current position
func (b *Builder) DrawWidget(widget Widget, opts ...DrawOptions) error {
	return widget.Draw(b, opts...)
}

type drawOptions struct {
	Position api.Position
	Size     api.Rectangle
}

func WithPosition(x, y int) DrawOptions {
	return func(o *drawOptions) {
		o.Position = api.Position{X: x, Y: y}
	}
}

func WithSize(width, height float64) DrawOptions {
	return func(o *drawOptions) {
		o.Size = api.Rectangle{Width: int(width), Height: int(height)}
	}
}

type DrawOptions func(o *drawOptions)
