package pdf

import (
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/go-pdf/fpdf"
)

type Text struct {
	Text api.Text `json:"text,omitempty"`
}

func (t Text) GetWidth() float64 {
	return GetTextMetrics(t.Text).Width
}

func (t Text) GetHeight() float64 {
	return GetTextMetrics(t.Text).Height
}

func (t Text) Draw(b *Builder, opts ...DrawOptions) error {
	// Parse options
	options := &drawOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Save current state
	savedPos := b.GetCurrentPosition()
	savedState := b.GetStyleConverter().SaveCurrentState()

	// Apply position if specified
	if options.Position != (api.Position{}) {
		b.MoveTo(options.Position)
	}

	// Apply styling and draw the text
	t.drawTextWithStyling(b, t.Text)

	// Restore state
	b.MoveTo(savedPos)
	b.GetStyleConverter().RestoreState(savedState)

	return nil
}

// drawTextWithStyling recursively draws api.Text with proper styling
func (t Text) drawTextWithStyling(b *Builder, text api.Text) {
	style := b.GetStyleConverter()
	pdf := b.GetPDF()

	// Save current state
	savedState := style.SaveCurrentState()

	// Apply styling from Class or Style string
	if text.Class != (api.Class{}) {
		style.ApplyClassToPDF(text.Class)
		
		// Apply padding if specified
		if text.Class.Padding != nil {
			leftPadding, topPadding := style.ApplyPadding(*text.Class.Padding)
			b.MoveBy(int(leftPadding), int(topPadding))
		}
	} else if text.Style != "" {
		style.ParseTailwindToPDF(text.Style)
	}

	// Position the text
	pos := b.GetCurrentPosition()
	pdf.SetXY(float64(pos.X), float64(pos.Y))

	// Handle background color if specified
	if text.Class.Background != nil {
		// Calculate text dimensions for background
		textWidth := style.GetTextWidth(text.Content)
		textHeight := style.GetTextHeight()
		
		// Draw background rectangle
		pdf.Rect(float64(pos.X), float64(pos.Y), textWidth, textHeight, "F")
	}

	// Write the main content
	if text.Content != "" {
		textHeight := style.GetTextHeight()
		
		// Handle text wrapping if content is long
		lines := t.wrapText(text.Content, 180) // Wrap at ~180mm (A4 width minus margins)
		
		for _, line := range lines {
			pdf.SetXY(float64(pos.X), float64(pos.Y))
			pdf.Cell(0, textHeight, line)
			pos.Y += int(textHeight)
			b.MoveTo(pos) // Update builder position
		}
	}

	// Draw children with proper nesting
	for _, child := range text.Children {
		t.drawTextWithStyling(b, child)
	}

	// Restore state
	style.RestoreState(savedState)
}

// wrapText wraps text to fit within the specified width
func (t Text) wrapText(text string, maxWidth float64) []string {
	// Simple word wrapping implementation
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	var lines []string
	currentLine := ""

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		// For now, use a simple character count estimate
		// In a more sophisticated implementation, you'd measure actual text width
		if len(testLine) > 80 { // Rough estimate for A4 width
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = word
			} else {
				// Single word is too long, break it anyway
				lines = append(lines, testLine)
				currentLine = ""
			}
		} else {
			currentLine = testLine
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// TextMetrics represents the dimensions of text
type TextMetrics struct {
	Width  float64
	Height float64
}

// GetTextMetrics calculates the width and height of text with default styling
func GetTextMetrics(text api.Text) TextMetrics {
	// Create a temporary PDF to measure text
	pdf := fpdf.New("P", "mm", "A4", "")
	style := NewStyleConverter(pdf)

	// Apply styling if available
	if text.Class != (api.Class{}) {
		style.ApplyClassToPDF(text.Class)
	} else if text.Style != "" {
		style.ParseTailwindToPDF(text.Style)
	} else {
		// Set default font
		pdf.SetFont("Arial", "", 12)
	}

	// Calculate dimensions
	width := style.GetTextWidth(text.Content)
	height := style.GetTextHeight()

	// Add dimensions of children
	for _, child := range text.Children {
		childMetrics := GetTextMetrics(child)
		width = max(width, childMetrics.Width) // Take maximum width
		height += childMetrics.Height          // Add heights
	}

	return TextMetrics{
		Width:  width,
		Height: height,
	}
}

// max returns the maximum of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
