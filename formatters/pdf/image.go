package pdf

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "image/jpeg" // Register JPEG format

	"github.com/flanksource/maroto/v2/pkg/components/col"
	marotoimagecomponent "github.com/flanksource/maroto/v2/pkg/components/image"
	"github.com/flanksource/maroto/v2/pkg/components/row"
	"github.com/flanksource/maroto/v2/pkg/components/text"
	"github.com/flanksource/maroto/v2/pkg/consts/align"
	"github.com/flanksource/maroto/v2/pkg/consts/border"
	"github.com/flanksource/maroto/v2/pkg/consts/extension"
	"github.com/flanksource/maroto/v2/pkg/consts/fontstyle"
	"github.com/flanksource/maroto/v2/pkg/props"
)

// Image widget for rendering images in PDF
type Image struct {
	// Local path or URL of an image
	Source  string   `json:"source,omitempty"`
	AltText string   `json:"alt_text,omitempty"`
	Width   *float64 `json:"width,omitempty"`
	Height  *float64 `json:"height,omitempty"`

	// SVG conversion options
	ConverterOptions   *ConvertOptions `json:"converter_options,omitempty"`
	PreferredConverter string          `json:"preferred_converter,omitempty"`

	// Internal field to capture conversion metadata
	lastConversionMetadata *ConversionMetadata
}

// ConversionMetadata holds information about an SVG conversion
type ConversionMetadata struct {
	ConverterUsed  string
	Duration       time.Duration
	InputSVGPath   string
	OutputPNGPath  string
	OutputFileSize int64
	DPI            int
	OutputWidth    int
	OutputHeight   int
}

// Draw implements the Widget interface
func (i *Image) Draw(b *Builder) error {
	if i.Source == "" {
		// Draw a placeholder rectangle with alt text
		return i.drawPlaceholder(b)
	}

	// Get image dimensions
	height := 50.0 // Default height in mm
	if i.Height != nil {
		height = *i.Height
	} else if i.Width != nil {
		// Assume 4:3 aspect ratio as default
		height = (*i.Width * 3.0) / 4.0
	}

	return i.drawImage(b, height)
}

// drawImage attempts to draw the actual image
func (i *Image) drawImage(b *Builder, height float64) error {
	// Handle URL vs local file
	if isURL(i.Source) {
		// Download image to bytes
		imageBytes, ext, err := i.downloadImageBytes(i.Source)
		if err != nil {
			return fmt.Errorf("failed to download image: %w", err)
		}

		// Create image component from bytes and add to row
		imageComponent := marotoimagecomponent.NewFromBytes(imageBytes, ext)
		imageCol := col.New(12).Add(imageComponent)
		b.maroto.AddRow(height, imageCol)
	} else {
		// Check if file exists
		if _, err := os.Stat(i.Source); os.IsNotExist(err) {
			return fmt.Errorf("image file not found: %s", i.Source)
		}

		// Check if it's an SVG file that needs conversion
		imagePath := i.Source
		if isSVGFile(i.Source) {
			// Convert SVG to PDF using the converter manager
			convertedPath, metadata, err := i.convertSVGWithMetadata(b, i.Source)
			if err != nil {
				return fmt.Errorf("failed to convert SVG: %w", err)
			}

			// Store metadata for potential future use (used by showcase for reporting)
			i.lastConversionMetadata = metadata

			// Since we converted to PDF, we need to embed it as a PDF file
			// PDF streams are now automatically decompressed by converters for gofpdi compatibility
			if metadata != nil && strings.HasSuffix(convertedPath, ".pdf") {
				embedWidget := NewPDFEmbedWidget(convertedPath)
				if i.Width != nil && i.Height != nil {
					embedWidget = embedWidget.WithSize(*i.Width, *i.Height)
				}
				return embedWidget.Draw(b)
			}

			imagePath = convertedPath
			// Note: We don't delete the temp file here because Maroto needs it during PDF generation
			// Temp files will be cleaned up by the OS automatically
		} else if isPDFFile(i.Source) {
			// Handle PDF files - embed directly using PDF embed widget
			embedWidget := NewPDFEmbedWidget(i.Source)
			if i.Width != nil && i.Height != nil {
				embedWidget = embedWidget.WithSize(*i.Width, *i.Height)
			}
			return embedWidget.Draw(b)
		}

		// Create image component from file and add to row
		imageComponent := marotoimagecomponent.NewFromFile(imagePath)
		imageCol := col.New(12).Add(imageComponent)
		b.maroto.AddRow(height, imageCol)
	}

	// Add alt text caption if available
	if i.AltText != "" {
		captionProps := props.Text{
			Size:  8,
			Style: fontstyle.Italic,
			Align: align.Center,
			Color: &props.Color{Red: 100, Green: 100, Blue: 100},
		}
		captionText := text.New(i.AltText, captionProps)
		captionCol := col.New(12).Add(captionText)
		b.maroto.AddRow(5, captionCol)
	}

	return nil
}

