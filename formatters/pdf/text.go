package pdf

import (
	"github.com/flanksource/clicky/api"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
)

// Text widget for rendering text in PDF
type Text struct {
	Text api.Text `json:"text,omitempty"`
}

// Draw implements the Widget interface
func (t Text) Draw(b *Builder) error {
	// Draw the text and all its children
	t.drawTextWithChildren(b, t.Text)
	return nil
}

// drawTextWithChildren recursively draws text and its children
func (t Text) drawTextWithChildren(b *Builder, apiText api.Text) {
	// Calculate height for this text
	height := b.style.CalculateTextHeight(apiText.Class)
	
	// Apply padding if specified
	var topPadding, bottomPadding float64
	if apiText.Class.Padding != nil {
		_, topPadding, _, bottomPadding = b.style.CalculatePadding(apiText.Class.Padding)
		
		// Add top padding as empty row
		if topPadding > 0 {
			b.maroto.AddRows(row.New(topPadding))
		}
	}
	
	// Draw main content if present
	if apiText.Content != "" {
		// Convert style to text properties
		textProps := b.style.ConvertToTextProps(apiText.Class)
		
		// Create text component
		textComponent := text.New(apiText.Content, *textProps)
		
		// Create column with text
		textCol := col.New(12).Add(textComponent)
		
		// Add row with text
		b.maroto.AddRow(height, textCol)
	}
	
	// Add bottom padding if specified
	if bottomPadding > 0 {
		b.maroto.AddRows(row.New(bottomPadding))
	}
	
	// Draw children recursively
	for _, child := range apiText.Children {
		t.drawTextWithChildren(b, child)
	}
}