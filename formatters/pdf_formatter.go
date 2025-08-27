package formatters

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/playwright-community/playwright-go"
)

// PDFFormatter handles PDF formatting using HTML-to-PDF conversion via Playwright/Chromium
type PDFFormatter struct{}

// NewPDFFormatter creates a new PDF formatter
func NewPDFFormatter() *PDFFormatter {
	return &PDFFormatter{}
}

// Format formats PrettyData as PDF using Rod/Chromium
func (f *PDFFormatter) Format(data *api.PrettyData) (string, error) {
	// Generate HTML using the HTML formatter
	htmlFormatter := NewHTMLFormatter()
	htmlContent, err := htmlFormatter.Format(data)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML for PDF conversion: %w", err)
	}

	// Optimize HTML for PDF output
	pdfOptimizedHTML := f.optimizeHTMLForPDF(htmlContent)

	// Convert HTML to PDF using Playwright
	pdfBytes, err := f.convertHTMLToPDFWithRod(pdfOptimizedHTML)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to PDF with Playwright: %w", err)
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

// convertHTMLToPDFWithRod converts HTML content to PDF using Playwright/Chromium
func (f *PDFFormatter) convertHTMLToPDFWithRod(htmlContent string) ([]byte, error) {
	// Install playwright if needed
	err := playwright.Install()
	if err != nil {
		return nil, fmt.Errorf("failed to install playwright: %w", err)
	}

	// Launch playwright
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run playwright: %w", err)
	}
	defer pw.Stop()

	// Launch browser with headless mode
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args: []string{
			"--disable-gpu",
			"--no-sandbox",
			"--disable-dev-shm-usage",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}
	defer browser.Close()

	// Create a new page
	page, err := browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create new page: %w", err)
	}
	defer page.Close()

	// Set the HTML content
	err = page.SetContent(htmlContent)
	if err != nil {
		return nil, fmt.Errorf("failed to set HTML content: %w", err)
	}

	// Wait for the page to be fully loaded
	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait for page to load: %w", err)
	}

	// Generate PDF with options
	pdfData, err := page.PDF(playwright.PagePdfOptions{
		DisplayHeaderFooter: playwright.Bool(false),
		PrintBackground:     playwright.Bool(true),
		Scale:               playwright.Float(1.0),
		Width:               playwright.String("8.27in"),  // A4 width
		Height:              playwright.String("11.69in"), // A4 height
		Margin: &playwright.Margin{
			Top:    playwright.String("0.2in"), // ~0.5cm
			Bottom: playwright.String("0.2in"),
			Left:   playwright.String("0.2in"),
			Right:  playwright.String("0.2in"),
		},
		PreferCSSPageSize: playwright.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return pdfData, nil
}