// drawPlaceholder draws a placeholder rectangle with alt text
func (i *Image) drawPlaceholder(b *Builder) error {
	// Get dimensions
	height := 50.0 // Default height in mm
	if i.Height != nil {
		height = *i.Height
	} else if i.Width != nil {
		// Assume 4:3 aspect ratio as default
		height = (*i.Width * 3.0) / 4.0
	}

	// Create placeholder box with border
	placeholderRow := row.New(height)
	placeholderCol := col.New(12)

	// Add alt text in the center if available
	if i.AltText != "" {
		textProps := props.Text{
			Size:  10,
			Style: fontstyle.Normal,
			Align: align.Center,
			Color: &props.Color{Red: 64, Green: 64, Blue: 64},
		}
		altTextComponent := text.New(i.AltText, textProps)
		placeholderCol.Add(altTextComponent)
	} else {
		// Add generic placeholder text
		textProps := props.Text{
			Size:  10,
			Style: fontstyle.Italic,
			Align: align.Center,
			Color: &props.Color{Red: 128, Green: 128, Blue: 128},
		}
		placeholderText := text.New("[Image Placeholder]", textProps)
		placeholderCol.Add(placeholderText)
	}

	placeholderRow.Add(placeholderCol)

	// Add border and background
	placeholderRow.WithStyle(&props.Cell{
		BackgroundColor: &props.Color{Red: 240, Green: 240, Blue: 240},
		BorderType:      border.Full,
		BorderColor:     &props.Color{Red: 128, Green: 128, Blue: 128},
		BorderThickness: 0.5,
	})

	b.maroto.AddRows(placeholderRow)

	return nil
}

// downloadImageBytes downloads an image from URL and returns bytes
func (i *Image) downloadImageBytes(url string) ([]byte, extension.Type, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Read image data
	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	// Determine extension
	ext := getImageExtension(url, resp.Header.Get("Content-Type"))

	return imageBytes, ext, nil
}

// isURL checks if a string looks like a URL
func isURL(str string) bool {
	return len(str) > 7 && (str[:7] == "http://" || str[:8] == "https://")
}

// getImageExtension determines the image extension
func getImageExtension(url, contentType string) extension.Type {
	// Try content type first
	switch contentType {
	case "image/jpeg", "image/jpg":
		return extension.Jpg
	case "image/png":
		return extension.Png
	case "image/gif":
		// Maroto doesn't support GIF, treat as PNG
		return extension.Png
	}

	// Try URL extension
	ext := strings.ToLower(filepath.Ext(url))
	// Remove query parameters if present
	if idx := strings.Index(ext, "?"); idx != -1 {
		ext = ext[:idx]
	}

	switch ext {
	case ".jpg", ".jpeg":
		return extension.Jpg
	case ".png":
		return extension.Png
	case ".gif":
		// Maroto doesn't support GIF, treat as PNG
		return extension.Png
	default:
		// Default to PNG
		return extension.Png
	}
}

// isSVGFile checks if a file path points to an SVG file
func isSVGFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".svg"
}

// isPDFFile checks if a file path points to a PDF file
func isPDFFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".pdf"
}

// validatePDFFile validates that a PDF file is valid and can be imported
func validatePDFFile(path string) error {
	// Check file exists
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("PDF file not found: %w", err)
	}

	// Check file is not empty
	if info.Size() == 0 {
		return fmt.Errorf("PDF file is empty")
	}

	// Check PDF header
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open PDF file: %w", err)
	}
	defer file.Close()

	header := make([]byte, 4)
	_, err = file.Read(header)
	if err != nil {
		return fmt.Errorf("cannot read PDF header: %w", err)
	}

	if string(header) != "%PDF" {
		return fmt.Errorf("invalid PDF file: missing PDF header")
	}

	// More comprehensive PDF structure validation
	file.Seek(0, 0) // Reset to beginning

	// Read entire file content for structure validation
	// Since we need to check for XRef streams which are typically at the end
	readSize := int(info.Size())
	content := make([]byte, readSize)
	n, err := file.Read(content)
	if err != nil {
		return fmt.Errorf("cannot read PDF content for validation: %w", err)
	}

	contentStr := string(content[:n])

	// Check for essential PDF structure elements
	requiredElements := []string{
		"obj",       // Object start
		"endobj",    // Object end
		"startxref", // Start of xref table (present in both traditional and compressed)
	}

	for _, element := range requiredElements {
		if !strings.Contains(contentStr, element) {
			return fmt.Errorf("invalid PDF file: missing required element '%s'", element)
		}
	}

	// Check for cross-reference structure - accept either traditional or compressed
	hasTraditionalXref := strings.Contains(contentStr, "xref") && strings.Contains(contentStr, "trailer")
	hasCompressedXref := strings.Contains(contentStr, "/Type /XRef")

	if !hasTraditionalXref && !hasCompressedXref {
		return fmt.Errorf("invalid PDF file: missing cross-reference structure (neither traditional xref nor compressed XRef stream found)")
	}

	// Additional check: ensure the file doesn't contain obvious error markers
	errorMarkers := []string{
		"ERROR",
		"Failed",
		"Unable to",
		"could not",
		"<html>", // Sometimes converters return HTML error pages
	}

	for _, marker := range errorMarkers {
		if strings.Contains(contentStr, marker) {
			return fmt.Errorf("invalid PDF file: contains error marker '%s'", marker)
		}
	}

	return nil
}

