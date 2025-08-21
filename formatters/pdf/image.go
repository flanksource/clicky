package pdf

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/go-pdf/fpdf"
)

type Image struct {
	// Local path or URL of an image
	Source  string   `json:"source,omitempty"`
	AltText string   `json:"alt_text,omitempty"`
	Width   *float64 `json:"width,omitempty"`
	Height  *float64 `json:"height,omitempty"`
}

func (i Image) GetWidth() float64 {
	if i.Width != nil {
		return *i.Width
	}
	
	// Try to extract from actual image if possible
	// For now, return a default width
	return 50.0 // Default width in mm
}

func (i Image) GetHeight() float64 {
	if i.Height != nil {
		return *i.Height
	}
	
	// Try to maintain aspect ratio if width is specified
	if i.Width != nil {
		// Assume 4:3 aspect ratio as default
		return (*i.Width * 3.0) / 4.0
	}
	
	// Default height
	return 37.5 // Default height in mm (maintains 4:3 with 50mm width)
}

func (i Image) Draw(b *Builder, opts ...DrawOptions) error {
	if i.Source == "" {
		// Draw a placeholder rectangle with alt text
		return i.drawPlaceholder(b, opts...)
	}

	// Parse options
	options := &drawOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Save current state
	savedPos := b.GetCurrentPosition()

	// Apply position if specified
	if options.Position != (api.Position{}) {
		b.MoveTo(options.Position)
	}

	// Get image dimensions
	width := i.GetWidth()
	height := i.GetHeight()

	// Override with options if provided
	if options.Size != (api.Rectangle{}) {
		width = float64(options.Size.Width)
		height = float64(options.Size.Height)
	}

	pos := b.GetCurrentPosition()
	pdf := b.GetPDF()

	// Try to load and draw the image
	err := i.drawImage(pdf, float64(pos.X), float64(pos.Y), width, height)
	if err != nil {
		// Fall back to placeholder
		b.MoveTo(savedPos)
		return i.drawPlaceholder(b, opts...)
	}

	// Update builder position to below the image
	b.MoveTo(api.Position{X: pos.X, Y: pos.Y + int(height)})

	return nil
}

// drawImage attempts to draw the actual image
func (i Image) drawImage(pdf *fpdf.Fpdf, x, y, width, height float64) error {
	var imagePath string
	var isTemp bool

	// Handle URL vs local file
	if isURL(i.Source) {
		// Download image to temporary file
		tempPath, err := i.downloadImage(i.Source)
		if err != nil {
			return fmt.Errorf("failed to download image: %w", err)
		}
		imagePath = tempPath
		isTemp = true
		defer func() {
			if isTemp {
				os.Remove(imagePath)
			}
		}()
	} else {
		imagePath = i.Source
	}

	// Check if file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return fmt.Errorf("image file not found: %s", imagePath)
	}

	// Get image format from extension
	format := getImageFormat(imagePath)
	if format == "" {
		return fmt.Errorf("unsupported image format: %s", filepath.Ext(imagePath))
	}

	// Add image to PDF
	pdf.ImageOptions(imagePath, x, y, width, height, false, fpdf.ImageOptions{
		ReadDpi:   false,
		ImageType: format,
	}, 0, "")

	return nil
}

// drawPlaceholder draws a placeholder rectangle with alt text
func (i Image) drawPlaceholder(b *Builder, opts ...DrawOptions) error {
	// Parse options
	options := &drawOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Apply position if specified
	if options.Position != (api.Position{}) {
		b.MoveTo(options.Position)
	}

	pos := b.GetCurrentPosition()
	pdf := b.GetPDF()

	// Get dimensions
	width := i.GetWidth()
	height := i.GetHeight()

	// Override with options if provided
	if options.Size != (api.Rectangle{}) {
		width = float64(options.Size.Width)
		height = float64(options.Size.Height)
	}

	// Draw placeholder rectangle
	pdf.SetDrawColor(128, 128, 128) // Gray border
	pdf.SetFillColor(240, 240, 240) // Light gray fill
	pdf.Rect(float64(pos.X), float64(pos.Y), width, height, "DF")

	// Add alt text if available
	if i.AltText != "" {
		pdf.SetFont("Arial", "", 10)
		pdf.SetTextColor(64, 64, 64) // Dark gray text
		
		// Center text in placeholder
		textWidth := pdf.GetStringWidth(i.AltText)
		textHeight := 3.5 // Approximate height for 10pt font
		
		textX := float64(pos.X) + (width-textWidth)/2
		textY := float64(pos.Y) + (height+textHeight)/2
		
		pdf.SetXY(textX, textY)
		pdf.Cell(textWidth, textHeight, i.AltText)
	}

	// Update builder position
	b.MoveTo(api.Position{X: pos.X, Y: pos.Y + int(height)})

	return nil
}

// downloadImage downloads an image from URL to a temporary file
func (i Image) downloadImage(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Create temporary file
	ext := getFileExtension(url)
	tempFile, err := os.CreateTemp("", "pdf_image_*"+ext)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// Copy image data
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// isURL checks if a string looks like a URL
func isURL(str string) bool {
	return len(str) > 7 && (str[:7] == "http://" || str[:8] == "https://")
}

// getFileExtension gets file extension from path or URL
func getFileExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		// Try to extract from URL query parameters
		if idx := strings.LastIndex(path, "."); idx != -1 {
			if endIdx := strings.Index(path[idx:], "?"); endIdx != -1 {
				ext = path[idx : idx+endIdx]
			} else if endIdx := strings.Index(path[idx:], "&"); endIdx != -1 {
				ext = path[idx : idx+endIdx]
			} else {
				ext = path[idx:]
			}
		}
	}
	return ext
}

// getImageFormat returns the fpdf image format string
func getImageFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "JPG"
	case ".png":
		return "PNG"
	case ".gif":
		return "GIF"
	default:
		return ""
	}
}
