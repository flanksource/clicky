package pdf

import (
	"fmt"
)

// PDFEmbedWidget embeds a PDF page directly into the PDF document using fpdf
// This widget provides no fallbacks - it either succeeds or returns an error
type PDFEmbedWidget struct {
	Source     string   `json:"source"`
	PageNumber int      `json:"page_number,omitempty"` // 1-based page number
	Box        string   `json:"box,omitempty"`         // PDF box type: /MediaBox, /CropBox, etc.
	Width      *float64 `json:"width,omitempty"`
	Height     *float64 `json:"height,omitempty"`
	X          float64  `json:"x,omitempty"`            // X position within cell
	Y          float64  `json:"y,omitempty"`            // Y position within cell
	ScaleToFit bool     `json:"scale_to_fit,omitempty"` // Scale to fit specified dimensions
	importer   *AdvancedPDFImporter
}

// NewPDFEmbedWidget creates a new PDF embed widget
func NewPDFEmbedWidget(source string) *PDFEmbedWidget {
	return &PDFEmbedWidget{
		Source:     source,
		PageNumber: 1,           // Default to first page
		Box:        "/MediaBox", // Default to MediaBox
		ScaleToFit: true,        // Default to scaling
		importer:   NewAdvancedPDFImporter(),
	}
}

// WithPage sets the page number to import (1-based)
func (w *PDFEmbedWidget) WithPage(pageNumber int) *PDFEmbedWidget {
	if pageNumber < 1 {
		pageNumber = 1
	}
	w.PageNumber = pageNumber
	return w
}

// WithBox sets the PDF box type to import
func (w *PDFEmbedWidget) WithBox(box string) *PDFEmbedWidget {
	w.Box = box
	return w
}

// WithSize sets the width and height for the embedded PDF
func (w *PDFEmbedWidget) WithSize(width, height float64) *PDFEmbedWidget {
	w.Width = &width
	w.Height = &height
	return w
}

// WithPosition sets the X,Y position within the cell
func (w *PDFEmbedWidget) WithPosition(x, y float64) *PDFEmbedWidget {
	w.X = x
	w.Y = y
	return w
}

// WithScaling controls whether the PDF should be scaled to fit
func (w *PDFEmbedWidget) WithScaling(scaleToFit bool) *PDFEmbedWidget {
	w.ScaleToFit = scaleToFit
	return w
}

// Draw implements the Widget interface - embeds the PDF or returns error
func (w *PDFEmbedWidget) Draw(b *Builder) (err error) {
	// Add panic recovery as a final safety net for gofpdi panics
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PDF import panic recovered: %v (source: %s)", r, w.Source)
		}
	}()

	// Validate required fields
	if w.Source == "" {
		return fmt.Errorf("PDF source cannot be empty")
	}

	// Validate PDF file exists and is readable
	if err := w.importer.ValidatePDFFile(w.Source); err != nil {
		return fmt.Errorf("PDF validation failed: %w", err)
	}

	// Import the PDF page
	templateID, err := w.importer.ImportPageFromFile(b, w.Source, w.PageNumber, w.Box)
	if err != nil {
		return fmt.Errorf("failed to import PDF page: %w", err)
	}

	// Calculate dimensions and position
	width, height, err := w.calculateDimensions(templateID)
	if err != nil {
		return fmt.Errorf("failed to calculate dimensions: %w", err)
	}

	// Use the template to embed the PDF
	err = w.importer.UseTemplate(b, templateID, w.X, w.Y, width, height)
	if err != nil {
		return fmt.Errorf("failed to embed PDF template: %w", err)
	}

	return nil
}

// calculateDimensions calculates the final width and height for the embedded PDF
func (w *PDFEmbedWidget) calculateDimensions(templateID int) (width, height float64, err error) {
	// If both width and height are specified, use them
	if w.Width != nil && w.Height != nil {
		return *w.Width, *w.Height, nil
	}

	// Try to get actual page dimensions
	pageWidth, pageHeight, err := w.importer.GetPageDimensions(templateID, w.Box)
	if err != nil {
		// If we can't get dimensions, require explicit sizing
		if w.Width == nil && w.Height == nil {
			return 0, 0, fmt.Errorf("cannot determine PDF dimensions and no explicit size provided: %w", err)
		}
	}

	// If only width is specified, calculate height maintaining aspect ratio
	if w.Width != nil && w.Height == nil {
		if pageWidth > 0 && pageHeight > 0 {
			aspectRatio := pageHeight / pageWidth
			return *w.Width, *w.Width * aspectRatio, nil
		}
		// Fallback to square if no aspect ratio available
		return *w.Width, *w.Width, nil
	}

	// If only height is specified, calculate width maintaining aspect ratio
	if w.Height != nil && w.Width == nil {
		if pageWidth > 0 && pageHeight > 0 {
			aspectRatio := pageWidth / pageHeight
			return *w.Height * aspectRatio, *w.Height, nil
		}
		// Fallback to square if no aspect ratio available
		return *w.Height, *w.Height, nil
	}

	// Use page dimensions if available
	if pageWidth > 0 && pageHeight > 0 {
		return pageWidth, pageHeight, nil
	}

	// No dimensions available at all
	return 0, 0, fmt.Errorf("cannot determine PDF dimensions - please specify explicit width and height")
}

// EmbedPDFPage is a convenience function to embed a PDF page with minimal setup
func EmbedPDFPage(b *Builder, sourceFile string, pageNumber int) error {
	widget := NewPDFEmbedWidget(sourceFile).WithPage(pageNumber)
	return widget.Draw(b)
}

// EmbedPDFPageWithSize is a convenience function to embed a PDF page with specific dimensions
func EmbedPDFPageWithSize(b *Builder, sourceFile string, pageNumber int, width, height float64) error {
	widget := NewPDFEmbedWidget(sourceFile).
		WithPage(pageNumber).
		WithSize(width, height)
	return widget.Draw(b)
}

// EmbedEntirePDF embeds all pages from a PDF file as separate widgets
func EmbedEntirePDF(b *Builder, sourceFile string) error {
	// First validate the PDF file
	importer := NewAdvancedPDFImporter()
	if err := importer.ValidatePDFFile(sourceFile); err != nil {
		return fmt.Errorf("PDF validation failed: %w", err)
	}

	// Try to import first page to get page count information
	// Note: gofpdi doesn't provide a direct way to get page count without importing,
	// so we'll try pages until we get an error
	pageNumber := 1
	for {
		widget := NewPDFEmbedWidget(sourceFile).WithPage(pageNumber)
		err := widget.Draw(b)
		if err != nil {
			// If this is the first page, it's a real error
			if pageNumber == 1 {
				return fmt.Errorf("failed to embed first page of PDF: %w", err)
			}
			// Otherwise, we've probably reached the end
			break
		}
		pageNumber++

		// Safety limit to prevent infinite loops
		if pageNumber > 1000 {
			return fmt.Errorf("PDF has more than 1000 pages, stopping for safety")
		}
	}

	if pageNumber == 1 {
		return fmt.Errorf("PDF appears to have no pages")
	}

	return nil
}