// convertSVGWithMetadata converts an SVG file to PNG using the converter manager and returns metadata
func (i *Image) convertSVGWithMetadata(b *Builder, svgPath string) (string, *ConversionMetadata, error) {
	startTime := time.Now()

	// Prepare conversion options
	options := i.ConverterOptions
	if options == nil {
		options = &ConvertOptions{
			Format: "pdf", // Use PDF for better quality
			DPI:    288,   // 3x resolution for higher quality images
		}
	}

	// Set dimensions if provided
	if i.Width != nil {
		pixelsPerMM := float64(options.DPI) / 25.4 // Convert DPI to pixels per mm
		options.Width = int(*i.Width * pixelsPerMM)
	}
	if i.Height != nil {
		pixelsPerMM := float64(options.DPI) / 25.4 // Convert DPI to pixels per mm
		options.Height = int(*i.Height * pixelsPerMM)
	}

	// Create temporary output file with appropriate extension
	var tempFile *os.File
	var outputPath string
	var err error
	if options.Format == "pdf" {
		tempFile, err = os.CreateTemp("", "converted_*.pdf")
	} else {
		tempFile, err = os.CreateTemp("", "converted_*.png")
	}
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFile.Close()
	outputPath = tempFile.Name()

	// Set preferred converter if specified
	converterUsed := "default"
	if i.PreferredConverter != "" {
		SetPreferred(i.PreferredConverter)
		converterUsed = i.PreferredConverter
	} else if available := GetAvailableConverters(); len(available) > 0 {
		converterUsed = available[0] // First available converter
	}

	// Convert with fallback
	ctx := context.Background()
	err = ConvertWithFallback(ctx, svgPath, outputPath, options)
	conversionDuration := time.Since(startTime)

	if err != nil {
		os.Remove(outputPath)
		return "", nil, fmt.Errorf("SVG conversion failed: %w", err)
	}

	// Check if output file exists and has content
	stat, err := os.Stat(outputPath)
	if err != nil {
		os.Remove(outputPath)
		return "", nil, fmt.Errorf("conversion output file missing: %w", err)
	} else if stat.Size() == 0 {
		os.Remove(outputPath)
		return "", nil, fmt.Errorf("conversion produced empty file")
	}

	// Validate the converted file
	if options.Format == "pdf" {
		// Validate the converted PDF
		if err := validatePDFFile(outputPath); err != nil {
			os.Remove(outputPath)
			return "", nil, fmt.Errorf("converted PDF validation failed: %w", err)
		}
	} else {
		// Validate that the converted PNG is actually loadable by image libraries
		if err := ValidatePNGFile(outputPath); err != nil {
			os.Remove(outputPath)
			return "", nil, fmt.Errorf("SVG conversion produced invalid PNG: %w", err)
		}
	}

	// Create metadata object
	metadata := &ConversionMetadata{
		ConverterUsed:  converterUsed,
		Duration:       conversionDuration,
		InputSVGPath:   svgPath,
		OutputPNGPath:  outputPath,
		OutputFileSize: stat.Size(),
		DPI:            options.DPI,
		OutputWidth:    options.Width,
		OutputHeight:   options.Height,
	}

	return outputPath, metadata, nil
}

// GetLastConversionMetadata returns metadata from the most recent SVG conversion
func (i *Image) GetLastConversionMetadata() *ConversionMetadata {
	return i.lastConversionMetadata
}

// ValidatePNGFile validates that a PNG file can be properly decoded
// This prevents Maroto from embedding "could not load image" error text
func ValidatePNGFile(pngPath string) error {
	file, err := os.Open(pngPath)
	if err != nil {
		return fmt.Errorf("cannot open PNG file: %w", err)
	}
	defer file.Close()

	// Try to decode the PNG to ensure it's valid
	img, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("PNG decode failed: %w", err)
	}

	// Check that the image has valid dimensions
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return fmt.Errorf("PNG has invalid dimensions: %dx%d", bounds.Dx(), bounds.Dy())
	}

	return nil
}

// validateImageFile validates that an image file can be properly decoded
func validateImageFile(imagePath string) error {
	file, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("cannot open image file: %w", err)
	}
	defer file.Close()

	// Try to decode as any supported image format
	_, _, err = image.DecodeConfig(file)
	if err != nil {
		return fmt.Errorf("image decode config failed: %w", err)
	}

	return nil
}

// getFileSize returns the size of a file, or 0 if there's an error
func getFileSize(path string) int64 {
	if stat, err := os.Stat(path); err == nil {
		return stat.Size()
	}
	return 0
}

// formatFileSize formats a file size in bytes to a human-readable string
func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d bytes", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	} else {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
}
