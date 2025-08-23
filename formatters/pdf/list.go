package pdf

import (
	"fmt"
	"strings"
	
	"github.com/flanksource/clicky/api"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// ListType represents the type of list
type ListType string

const (
	UnorderedList ListType = "unordered"
	OrderedList   ListType = "ordered"
)

// List represents a list widget
type List struct {
	Type        ListType    `json:"type,omitempty"`
	Items       []string    `json:"items,omitempty"`
	Style       api.Class   `json:"style,omitempty"`       // Tailwind classes for list
	ItemStyle   api.Class   `json:"item_style,omitempty"`  // Tailwind classes for items
	BulletStyle string      `json:"bullet_style,omitempty"` // bullet, circle, square, dash
	Indent      float64     `json:"indent,omitempty"`       // Indentation in mm
	Spacing     float64     `json:"spacing,omitempty"`      // Spacing between items
	ColumnSpan  int         `json:"column_span,omitempty"` // How many columns to span (1-12)
}

// Draw implements the Widget interface
func (l List) Draw(b *Builder) error {
	// Set defaults
	if l.Type == "" {
		l.Type = UnorderedList
	}
	if l.Indent == 0 {
		l.Indent = 5
	}
	if l.Spacing == 0 {
		l.Spacing = 4
	}
	if l.ColumnSpan == 0 || l.ColumnSpan > 12 {
		l.ColumnSpan = 12
	}
	if l.BulletStyle == "" {
		l.BulletStyle = "bullet"
	}
	
	// Apply Tailwind classes
	var textProps *props.Text
	if l.ItemStyle.Name != "" {
		resolvedClass := api.ResolveStyles(l.ItemStyle.Name)
		textProps = b.style.ConvertToTextProps(resolvedClass)
	} else {
		textProps = &props.Text{
			Size:  10,
			Align: align.Left,
		}
	}
	
	// Draw each list item
	for i, item := range l.Items {
		var prefix string
		
		// Generate prefix based on list type
		if l.Type == OrderedList {
			prefix = fmt.Sprintf("%d. ", i+1)
		} else {
			// Use bullet style
			switch l.BulletStyle {
			case "circle":
				prefix = "○ "
			case "square":
				prefix = "▪ "
			case "dash":
				prefix = "- "
			default:
				prefix = "• "
			}
		}
		
		// Create the list item text
		itemText := prefix + item
		
		// Add indentation
		textProps.Left = l.Indent
		
		// Calculate row height based on text length
		rowHeight := l.Spacing + 2 // Base height
		if len(itemText) > 80 {
			// Approximate height for multiline text
			rowHeight += float64(len(itemText)/80) * 3
		}
		
		// Add the item to the PDF
		b.maroto.AddRow(rowHeight, 
			col.New(l.ColumnSpan).Add(
				text.New(itemText, *textProps),
			),
		)
	}
	
	// Add spacing after list
	b.maroto.AddRows(row.New(2))
	
	return nil
}

// ParseMarkdownList parses a markdown list string into items
func ParseMarkdownList(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	var items []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Check for unordered list markers
		if strings.HasPrefix(line, "- ") || 
		   strings.HasPrefix(line, "* ") || 
		   strings.HasPrefix(line, "+ ") {
			items = append(items, strings.TrimSpace(line[2:]))
		}
		
		// Check for ordered list markers
		for i := 1; i <= 99; i++ {
			prefix := fmt.Sprintf("%d. ", i)
			if strings.HasPrefix(line, prefix) {
				items = append(items, strings.TrimSpace(line[len(prefix):]))
				break
			}
		}
	}
	
	return items
}