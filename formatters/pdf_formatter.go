package formatters

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// PDFFormatter handles PDF formatting using HTML-to-PDF conversion
type PDFFormatter struct {
	// UseRod determines whether to use Rod/Chromium (true) or fall back to legacy gofpdf (false)
	UseRod bool
	// LegacyFormatter is the fallback formatter
	LegacyFormatter *PDFLegacyFormatter
}

// NewPDFFormatter creates a new PDF formatter
func NewPDFFormatter() *PDFFormatter {
	return &PDFFormatter{
		UseRod:          true, // Default to using Rod/Chromium
		LegacyFormatter: NewPDFLegacyFormatter(),
	}
}

// NewPDFFormatterLegacy creates a new PDF formatter that uses the legacy gofpdf approach
func NewPDFFormatterLegacy() *PDFFormatter {
	return &PDFFormatter{
		UseRod:          false,
		LegacyFormatter: NewPDFLegacyFormatter(),
	}
}

// Format formats PrettyData as PDF
func (f *PDFFormatter) Format(data *api.PrettyData) (string, error) {
	if !f.UseRod {
		// Fall back to legacy formatter
		return f.LegacyFormatter.Format(data)
	}

	// Try to use Rod/Chromium for PDF generation
	pdfContent, err := f.formatHTMLToPDFWithRod(data)
	if err != nil {
		// Fall back to legacy formatter on error
		return f.LegacyFormatter.Format(data)
	}

	return pdfContent, nil
}

// formatHTMLToPDFWithRod formats PrettyData as PDF using Rod/Chromium
func (f *PDFFormatter) formatHTMLToPDFWithRod(data *api.PrettyData) (string, error) {
	// Generate HTML using the HTML formatter
	htmlFormatter := NewHTMLFormatter()
	htmlContent, err := htmlFormatter.Format(data)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML for PDF conversion: %w", err)
	}

	// Optimize HTML for PDF output
	pdfOptimizedHTML := f.optimizeHTMLForPDF(htmlContent)

	// Convert HTML to PDF using Rod
	pdfBytes, err := f.convertHTMLToPDFWithRod(pdfOptimizedHTML)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to PDF with Rod: %w", err)
	}

	return string(pdfBytes), nil
}

// optimizeHTMLForPDF enhances the HTML content for better PDF output
func (f *PDFFormatter) optimizeHTMLForPDF(htmlContent string) string {
	// Add PDF-specific CSS optimizations
	pdfCSS := `
	<style>
		@media print {
			body { margin: 0; }
			.bg-gray-100 { background: white !important; }
			.shadow { box-shadow: none !important; border: 1px solid #e5e5e5; }
			.hover\:bg-gray-50:hover { background: transparent !important; }
		}
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
			line-height: 1.4;
			color: #333;
			padding: 20px;
		}
		.max-w-7xl { max-width: none !important; }
		table { page-break-inside: avoid; }
		.rounded-lg { border-radius: 4px; }
	</style>`

	// Insert the PDF CSS before the closing </head> tag
	if strings.Contains(htmlContent, "</head>") {
		htmlContent = strings.Replace(htmlContent, "</head>", pdfCSS+"\n</head>", 1)
	}

	return htmlContent
}

// convertHTMLToPDFWithRod converts HTML content to PDF using Rod/Chromium
func (f *PDFFormatter) convertHTMLToPDFWithRod(htmlContent string) ([]byte, error) {
	// Create a launcher with headless mode
	l := launcher.New().
		Headless(true).
		Set("disable-gpu").
		Set("no-sandbox").
		Set("disable-dev-shm-usage")

	// Launch the browser
	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	// Create a new browser instance
	browser := rod.New().ControlURL(url)
	err = browser.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}
	defer browser.MustClose()

	// Create a new page
	page := browser.MustPage()
	defer page.MustClose()

	// Set the HTML content
	err = page.SetDocumentContent(htmlContent)
	if err != nil {
		return nil, fmt.Errorf("failed to set HTML content: %w", err)
	}

	// Wait for the page to be fully loaded
	err = page.WaitStable(2 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for page to stabilize: %w", err)
	}

	// Helper function to create float64 pointer
	float64Ptr := func(v float64) *float64 {
		return &v
	}

	// Generate PDF with options
	pdfReader, err := page.PDF(&proto.PagePrintToPDF{
		DisplayHeaderFooter: false,
		PrintBackground:     true,
		Scale:               float64Ptr(1.0),
		PaperWidth:          float64Ptr(8.27),  // A4 width in inches
		PaperHeight:         float64Ptr(11.69), // A4 height in inches
		MarginTop:           float64Ptr(0.2),   // ~1cm in inches
		MarginBottom:        float64Ptr(0.2),
		MarginLeft:          float64Ptr(0.2),
		MarginRight:         float64Ptr(0.2),
		PreferCSSPageSize:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	// Read all data from the stream reader
	pdfData, err := io.ReadAll(pdfReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF data: %w", err)
	}

	return pdfData, nil
}
