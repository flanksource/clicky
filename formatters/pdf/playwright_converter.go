package pdf

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/playwright-community/playwright-go"
)

// PlaywrightConverter implements SVGConverter using Playwright
type PlaywrightConverter struct {
	pw      *playwright.Playwright
	browser playwright.Browser
}

// NewPlaywrightConverter creates a new Playwright converter
func NewPlaywrightConverter() *PlaywrightConverter {
	return &PlaywrightConverter{}
}

// Name returns the name of this converter
func (c *PlaywrightConverter) Name() string {
	return "playwright"
}

// IsAvailable checks if Playwright is available (lazy initialization)
func (c *PlaywrightConverter) IsAvailable() bool {
	return true
}

// SupportedFormats returns formats supported by Playwright
func (c *PlaywrightConverter) SupportedFormats() []string {
	return []string{"png", "jpg", "jpeg", "pdf"}
}

// Convert converts an SVG file using Playwright
func (c *PlaywrightConverter) Convert(ctx context.Context, svgPath, outputPath string, options *ConvertOptions) error {
	if !c.IsAvailable() {
		return NewConverterError(c.Name(), "convert", fmt.Errorf("playwright not available"))
	}

	if options == nil {
		options = DefaultConvertOptions()
	}

	if err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
	}); err != nil {
		return NewConverterError(c.Name(), "install browsers", err)
	}

	// Initialize browser if not already done
	if c.browser == nil {
		if pw, err := playwright.Run(); err != nil {
			return NewConverterError(c.Name(), "start playwright", err)
		} else {
			c.pw = pw
		}

		browser, err := c.pw.Chromium.Launch()
		if err != nil {
			return NewConverterError(c.Name(), "launch browser", err)
		}
		c.browser = browser
	}

	// Read SVG content
	svgContent, err := ioutil.ReadFile(svgPath)
	if err != nil {
		return NewConverterError(c.Name(), "read SVG", err)
	}

	// Create a new page
	page, err := c.browser.NewPage()
	if err != nil {
		return NewConverterError(c.Name(), "create page", err)
	}
	defer page.Close()

	// Create HTML wrapper for SVG
	htmlContent := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <style>
        body { margin: 0; padding: 0; }
        svg { display: block; }
    </style>
</head>
<body>
    %s
</body>
</html>`, string(svgContent))

	// Set content
	if err := page.SetContent(htmlContent); err != nil {
		return NewConverterError(c.Name(), "set content", err)
	}

	format := strings.ToLower(options.Format)
	switch format {
	case "png":
		return c.convertToPNG(page, outputPath, options)
	case "jpg", "jpeg":
		return c.convertToJPEG(page, outputPath, options)
	case "pdf":
		return c.convertToPDF(page, outputPath, options)
	default:
		return NewConverterError(c.Name(), "convert", fmt.Errorf("unsupported format: %s", format))
	}
}

func (c *PlaywrightConverter) convertToPNG(page playwright.Page, outputPath string, options *ConvertOptions) error {
	screenshotOptions := playwright.PageScreenshotOptions{
		Path: &outputPath,
		Type: playwright.ScreenshotTypePng,
	}

	if options.Width > 0 && options.Height > 0 {
		if err := page.SetViewportSize(options.Width, options.Height); err != nil {
			return NewConverterError(c.Name(), "set viewport", err)
		}
	}

	if _, err := page.Screenshot(screenshotOptions); err != nil {
		return NewConverterError(c.Name(), "screenshot PNG", err)
	}

	return nil
}

func (c *PlaywrightConverter) convertToJPEG(page playwright.Page, outputPath string, options *ConvertOptions) error {
	screenshotOptions := playwright.PageScreenshotOptions{
		Path: &outputPath,
		Type: playwright.ScreenshotTypeJpeg,
	}

	if options.Quality > 0 {
		quality := options.Quality
		screenshotOptions.Quality = &quality
	}

	if options.Width > 0 && options.Height > 0 {
		if err := page.SetViewportSize(options.Width, options.Height); err != nil {
			return NewConverterError(c.Name(), "set viewport", err)
		}
	}

	if _, err := page.Screenshot(screenshotOptions); err != nil {
		return NewConverterError(c.Name(), "screenshot JPEG", err)
	}

	return nil
}

func (c *PlaywrightConverter) convertToPDF(page playwright.Page, outputPath string, options *ConvertOptions) error {
	pdfOptions := playwright.PagePdfOptions{
		Path: &outputPath,
	}

	if options.Width > 0 && options.Height > 0 {
		width := fmt.Sprintf("%dpx", options.Width)
		height := fmt.Sprintf("%dpx", options.Height)
		pdfOptions.Width = &width
		pdfOptions.Height = &height
	}

	if _, err := page.PDF(pdfOptions); err != nil {
		return NewConverterError(c.Name(), "generate PDF", err)
	}

	return nil
}

// ConvertToFormat is a convenience method that determines output path based on format
func (c *PlaywrightConverter) ConvertToFormat(ctx context.Context, svgPath, format string, options *ConvertOptions) (string, error) {
	if options == nil {
		options = DefaultConvertOptions()
	}
	options.Format = format

	ext := "." + strings.ToLower(format)
	if format == "jpeg" {
		ext = ".jpg"
	}
	outputPath := strings.TrimSuffix(svgPath, filepath.Ext(svgPath)) + ext

	err := c.Convert(ctx, svgPath, outputPath, options)
	if err != nil {
		return "", err
	}

	return outputPath, nil
}

// Close closes the browser and Playwright instance
func (c *PlaywrightConverter) Close() error {
	if c.browser != nil {
		if err := c.browser.Close(); err != nil {
			return err
		}
		c.browser = nil
	}

	if c.pw != nil {
		if err := c.pw.Stop(); err != nil {
			return err
		}
		c.pw = nil
	}

	return nil
}
