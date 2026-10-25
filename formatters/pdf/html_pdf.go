package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flanksource/commons/logger"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

// Use registers the "pdf" and "html-pdf" formats. The formatters package
// ships no PDF renderer, so only binaries that call this compile one.
func Use() {
	htmlPdf := NewHTMLPDFFormatter()
	formatters.RegisterFormatter("html-pdf", htmlPdf.Format)
	formatters.RegisterFormatter("pdf", htmlPdf.Format)
}

// HTMLPDFFormatter handles HTML-to-PDF conversion using ChromiumConverter
type HTMLPDFFormatter struct {
	htmlFormatter *formatters.HTMLFormatter
	converter     *ChromiumConverter
}

// NewHTMLPDFFormatter creates a new HTML-PDF formatter
func NewHTMLPDFFormatter() *HTMLPDFFormatter {
	htmlFormatter := formatters.NewHTMLFormatter()
	htmlFormatter.IsPDFMode = true // Enable PDF-specific optimizations

	return &HTMLPDFFormatter{
		htmlFormatter: htmlFormatter,
		converter:     NewChromiumConverter(),
	}
}

// ToPrettyData converts various input types to PrettyData
func (f *HTMLPDFFormatter) ToPrettyData(data interface{}) (*api.PrettyData, error) {
	return formatters.ToPrettyData(data)
}

// Format formats data as PDF by first rendering as HTML, then converting with Chromium
func (f *HTMLPDFFormatter) Format(data interface{}, opts formatters.FormatOptions) (string, error) {
	// Check if ChromiumConverter is available
	if !f.converter.IsAvailable() {
		return "", fmt.Errorf("Chrome/Chromium not found - required for HTML-PDF conversion")
	}

	if prettData, ok := data.(*api.PrettyData); ok {
		return f.FormatPrettyData(prettData)
	}

	// Generate HTML using the HTML formatter
	htmlContent, err := f.htmlFormatter.Format(data, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML: %w", err)
	}

	// Create temporary HTML file
	tempHTMLFile, err := os.CreateTemp("", "clicky_*.html")
	if err != nil {
		return "", fmt.Errorf("failed to create temp HTML file: %w", err)
	}
	defer func() {
		if err := os.Remove(tempHTMLFile.Name()); err != nil {
			logger.Errorf("failed to remove temp HTML file: %v", err)
		}
	}()

	// Write HTML content to temp file
	if _, err := tempHTMLFile.WriteString(htmlContent); err != nil {
		_ = tempHTMLFile.Close()
		return "", fmt.Errorf("failed to write HTML to temp file: %w", err)
	}
	_ = tempHTMLFile.Close()

	// Create temporary PDF file
	tempPDFFile, err := os.CreateTemp("", "clicky_*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	_ = tempPDFFile.Close() // Close immediately, we just need the path
	defer func() {
		if err := os.Remove(tempPDFFile.Name()); err != nil {
			logger.Errorf("failed to remove temp PDF file: %v", err)
		}
	}()

	// Convert HTML to PDF using ChromiumConverter
	ctx := context.Background()
	options := DefaultConvertOptions()
	options.Format = "pdf"

	err = f.converter.Convert(ctx, tempHTMLFile.Name(), tempPDFFile.Name(), options)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to PDF: %w", err)
	}

	// Read the generated PDF file
	pdfBytes, err := os.ReadFile(tempPDFFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read generated PDF: %w", err)
	}

	return string(pdfBytes), nil
}

// FormatToFile formats data and writes PDF directly to a file
func (f *HTMLPDFFormatter) FormatToFile(data interface{}, outputPath string) error {
	// Check if ChromiumConverter is available
	if !f.converter.IsAvailable() {
		return fmt.Errorf("Chrome/Chromium not found - required for HTML-PDF conversion")
	}

	// Generate HTML using the HTML formatter
	htmlContent, err := f.htmlFormatter.Format(data, formatters.FormatOptions{})
	if err != nil {
		return fmt.Errorf("failed to generate HTML: %w", err)
	}

	// Create temporary HTML file
	tempHTMLFile, err := os.CreateTemp("", "clicky_*.html")
	if err != nil {
		return fmt.Errorf("failed to create temp HTML file: %w", err)
	}
	defer func() {
		if err := os.Remove(tempHTMLFile.Name()); err != nil {
			logger.Errorf("failed to remove temp HTML file: %v", err)
		}
	}()

	// Write HTML content to temp file
	if _, err := tempHTMLFile.WriteString(htmlContent); err != nil {
		_ = tempHTMLFile.Close()
		return fmt.Errorf("failed to write HTML to temp file: %w", err)
	}
	_ = tempHTMLFile.Close()

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Convert HTML to PDF using ChromiumConverter
	ctx := context.Background()
	options := DefaultConvertOptions()
	options.Format = "pdf"

	err = f.converter.Convert(ctx, tempHTMLFile.Name(), outputPath, options)
	if err != nil {
		return fmt.Errorf("failed to convert HTML to PDF: %w", err)
	}

	return nil
}

// FormatPrettyData formats PrettyData as PDF by first rendering as HTML, then converting with Chromium
func (f *HTMLPDFFormatter) FormatPrettyData(data *api.PrettyData) (string, error) {
	// Check if ChromiumConverter is available
	if !f.converter.IsAvailable() {
		return "", fmt.Errorf("Chrome/Chromium not found - required for HTML-PDF conversion")
	}

	// Generate HTML using the HTML formatter's FormatPrettyData method
	htmlContent, err := f.htmlFormatter.FormatPrettyData(data)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML: %w", err)
	}

	// Create temporary HTML file
	tempHTMLFile, err := os.CreateTemp("", "clicky_*.html")
	if err != nil {
		return "", fmt.Errorf("failed to create temp HTML file: %w", err)
	}
	defer func() {
		if err := os.Remove(tempHTMLFile.Name()); err != nil {
			logger.Errorf("failed to remove temp HTML file: %v", err)
		}
	}()

	// Write HTML content to temp file
	if _, err := tempHTMLFile.WriteString(htmlContent); err != nil {
		_ = tempHTMLFile.Close()
		return "", fmt.Errorf("failed to write HTML to temp file: %w", err)
	}
	_ = tempHTMLFile.Close()

	// Create temporary PDF file
	tempPDFFile, err := os.CreateTemp("", "clicky_*.pdf")
	if err != nil {
		return "", fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	_ = tempPDFFile.Close() // Close immediately, we just need the path
	defer func() {
		if err := os.Remove(tempPDFFile.Name()); err != nil {
			logger.Errorf("failed to remove temp PDF file: %v", err)
		}
	}()

	// Convert HTML to PDF using ChromiumConverter
	ctx := context.Background()
	options := DefaultConvertOptions()
	options.Format = "pdf"

	err = f.converter.Convert(ctx, tempHTMLFile.Name(), tempPDFFile.Name(), options)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to PDF: %w", err)
	}

	// Read the generated PDF file
	pdfBytes, err := os.ReadFile(tempPDFFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read generated PDF: %w", err)
	}

	return string(pdfBytes), nil
}
