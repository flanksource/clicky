package pdf

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// Image widget for rendering images in PDF
type Image struct {
	// Local path or URL of an image
	Source  string   `json:"source,omitempty"`
	AltText string   `json:"alt_text,omitempty"`
	Width   *float64 `json:"width,omitempty"`
	Height  *float64 `json:"height,omitempty"`
}

// Draw implements the Widget interface
func (i Image) Draw(b *Builder) error {
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

	// Try to load and draw the image
	err := i.drawImage(b, height)
	if err != nil {
		// Fall back to placeholder
		return i.drawPlaceholder(b)
	}

	return nil
}

// drawImage attempts to draw the actual image
func (i Image) drawImage(b *Builder, height float64) error {
	// Handle URL vs local file
	if isURL(i.Source) {
		// Download image to bytes
		imageBytes, ext, err := i.downloadImageBytes(i.Source)
		if err != nil {
			return fmt.Errorf("failed to download image: %w", err)
		}
		
		// Create image component from bytes and add to row
		imageComponent := image.NewFromBytes(imageBytes, ext)
		imageCol := col.New(12).Add(imageComponent)
		b.maroto.AddRow(height, imageCol)
	} else {
		// Check if file exists
		if _, err := os.Stat(i.Source); os.IsNotExist(err) {
			return fmt.Errorf("image file not found: %s", i.Source)
		}
		
		// Create image component from file and add to row
		imageComponent := image.NewFromFile(i.Source)
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
func (i Image) drawPlaceholder(b *Builder) error {
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
func (i Image) downloadImageBytes(url string) ([]byte, extension.Type, error) {
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
func getImageExtension(url string, contentType string) extension.Type {
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