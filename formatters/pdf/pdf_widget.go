package pdf

import (
	"fmt"
	"os"
)

// PDFWidget renders a PDF page as a widget in the PDF
type PDFWidget struct {
	Source     string   `json:"source"`
	PageNumber int      `json:"page_number,omitempty"` // 1-based page number
	Width      *float64 `json:"width,omitempty"`
	Height     *float64 `json:"height,omitempty"`
	AltText    string   `json:"alt_text,omitempty"`
}

// NewPDFWidget creates a new PDF widget
func NewPDFWidget(source string) *PDFWidget {
	return &PDFWidget{
		Source:     source,
		PageNumber: 1, // Default to first page
	}
}

// WithPage sets the page number to import (1-based)
func (w *PDFWidget) WithPage(pageNumber int) *PDFWidget {
	w.PageNumber = pageNumber
	return w
}

// WithSize sets the width and height
func (w *PDFWidget) WithSize(width, height float64) *PDFWidget {
	w.Width = &width
	w.Height = &height
	return w
}

// WithAltText sets alternative text
func (w *PDFWidget) WithAltText(altText string) *PDFWidget {
	w.AltText = altText
	return w
}

// Draw implements the Widget interface
func (w *PDFWidget) Draw(b *Builder) error {
	if w.Source == "" {
		return fmt.Errorf("PDF source cannot be empty")
	}

	// Check if file exists
	if _, err := os.Stat(w.Source); os.IsNotExist(err) {
		return fmt.Errorf("PDF file not found: %s", w.Source)
	}

	// Get page number (default to 1)
	pageNumber := w.PageNumber
	if pageNumber < 1 {
		pageNumber = 1
	}

	// Use the new PDF embed widget for actual embedding
	embedWidget := NewPDFEmbedWidget(w.Source).WithPage(pageNumber)

	if w.Width != nil && w.Height != nil {
		embedWidget = embedWidget.WithSize(*w.Width, *w.Height)
	}

	return embedWidget.Draw(b)
}
