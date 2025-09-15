package pdf

import (
	"fmt"
	"io"
	"os"

	"github.com/flanksource/maroto/v2/pkg/fpdf"
	"github.com/jung-kurt/gofpdf"
	"github.com/jung-kurt/gofpdf/contrib/gofpdi"
)

// AdvancedPDFImporter provides PDF import functionality using direct fpdf access
type AdvancedPDFImporter struct {
	importer *gofpdi.Importer
}

// NewAdvancedPDFImporter creates a new advanced PDF importer
func NewAdvancedPDFImporter() *AdvancedPDFImporter {
	return &AdvancedPDFImporter{
		importer: gofpdi.NewImporter(),
	}
}

// ImportPageFromFile imports a PDF page from a file and returns the template ID
func (api *AdvancedPDFImporter) ImportPageFromFile(b *Builder, sourceFile string, pageNumber int, box string) (int, error) {
	// Validate inputs
	if sourceFile == "" {
		return 0, fmt.Errorf("source file path cannot be empty")
	}
	if pageNumber < 1 {
		return 0, fmt.Errorf("page number must be >= 1, got %d", pageNumber)
	}
	if box == "" {
		box = "/MediaBox" // Default to MediaBox
	}

	// Check if file exists
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		return 0, fmt.Errorf("PDF file not found: %s", sourceFile)
	}

	// Get fpdf instance from maroto
	fpdfWrapper, ok := fpdf.GetFpdfFromMaroto(b.maroto)
	if !ok {
		return 0, fmt.Errorf("cannot access fpdf instance from maroto - PDF import requires direct fpdf access")
	}

	// Cast to concrete gofpdf.Fpdf type (since maroto uses gofpdf.NewCustom)
	fpdfInstance, ok := fpdfWrapper.(*gofpdf.Fpdf)
	if !ok {
		return 0, fmt.Errorf("fpdf instance is not a *gofpdf.Fpdf - cannot use with gofpdi")
	}

	// Import the page
	templateID := api.importer.ImportPage(fpdfInstance, sourceFile, pageNumber, box)
	if templateID == 0 {
		return 0, fmt.Errorf("failed to import page %d from %s (box: %s) - gofpdi returned template ID 0", pageNumber, sourceFile, box)
	}

	return templateID, nil
}

// ImportPageFromStream imports a PDF page from a stream and returns the template ID
func (api *AdvancedPDFImporter) ImportPageFromStream(b *Builder, stream io.ReadSeeker, pageNumber int, box string) (int, error) {
	// Validate inputs
	if stream == nil {
		return 0, fmt.Errorf("stream cannot be nil")
	}
	if pageNumber < 1 {
		return 0, fmt.Errorf("page number must be >= 1, got %d", pageNumber)
	}
	if box == "" {
		box = "/MediaBox" // Default to MediaBox
	}

	// Get fpdf instance from maroto
	fpdfWrapper, ok := fpdf.GetFpdfFromMaroto(b.maroto)
	if !ok {
		return 0, fmt.Errorf("cannot access fpdf instance from maroto - PDF import requires direct fpdf access")
	}

	// Cast to concrete gofpdf.Fpdf type
	fpdfInstance, ok := fpdfWrapper.(*gofpdf.Fpdf)
	if !ok {
		return 0, fmt.Errorf("fpdf instance is not a *gofpdf.Fpdf - cannot use with gofpdi")
	}

	// Import the page
	templateID := api.importer.ImportPageFromStream(fpdfInstance, &stream, pageNumber, box)
	if templateID == 0 {
		return 0, fmt.Errorf("failed to import page %d from stream (box: %s)", pageNumber, box)
	}

	return templateID, nil
}

// UseTemplate draws the imported template at the specified position and size
func (api *AdvancedPDFImporter) UseTemplate(b *Builder, templateID int, x, y, width, height float64) error {
	if templateID == 0 {
		return fmt.Errorf("invalid template ID: %d", templateID)
	}

	// Get fpdf instance from maroto
	fpdfWrapper, ok := fpdf.GetFpdfFromMaroto(b.maroto)
	if !ok {
		return fmt.Errorf("cannot access fpdf instance from maroto - PDF template usage requires direct fpdf access")
	}

	// Cast to concrete gofpdf.Fpdf type
	fpdfInstance, ok := fpdfWrapper.(*gofpdf.Fpdf)
	if !ok {
		return fmt.Errorf("fpdf instance is not a *gofpdf.Fpdf - cannot use with gofpdi")
	}

	// Use the template
	api.importer.UseImportedTemplate(fpdfInstance, templateID, x, y, width, height)

	// Note: UseImportedTemplate doesn't return an error in the gofpdi library,
	// but we could add validation here if needed

	return nil
}

// GetPageDimensions returns the dimensions of an imported page
func (api *AdvancedPDFImporter) GetPageDimensions(templateID int, box string) (width, height float64, err error) {
	if templateID == 0 {
		return 0, 0, fmt.Errorf("invalid template ID: %d", templateID)
	}
	if box == "" {
		box = "/MediaBox"
	}

	// Get page sizes from the importer
	pageSizes := api.importer.GetPageSizes()
	if pageSizes == nil {
		return 0, 0, fmt.Errorf("no page sizes available - make sure a page has been imported first")
	}

	// Find the dimensions for this template
	// Note: This is a simplified approach. In practice, we'd need to map templateID to page number
	// For now, we'll return reasonable defaults and let the caller handle sizing
	for pageNum, boxes := range pageSizes {
		if boxData, exists := boxes[box]; exists {
			if w, hasW := boxData["w"]; hasW {
				if h, hasH := boxData["h"]; hasH {
					return w, h, nil
				}
			}
		}
		// Just use first page if we find any
		if pageNum > 0 {
			break
		}
	}

	return 0, 0, fmt.Errorf("could not find dimensions for template %d with box %s", templateID, box)
}

// ImportAndUseTemplate is a convenience method that imports and immediately uses a template
func (api *AdvancedPDFImporter) ImportAndUseTemplate(b *Builder, sourceFile string, pageNumber int, x, y, width, height float64) error {
	templateID, err := api.ImportPageFromFile(b, sourceFile, pageNumber, "/MediaBox")
	if err != nil {
		return fmt.Errorf("failed to import page: %w", err)
	}

	err = api.UseTemplate(b, templateID, x, y, width, height)
	if err != nil {
		return fmt.Errorf("failed to use template: %w", err)
	}

	return nil
}

// ValidatePDFFile validates that a PDF file can be read and processed
func (api *AdvancedPDFImporter) ValidatePDFFile(sourceFile string) error {
	if sourceFile == "" {
		return fmt.Errorf("source file path cannot be empty")
	}

	// Check if file exists
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		return fmt.Errorf("PDF file not found: %s", sourceFile)
	}

	// Try to open and read the file
	file, err := os.Open(sourceFile)
	if err != nil {
		return fmt.Errorf("cannot open PDF file: %w", err)
	}
	defer file.Close()

	// Read a small portion to check it's actually a PDF
	header := make([]byte, 4)
	_, err = file.Read(header)
	if err != nil {
		return fmt.Errorf("cannot read PDF file header: %w", err)
	}

	if string(header) != "%PDF" {
		return fmt.Errorf("file %s does not appear to be a valid PDF (missing PDF header)", sourceFile)
	}

	return nil
}
