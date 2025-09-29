package pdf_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/clicky/api"
	. "github.com/flanksource/clicky/formatters/pdf"
	"github.com/flanksource/maroto/v2/pkg/components/col"
	marotoimagecomponent "github.com/flanksource/maroto/v2/pkg/components/image"
	"github.com/flanksource/maroto/v2/pkg/components/text"
	"github.com/flanksource/maroto/v2/pkg/consts/align"
	"github.com/flanksource/maroto/v2/pkg/consts/fontstyle"
	"github.com/flanksource/maroto/v2/pkg/core"
	"github.com/flanksource/maroto/v2/pkg/props"
)

// TestGenerateShowcasePDF generates a comprehensive PDF showcasing all widgets
func TestGenerateShowcasePDF(t *testing.T) {
	// Generate both normal and debug versions
	for _, debugMode := range []bool{false, true} {
		name := "showcase"
		if debugMode {
			name = "showcase_debug"
		}

		t.Run(name, func(t *testing.T) {
			// Create builder with debug mode
			builder := NewBuilder(WithDebug(debugMode))

			// Add header
			builder.SetHeader(api.Text{
				Content: "Clicky PDF Widget Showcase",
				Class: api.Class{
					Font: &api.Font{Bold: true, Size: 1.2},
				},
			})

			// Page 1: Text Features
			addTextFeaturesPage(builder)

			// Page 2: Table Features
			addTableFeaturesPage(builder)

			// Page 3: Layout Features
			if err := addLayoutFeaturesPage(builder); err != nil {
				t.Fatalf("Failed to add layout features page: %v", err)
			}

			// Page 4: Styling Features
			addStylingFeaturesPage(builder)

			// Page 5: Image Features
			addImageFeaturesPage(builder)

			// Page 6: SVG Features - REMOVED (consolidated into image_test.go)

			// Page 7: Label Positions Gallery
			if err := addLabelPositionsGalleryPage(builder); err != nil {
				t.Fatalf("Failed to add label positions gallery page: %v", err)
			}

			// Page 8: Two-Column Layout with Image and Table
			if err := addTwoColumnLayoutPage(builder); err != nil {
				t.Fatalf("Failed to add two-column layout page: %v", err)
			}

			// Page 9: Font Family Features
			addFontFamilyFeaturesPage(builder)

			// Page 10: Combined Examples
			addCombinedExamplesPage(builder)

			// Generate PDF
			pdfData, err := builder.Build()
			if err != nil {
				t.Fatalf("Failed to build PDF: %v", err)
			}

			// Verify no errors in the generated PDF
			assertPDFDoesNotContainErrors(t, pdfData)
			assertNoImageLoadErrors(t, pdfData)
			assertNoSVGRenderingErrors(t, pdfData)

			// Save PDF
			saveShowcasePDF(t, name, pdfData)
		})
	}
}

func addTextFeaturesPage(b *Builder) {
	// Page title
	textWidget := Text{
		Text: api.Text{
			Content: "Text Features",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	textWidget.Draw(b)

	// Section: Alignments
	sectionTitle := Text{
		Text: api.Text{
			Content: "Text Alignments",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle.Draw(b)

	// Left aligned
	leftText := Text{
		Text: api.Text{
			Content: "This text is left-aligned (default)",
			Style:   "text-left",
			Class:   api.ResolveStyles("text-left"),
		},
	}
	leftText.Draw(b)

	// Center aligned
	centerText := Text{
		Text: api.Text{
			Content: "This text is center-aligned",
			Style:   "text-center",
			Class:   api.ResolveStyles("text-center"),
		},
	}
	centerText.Draw(b)

	// Right aligned
	rightText := Text{
		Text: api.Text{
			Content: "This text is right-aligned",
			Style:   "text-right",
			Class:   api.ResolveStyles("text-right"),
		},
	}
	rightText.Draw(b)

	// Justified text
	justifyText := Text{
		Text: api.Text{
			Content: "This is justified text that will spread across the full width of the line. Lorem ipsum dolor sit amet, consectetur adipiscing elit. This demonstrates text justification in PDF generation.",
			Style:   "text-justify",
			Class:   api.ResolveStyles("text-justify"),
		},
	}
	justifyText.Draw(b)

	// Section: Font Styles
	sectionTitle2 := Text{
		Text: api.Text{
			Content: "Font Styles",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle2.Draw(b)

	// Bold text
	boldText := Text{
		Text: api.Text{
			Content: "Bold text using Tailwind font-bold",
			Class:   api.ResolveStyles("font-bold"),
		},
	}
	boldText.Draw(b)

	// Italic text
	italicText := Text{
		Text: api.Text{
			Content: "Italic text using Tailwind italic",
			Class:   api.ResolveStyles("italic"),
		},
	}
	italicText.Draw(b)

	// Different sizes
	sizes := []string{"text-xs", "text-sm", "text-base", "text-lg", "text-xl", "text-2xl"}
	for _, size := range sizes {
		sizeText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Text size: %s", size),
				Class:   api.ResolveStyles(size),
			},
		}
		sizeText.Draw(b)
	}

	// Section: Colors
	sectionTitle3 := Text{
		Text: api.Text{
			Content: "Text Colors",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle3.Draw(b)

	colors := []string{"text-red-500", "text-blue-500", "text-green-500", "text-yellow-600", "text-purple-500"}
	for _, color := range colors {
		colorText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Text with %s color", color),
				Class:   api.ResolveStyles(color),
			},
		}
		colorText.Draw(b)
	}

	// Section: Markdown Support
	sectionTitle4 := Text{
		Text: api.Text{
			Content: "Markdown Support",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle4.Draw(b)

	markdownText := Text{
		Text: api.Text{
			Content: "This text has **bold**, *italic*, and ~~strikethrough~~ markdown formatting. Also supports [links](https://example.com) and `inline code`.",
		},
		EnableMD: true,
	}
	markdownText.Draw(b)

	// Section: HTML Support
	sectionTitle5 := Text{
		Text: api.Text{
			Content: "HTML Support",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle5.Draw(b)

	htmlText := Text{
		Text: api.Text{
			Content: "This text has <b>bold</b>, <i>italic</i>, <u>underline</u>, and <s>strikethrough</s> HTML formatting.<br/>It also supports line breaks.",
		},
		EnableHTML: true,
	}
	htmlText.Draw(b)

	// Add line separator
	line := LineWidget{
		Style:     "solid",
		Color:     api.Color{Hex: "#e5e5e5"},
		Thickness: 0.5,
	}
	line.Draw(b)
}

func addTableFeaturesPage(b *Builder) {
	// Page title
	pageTitle := Text{
		Text: api.Text{
			Content: "Table Features",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	pageTitle.Draw(b)

	// Simple table with 3 columns
	sectionTitle := Text{
		Text: api.Text{
			Content: "Basic Table (3 columns)",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle.Draw(b)

	table1 := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{Label: "Name", Style: "w-[33.33%] text-left align-middle"},
				{Label: "Age", Style: "w-[33.33%] text-center align-middle"},
				{Label: "City", Style: "w-[33.33%] text-left align-middle"},
			},
			Rows: [][]any{
				{"Alice Johnson", 28, "New York"},
				{"Bob Smith", 35, "Los Angeles"},
				{"Charlie Brown", 42, "Chicago"},
			},
			HeaderStyle:       "font-bold bg-gray-100 text-center align-middle",
			RowStyle:          "text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}
	table1.Draw(b)

	// Table with custom column widths
	sectionTitle2 := Text{
		Text: api.Text{
			Content: "Table with Custom Column Widths",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle2.Draw(b)

	table2 := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{Label: "Product", Style: "w-[16.67%] text-left align-middle"},
				{Label: "Description", Style: "w-[50%] text-left align-middle"},
				{Label: "Price", Style: "w-[16.67%] text-right align-middle font-mono"},
				{Label: "Stock", Style: "w-[16.67%] text-center align-middle"},
			},
			Rows: [][]any{
				{"Laptop", "High-performance laptop with 16GB RAM", "$1,299", 15},
				{"Mouse", "Wireless ergonomic mouse", "$29.99", 150},
				{"Keyboard", "Mechanical keyboard with RGB", "$89.99", 45},
			},
			HeaderStyle:       "font-bold bg-gray-100 text-center align-middle",
			RowStyle:          "text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}
	table2.Draw(b)

	// Table with many columns
	sectionTitle3 := Text{
		Text: api.Text{
			Content: "Table with Many Columns (12 columns)",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle3.Draw(b)

	rows12 := [][]any{
		{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"},
		{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
	}

	table3 := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{Label: "1", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "2", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "3", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "4", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "5", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "6", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "7", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "8", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "9", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "10", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "11", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
				{Label: "12", Style: "w-[8.33%] text-center align-middle font-bold text-xs"},
			},
			Rows:        rows12,
			HeaderStyle: "bg-blue-100 font-bold text-xs text-center align-middle",
			RowStyle:    "text-xs text-gray-700 text-center align-middle",
			ShowBorders: true,
		},
	}
	table3.Draw(b)

	// Table with Tailwind styling
	sectionTitle4 := Text{
		Text: api.Text{
			Content: "Table with Tailwind Styling",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle4.Draw(b)

	table4 := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{Label: "Task", Style: "w-[50%] text-left align-middle"},
				{Label: "Status", Style: "w-[25%] text-center align-middle"},
				{Label: "Priority", Style: "w-[25%] text-center align-middle"},
			},
			Rows: [][]any{
				{"Complete documentation", "Done", "High"},
				{"Review pull requests", "In Progress", "Medium"},
				{"Deploy to production", "Pending", "High"},
				{"Update dependencies", "Pending", "Low"},
			},
			HeaderStyle:       "bg-gray-800 text-white font-bold text-center align-middle",
			RowStyle:          "text-sm text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}
	table4.Draw(b)
}

func addLayoutFeaturesPage(b *Builder) error {
	// Page title
	pageTitle := Text{
		Text: api.Text{
			Content: "Layout Features",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	pageTitle.Draw(b)

	// Grid demonstration
	sectionTitle := Text{
		Text: api.Text{
			Content: "12-Column Grid System",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle.Draw(b)

	// Show different column combinations
	gridExamples := []struct {
		title string
		table Table
	}{
		{
			title: "Equal columns (33.33%-33.33%-33.33%)",
			table: Table{
				BaseTable: BaseTable{
					Columns: []Column{
						{Label: "Column 1", Style: "w-[33.33%] text-center align-middle"},
						{Label: "Column 2", Style: "w-[33.33%] text-center align-middle"},
						{Label: "Column 3", Style: "w-[33.33%] text-center align-middle"},
					},
					Rows:        [][]any{{"33.33% width", "33.33% width", "33.33% width"}},
					ShowBorders: true,
					HeaderStyle: "font-bold bg-gray-100 text-center align-middle",
					RowStyle:    "text-gray-700 text-center align-middle",
				},
			},
		},
		{
			title: "Asymmetric columns (16.67%-66.67%-16.67%)",
			table: Table{
				BaseTable: BaseTable{
					Columns: []Column{
						{Label: "Side", Style: "w-[16.67%] text-center align-middle"},
						{Label: "Main Content", Style: "w-[66.67%] text-center align-middle"},
						{Label: "Side", Style: "w-[16.67%] text-center align-middle"},
					},
					Rows:        [][]any{{"16.67% width", "66.67% width (main content area)", "16.67% width"}},
					ShowBorders: true,
					HeaderStyle: "font-bold bg-gray-100 text-center align-middle",
					RowStyle:    "text-gray-700 text-center align-middle",
				},
			},
		},
		{
			title: "Progressive columns (8.33%-16.67%-25%-50%)",
			table: Table{
				BaseTable: BaseTable{
					Columns: []Column{
						{Label: "1", Style: "w-[8.33%] text-center align-middle"},
						{Label: "2", Style: "w-[16.67%] text-center align-middle"},
						{Label: "3", Style: "w-[25%] text-center align-middle"},
						{Label: "6", Style: "w-[50%] text-center align-middle"},
					},
					Rows:        [][]any{{"Tiny", "Small", "Medium", "Large content area"}},
					ShowBorders: true,
					HeaderStyle: "font-bold bg-gray-100 text-center align-middle",
					RowStyle:    "text-gray-700 text-center align-middle",
				},
			},
		},
	}

	for _, example := range gridExamples {
		exampleTitle := Text{
			Text: api.Text{
				Content: example.title,
				Class:   api.ResolveStyles("text-sm font-medium mt-2"),
			},
		}
		exampleTitle.Draw(b)
		example.table.Draw(b)
	}

	// Lists demonstration
	sectionTitle2 := Text{
		Text: api.Text{
			Content: "Lists",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle2.Draw(b)

	// Unordered list
	listTitle := Text{
		Text: api.Text{
			Content: "Unordered List:",
			Class:   api.ResolveStyles("text-sm font-medium"),
		},
	}
	listTitle.Draw(b)

	unorderedList := List{
		Type:        UnorderedList,
		Items:       []string{"First item", "Second item", "Third item with longer text", "Fourth item"},
		BulletStyle: "bullet",
		ItemStyle:   api.ResolveStyles("text-sm"),
	}
	unorderedList.Draw(b)

	// Ordered list
	listTitle2 := Text{
		Text: api.Text{
			Content: "Ordered List:",
			Class:   api.ResolveStyles("text-sm font-medium"),
		},
	}
	listTitle2.Draw(b)

	orderedList := List{
		Type:      OrderedList,
		Items:     []string{"Step one", "Step two", "Step three", "Step four"},
		ItemStyle: api.ResolveStyles("text-sm"),
	}
	orderedList.Draw(b)
	return nil
}

func addStylingFeaturesPage(b *Builder) {
	// Page title
	pageTitle := Text{
		Text: api.Text{
			Content: "Styling Features",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	pageTitle.Draw(b)

	// Lines section
	sectionTitle := Text{
		Text: api.Text{
			Content: "Line Styles",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle.Draw(b)

	// Solid line
	lineDesc := Text{
		Text: api.Text{
			Content: "Solid line:",
			Class:   api.ResolveStyles("text-sm"),
		},
	}
	lineDesc.Draw(b)

	solidLine := LineWidget{
		Style:     "solid",
		Color:     api.Color{Hex: "#000000"},
		Thickness: 1,
	}
	solidLine.Draw(b)

	// Dashed line
	lineDesc2 := Text{
		Text: api.Text{
			Content: "Dashed line:",
			Class:   api.ResolveStyles("text-sm mt-2"),
		},
	}
	lineDesc2.Draw(b)

	dashedLine := LineWidget{
		Style:     "dashed",
		Color:     api.Color{Hex: "#666666"},
		Thickness: 0.5,
	}
	dashedLine.Draw(b)

	// Dotted line
	lineDesc3 := Text{
		Text: api.Text{
			Content: "Dotted line:",
			Class:   api.ResolveStyles("text-sm mt-2"),
		},
	}
	lineDesc3.Draw(b)

	dottedLine := LineWidget{
		Style:     "dotted",
		Color:     api.Color{Hex: "#999999"},
		Thickness: 0.5,
	}
	dottedLine.Draw(b)

	// Colored lines
	sectionTitle2 := Text{
		Text: api.Text{
			Content: "Colored Lines",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle2.Draw(b)

	colors := []struct {
		name  string
		color api.Color
	}{
		{"Red", api.Color{Hex: "#ef4444"}},
		{"Blue", api.Color{Hex: "#3b82f6"}},
		{"Green", api.Color{Hex: "#10b981"}},
		{"Purple", api.Color{Hex: "#8b5cf6"}},
	}

	for _, c := range colors {
		colorLine := LineWidget{
			Style:      "solid",
			Color:      c.color,
			Thickness:  2,
			ColumnSpan: 6,
		}
		colorLine.Draw(b)
	}

	// Box demonstrations
	sectionTitle3 := Text{
		Text: api.Text{
			Content: "Box Widgets",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle3.Draw(b)

	// Create boxes with different styles
	box1 := Box{
		Rectangle: api.Rectangle{
			Width:  100,
			Height: 30,
		},
		Labels: []Label{
			{
				Text: api.Text{
					Content: "Box with border",
					Class:   api.ResolveStyles("text-center"),
				},
			},
		},
		Borders: &api.Borders{
			Top:    api.Line{Color: api.Color{Hex: "#000000"}, Width: 1},
			Bottom: api.Line{Color: api.Color{Hex: "#000000"}, Width: 1},
			Left:   api.Line{Color: api.Color{Hex: "#000000"}, Width: 1},
			Right:  api.Line{Color: api.Color{Hex: "#000000"}, Width: 1},
		},
	}
	box1.Draw(b)

	box2 := Box{
		Rectangle: api.Rectangle{
			Width:  100,
			Height: 30,
		},
		Labels: []Label{
			{
				Text: api.Text{
					Content: "Colored background",
					Class:   api.ResolveStyles("text-center bg-blue-100"),
				},
			},
		},
		Borders: &api.Borders{
			Top:    api.Line{Color: api.Color{Hex: "#0284c7"}, Width: 2},
			Bottom: api.Line{Color: api.Color{Hex: "#0284c7"}, Width: 2},
			Left:   api.Line{Color: api.Color{Hex: "#0284c7"}, Width: 2},
			Right:  api.Line{Color: api.Color{Hex: "#0284c7"}, Width: 2},
		},
	}
	box2.Draw(b)
}

func addCombinedExamplesPage(b *Builder) {
	// Page title
	pageTitle := Text{
		Text: api.Text{
			Content: "Combined Examples",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	b.AddPage()
	pageTitle.Draw(b)

	// Invoice-like example
	sectionTitle := Text{
		Text: api.Text{
			Content: "Invoice Example",
			Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
		},
	}
	sectionTitle.Draw(b)

	// Company header
	companyText := Text{
		Text: api.Text{
			Content: "ACME Corporation",
			Class:   api.ResolveStyles("text-xl font-bold"),
		},
	}
	companyText.Draw(b)

	addressText := Text{
		Text: api.Text{
			Content: "123 Business Street, Suite 100\nNew York, NY 10001\nPhone: (555) 123-4567",
			Class:   api.ResolveStyles("text-sm text-gray-600"),
		},
	}
	addressText.Draw(b)

	// Invoice details table
	invoiceTable := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{Label: "Item", Style: "w-[16.67%] text-left align-middle font-mono"},
				{Label: "Description", Style: "w-[41.67%] text-left align-middle"},
				{Label: "Qty", Style: "w-[8.33%] text-center align-middle"},
				{Label: "Price", Style: "w-[16.67%] text-right align-middle font-mono"},
				{Label: "Total", Style: "w-[16.67%] text-right align-middle font-mono font-bold"},
			},
			Rows: [][]any{
				{"PRD-001", "Professional Services", 40, "$150.00", "$6,000.00"},
				{"PRD-002", "Software License", 5, "$299.00", "$1,495.00"},
				{"PRD-003", "Support Package", 1, "$500.00", "$500.00"},
			},
			HeaderStyle:       "bg-gray-700 text-white font-bold text-center align-middle",
			RowStyle:          "text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}
	invoiceTable.Draw(b)

	// Total line
	totalLine := LineWidget{
		Style:     "solid",
		Color:     api.Color{Hex: "#000000"},
		Thickness: 1,
		Offset:    50,
		Length:    50,
	}
	totalLine.Draw(b)

	// Total amount
	totalTable := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{Label: "", Style: "w-[83.33%] text-right align-middle font-bold"},
				{Label: "", Style: "w-[16.67%] text-right align-middle font-bold font-mono"},
			},
			Rows: [][]any{
				{"Subtotal:", "$7,995.00"},
				{"Tax (8%):", "$639.60"},
				{"Total:", "$8,634.60"},
			},
			HeaderStyle: "hidden", // Hide headers for this table
			RowStyle:    "font-bold text-gray-800 align-middle",
			ShowBorders: false,
		},
	}
	totalTable.Draw(b)

	// Report example with mixed content
	sectionTitle2 := Text{
		Text: api.Text{
			Content: "Report Example with Mixed Content",
			Class:   api.ResolveStyles("text-lg font-semibold mt-6 mb-2"),
		},
	}
	sectionTitle2.Draw(b)

	// Report text with markdown
	reportText := Text{
		Text: api.Text{
			Content: "## Executive Summary\n\nThis report demonstrates the **comprehensive capabilities** of the PDF generation system. It includes:\n\n- Multiple text formatting options\n- Dynamic table generation\n- Flexible layout system\n- Rich styling features",
		},
		EnableMD: true,
	}
	reportText.Draw(b)

	// Data table
	dataTable := Table{
		BaseTable: BaseTable{
			Columns: []Column{
				{Label: "Quarter", Style: "w-[25%] text-left align-middle"},
				{Label: "Revenue", Style: "w-[25%] text-right align-middle font-mono"},
				{Label: "Growth", Style: "w-[25%] text-center align-middle"},
				{Label: "Status", Style: "w-[25%] text-center align-middle"},
			},
			Rows: [][]any{
				{"Q1 2024", "$2.5M", "+15%", "✓ Target Met"},
				{"Q2 2024", "$3.1M", "+24%", "✓ Target Exceeded"},
				{"Q3 2024", "$2.8M", "-10%", "⚠ Below Target"},
				{"Q4 2024", "$3.5M", "+25%", "✓ Target Exceeded"},
			},
			HeaderStyle:       "font-bold bg-gray-100 text-center align-middle",
			RowStyle:          "text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}
	dataTable.Draw(b)

	// Conclusion
	conclusionText := Text{
		Text: api.Text{
			Content: "This showcase demonstrates the full range of PDF generation capabilities available in the Clicky PDF formatter, including Tailwind CSS integration, markdown/HTML support, and flexible layout options.",
			Class:   api.ResolveStyles("text-sm italic text-gray-600 mt-4"),
		},
	}
	conclusionText.Draw(b)
}

func saveShowcasePDF(t *testing.T, name string, pdfData []byte) {
	// Create output directory
	outDir := "out"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Logf("Warning: Could not create output directory: %v", err)
		return
	}

	// Save PDF
	filename := fmt.Sprintf("%s.pdf", name)
	filepath := filepath.Join(outDir, filename)
	if err := os.WriteFile(filepath, pdfData, 0o644); err != nil {
		t.Logf("Warning: Could not save PDF to %s: %v", filepath, err)
	} else {
		t.Logf("PDF saved to: %s", filepath)
	}
}

// addImageFeaturesPage adds the image features page
func addImageFeaturesPage(builder *Builder) {
	// Page header
	pageHeader := Text{
		Text: api.Text{
			Content: "5. Image Features",
			Style:   "text-3xl font-bold text-blue-600",
			Class:   api.ResolveStyles("text-3xl font-bold text-blue-600"),
		},
	}
	pageHeader.Draw(builder)

	// Placeholder image section
	sectionHeader := Text{
		Text: api.Text{
			Content: "Image Widget Examples",
			Style:   "text-xl font-semibold mt-4",
			Class:   api.ResolveStyles("text-xl font-semibold"),
		},
	}
	sectionHeader.Draw(builder)

	// Simple image example (SVGBox functionality moved to image_test.go)
	// Note: SVG box functionality has been consolidated into image_test.go

	// Note: SVG Box functionality has been moved to image_test.go for consolidation

	// Multiple placeholder images with different sizes
	sectionHeader = Text{
		Text: api.Text{
			Content: "Different Image Sizes",
			Style:   "text-xl font-semibold mt-4",
			Class:   api.ResolveStyles("text-xl font-semibold"),
		},
	}
	sectionHeader.Draw(builder)

	// Note: Image size demonstrations have been moved to image_test.go for better organization
}

// SVG Functions removed and consolidated into image_test.go

// addLabelPositionsGalleryPage creates a comprehensive page showing all label position variations
func addLabelPositionsGalleryPage(builder *Builder) error {
	builder.AddPage()

	// Page title
	titleWidget := Text{
		Text: api.Text{
			Content: "Label Positions Gallery",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	titleWidget.Draw(builder)

	// Description
	descWidget := Text{
		Text: api.Text{
			Content: "Complete showcase of all available label positioning options",
			Class:   api.ResolveStyles("text-md text-center text-gray-600 mb-6"),
		},
	}
	descWidget.Draw(builder)

	// Define all label positions to showcase
	labelPositions := []struct {
		filename string
		title    string
		category string
	}{
		// Basic positions
		{"label_position_center.svg", "Center", "Basic"},
		{"label_position_top.svg", "Top", "Basic"},
		{"label_position_bottom.svg", "Bottom", "Basic"},

		// Side positions
		{"label_position_left.svg", "Left", "Basic"},
		{"label_position_right.svg", "Right", "Basic"},

		// Corner positions
		{"label_position_top-left.svg", "Top-Left", "Corner"},
		{"label_position_top-right.svg", "Top-Right", "Corner"},
		{"label_position_bottom-left.svg", "Bottom-Left", "Corner"},
		{"label_position_bottom-right.svg", "Bottom-Right", "Corner"},

		// Outside positions
		{"label_position_top-outside.svg", "Top Outside", "Outside"},
		{"label_position_bottom-outside.svg", "Bottom Outside", "Outside"},
		{"label_position_left-outside.svg", "Left Outside", "Outside"},
		{"label_position_right-outside.svg", "Right Outside", "Outside"},
	}

	// Group by category for better organization
	categories := []struct {
		name  string
		items []struct {
			filename string
			title    string
			category string
		}
	}{
		{"Basic Positions", []struct {
			filename string
			title    string
			category string
		}{}},
		{"Corner Positions", []struct {
			filename string
			title    string
			category string
		}{}},
		{"Outside Positions", []struct {
			filename string
			title    string
			category string
		}{}},
	}

	// Organize positions by category
	for _, pos := range labelPositions {
		switch pos.category {
		case "Basic":
			categories[0].items = append(categories[0].items, pos)
		case "Corner":
			categories[1].items = append(categories[1].items, pos)
		case "Outside":
			categories[2].items = append(categories[2].items, pos)
		}
	}

	// Display each category
	for _, category := range categories {
		// Category header
		categoryHeader := Text{
			Text: api.Text{
				Content: category.name,
				Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-3"),
			},
		}
		categoryHeader.Draw(builder)

		// Display items in this category (3 per row for good layout)
		for i, pos := range category.items {
			svgPath := filepath.Join("formatters", "pdf", "out", pos.filename)

			// Check if file exists
			if _, err := os.Stat(svgPath); os.IsNotExist(err) {
				// Skip missing files
				continue
			}

			// Title for this position
			titleText := Text{
				Text: api.Text{
					Content: pos.title,
					Class:   api.ResolveStyles("text-md font-medium mt-3 mb-1"),
				},
			}
			titleText.Draw(builder)

			// Convert and embed SVG
			svgImage := &Image{
				Source:  svgPath,
				AltText: fmt.Sprintf("Label position: %s", pos.title),
				Width:   floatPtr(60), // 60mm width for good visibility
				Height:  floatPtr(45), // 45mm height maintaining aspect
				ConverterOptions: &ConvertOptions{
					Format: "png",
					DPI:    288, // High resolution
				},
			}

			if err := svgImage.Draw(builder); err != nil {
				// Show error but continue
				errorText := Text{
					Text: api.Text{
						Content: fmt.Sprintf("✗ Failed to convert %s: %v", pos.title, err),
						Class:   api.ResolveStyles("text-red-600 text-xs"),
					},
				}
				errorText.Draw(builder)
				continue
			}

			// Display conversion metadata
			if metadata := svgImage.GetLastConversionMetadata(); metadata != nil {
				metadataText := Text{
					Text: api.Text{
						Content: fmt.Sprintf("✓ %s: %v (DPI: %d, %s)",
							metadata.ConverterUsed,
							metadata.Duration.Round(time.Millisecond),
							metadata.DPI,
							formatFileSize(metadata.OutputFileSize)),
						Class: api.ResolveStyles("text-green-600 text-xs"),
					},
				}
				metadataText.Draw(builder)

				// Technical details
				detailsText := Text{
					Text: api.Text{
						Content: fmt.Sprintf("Output: %dx%d pixels, High Resolution PNG",
							metadata.OutputWidth, metadata.OutputHeight),
						Class: api.ResolveStyles("text-gray-600 text-xs mb-2"),
					},
				}
				detailsText.Draw(builder)
			}

			// Add some spacing between items, page break after every 3rd item in a category
			if (i+1)%3 == 0 && i < len(category.items)-1 {
				// Add a small break between rows
				spacer := Text{
					Text: api.Text{
						Content: " ",
						Class:   api.ResolveStyles("text-xs"),
					},
				}
				spacer.Draw(builder)
			}
		}
	}

	// Summary section
	summaryHeader := Text{
		Text: api.Text{
			Content: "Summary",
			Class:   api.ResolveStyles("text-lg font-semibold mt-6 mb-2"),
		},
	}
	summaryHeader.Draw(builder)

	summaryText := Text{
		Text: api.Text{
			Content: fmt.Sprintf("This page demonstrates all %d label positioning options available. All examples are rendered at 288 DPI for high quality output and include performance metrics.", len(labelPositions)),
			Class:   api.ResolveStyles("text-sm text-gray-700 mb-4"),
		},
	}
	summaryText.Draw(builder)

	return nil
}

// createComplexSVGBoxForTesting creates a feature-rich SVG box for converter testing
func createComplexSVGBoxForTesting() SVGBox {
	return SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{Width: 400, Height: 300},
			Fill:      api.Color{Hex: "f8f8f8"},
			Border: api.Borders{
				Top:    api.Line{Width: 3, Color: api.Color{Hex: "2563eb"}},
				Right:  api.Line{Width: 3, Color: api.Color{Hex: "2563eb"}},
				Bottom: api.Line{Width: 3, Color: api.Color{Hex: "2563eb"}},
				Left:   api.Line{Width: 3, Color: api.Color{Hex: "2563eb"}},
			},
		},
		Circles: []CircleShape{
			{X: 50, Y: 50, Diameter: 30, Label: "H1", Depth: 10},
			{X: 350, Y: 50, Diameter: 25, Label: "H2", Depth: 8},
			{X: 50, Y: 250, Diameter: 35, Label: "H3", Depth: 12},
			{X: 350, Y: 250, Diameter: 28, Label: "H4", Depth: 9},
			{X: 200, Y: 150, Diameter: 40, Label: "Center", Depth: 15},
		},
		Cuts: []Cut{
			{Orientation: "horizontal", Position: 100, Width: 10, Depth: 5, Label: "Top Cut"},
			{Orientation: "horizontal", Position: 200, Width: 8, Depth: 4, Label: "Bottom Cut"},
			{Orientation: "vertical", Position: 150, Width: 12, Depth: 6, Label: "Left Cut"},
			{Orientation: "vertical", Position: 250, Width: 10, Depth: 5, Label: "Right Cut"},
		},
		EdgeCuts: []EdgeCut{
			{Edge: "top", Width: 6, Depth: 3, Label: "Top Edge"},
			{Edge: "bottom", Width: 8, Depth: 4, Label: "Bottom Edge"},
			{Edge: "left", Width: 5, Depth: 3, Label: "Left Edge"},
			{Edge: "right", Width: 7, Depth: 4, Label: "Right Edge"},
		},
		Labels: []Label{
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalCenter},
				},
				Text: api.Text{
					Content: "Complex SVG Test",
					Class: api.Class{
						Font: &api.Font{Bold: true, Size: 1.2},
					},
				},
			},
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalBottom, Horizontal: HorizontalLeft},
				},
				Text: api.Text{Content: "400mm x 300mm"},
			},
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalBottom, Horizontal: HorizontalRight},
				},
				Text: api.Text{Content: "Rev 1.0"},
			},
		},
		MeasureLines: []MeasureLine{
			{
				X1:         0,
				Y1:         -30,
				X2:         400,
				Y2:         -30,
				Label:      "400mm",
				Offset:     30,
				ShowArrows: true,
				Style:      "solid",
			},
			{
				X1:         -30,
				Y1:         0,
				X2:         -30,
				Y2:         300,
				Label:      "300mm",
				Offset:     30,
				ShowArrows: true,
				Style:      "solid",
			},
			{
				X1:         50,
				Y1:         330,
				X2:         350,
				Y2:         330,
				Label:      "300mm",
				Offset:     30,
				ShowArrows: true,
				Style:      "solid",
			},
		},
		EnableCollisionAvoidance: true,
		ShowDimensions:           true,
	}
}

// formatFileSize formats a file size in bytes to a human-readable string
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// getConverterNotes returns converter-specific notes
func getConverterNotes(converterName string) string {
	switch converterName {
	case "inkscape":
		return "Inkscape: Professional vector graphics editor with comprehensive SVG support and high-quality output."
	case "rsvg-convert":
		return "RSVG: Lightweight and fast SVG renderer from the GNOME project, excellent for server environments."
	case "playwright":
		return "Playwright: Browser-based rendering using Chromium, provides pixel-perfect web-standard SVG rendering."
	default:
		return fmt.Sprintf("%s: SVG to raster/vector converter.", converterName)
	}
}

// addTwoColumnLayoutPage creates a strict 60%/40% two-column layout with image and table
func addTwoColumnLayoutPage(builder *Builder) error {
	builder.AddPage()

	// Page title
	titleWidget := Text{
		Text: api.Text{
			Content: "Two-Column Layout: Image (60%) + Table (40%)",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	titleWidget.Draw(builder)

	// Description
	descWidget := Text{
		Text: api.Text{
			Content: "This layout demonstrates strict column proportions: 60% for image content, 40% for tabular data",
			Class:   api.ResolveStyles("text-md text-center text-gray-600 mb-6"),
		},
	}
	descWidget.Draw(builder)

	// Create a demo SVG box for the image column
	demoSVGBox := SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{Width: 300, Height: 200},
			Fill:      api.Color{Hex: "#e3f2fd"},
			Border: api.Borders{
				Top:    api.Line{Width: 3, Color: api.Color{Hex: "#1976d2"}},
				Right:  api.Line{Width: 3, Color: api.Color{Hex: "#1976d2"}},
				Bottom: api.Line{Width: 3, Color: api.Color{Hex: "#1976d2"}},
				Left:   api.Line{Width: 3, Color: api.Color{Hex: "#1976d2"}},
			},
		},
		Circles: []CircleShape{
			{X: 75, Y: 50, Diameter: 25, Label: "A"},
			{X: 225, Y: 50, Diameter: 25, Label: "B"},
			{X: 150, Y: 150, Diameter: 30, Label: "C"},
		},
		Labels: []Label{
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalCenter},
				},
				Text: api.Text{
					Content: "Sample Diagram",
					Class:   api.Class{Font: &api.Font{Bold: true, Size: 1.1}},
				},
			},
		},
		ShowDimensions: true,
		ActualWidth:    300,
		ActualHeight:   200,
		DimensionUnit:  "mm",
	}

	// Generate temporary SVG file
	svgBytes, err := demoSVGBox.GenerateSVG()
	if err != nil {
		return fmt.Errorf("failed to generate demo SVG: %w", err)
	}

	tempFile, err := os.CreateTemp("", "two_column_demo_*.svg")
	if err != nil {
		return fmt.Errorf("failed to create temp SVG file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(svgBytes); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write SVG data: %w", err)
	}
	tempFile.Close()

	// Create TRUE side-by-side layout with exact 60%/40% proportions
	// We need to implement this at the maroto level for precise control

	return addTrueTwoColumnLayout(builder, tempFile.Name())
}

// addTrueTwoColumnLayout creates a true side-by-side layout with real image and table components
func addTrueTwoColumnLayout(builder *Builder, svgPath string) error {
	// Convert SVG to PNG using the existing conversion system
	ctx := context.Background()
	// Use a unique PNG path in the out directory to avoid cleanup issues
	pngPath := "out/two_column_demo.png"

	convertOptions := &ConvertOptions{
		Format: "png",
		DPI:    288, // High resolution
	}

	// Check if SVG file exists before conversion
	if _, err := os.Stat(svgPath); err != nil {
		return fmt.Errorf("SVG file not found: %w", err)
	}

	// Try to convert SVG to PNG
	if err := ConvertWithFallback(ctx, svgPath, pngPath, convertOptions); err != nil {
		return fmt.Errorf("SVG conversion failed: %w", err)
	}

	// Verify the PNG file was created and is valid
	if err := ValidatePNGFile(pngPath); err != nil {
		return fmt.Errorf("converted PNG is invalid: %w", err)
	}

	// Create the actual side-by-side row using maroto's column system
	// 60% = 7.2 columns ≈ 7 columns (7/12 = 58.33%)
	// 40% = 4.8 columns ≈ 5 columns (5/12 = 41.67%)

	// Left column: Real image component (7 columns ≈ 58.33%) aligned to top
	imageComponent := marotoimagecomponent.NewFromFile(pngPath, props.Rect{
		Top:     0,     // Align to top of cell
		Left:    0,     // Align to left of cell
		Percent: 95,    // Use 95% of available space
		Center:  false, // Don't center, use Top/Left positioning
	})
	leftCol := col.New(7).Add(imageComponent)

	// Right column: Real table component (5 columns ≈ 41.67%)
	tableComponent := NewTableComponent(
		[]string{"Property", "Value"},
		[][]any{
			{"Width", "300mm"},
			{"Height", "200mm"},
			{"Circles", "3"},
			{"Area", "60cm²"},
			{"Border", "3px"},
			{"Ratio", "3:2"},
			{"Scale", "1:1"},
			{"Status", "Valid"},
		},
	)

	rightCol := col.New(5).Add(tableComponent)

	// Add the side-by-side row to the builder
	rowHeight := 70.0 // Height in mm
	builder.GetMaroto().AddRow(rowHeight, leftCol, rightCol)

	// Add explanation text
	explanation := Text{
		Text: api.Text{
			Content: "Above: Real image (60%) and table (40%) in side-by-side layout",
			Class:   api.ResolveStyles("text-sm text-center text-gray-600 italic mt-2"),
		},
	}
	explanation.Draw(builder)

	// PNG file will remain in out/ directory for inspection

	return nil
}

// addSVGFormatConversionGrid adds a section showing SVG conversion to all supported formats
func addSVGFormatConversionGrid(builder *Builder) error {
	// Section title
	titleText := Text{
		Text: api.Text{
			Content: "SVG to Multiple Format Conversions (No Fallback)",
			Class:   api.ResolveStyles("text-xl font-semibold mt-6 mb-2"),
		},
	}
	titleText.Draw(builder)

	// Subtitle
	subtitleText := Text{
		Text: api.Text{
			Content: "Shows actual converter availability and errors",
			Class:   api.ResolveStyles("text-sm text-gray-600 mb-4"),
		},
	}
	subtitleText.Draw(builder)

	// Create test SVG
	testSVGBox := SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{Width: 80, Height: 60},
			Fill:      api.Color{Hex: "e6f3ff"},
			Border: api.Borders{
				Top:    api.Line{Width: 1, Color: api.Color{Hex: "1e88e5"}},
				Right:  api.Line{Width: 1, Color: api.Color{Hex: "1e88e5"}},
				Bottom: api.Line{Width: 1, Color: api.Color{Hex: "1e88e5"}},
				Left:   api.Line{Width: 1, Color: api.Color{Hex: "1e88e5"}},
			},
		},
		Circles: []CircleShape{
			{X: 25, Y: 20, Diameter: 15, Label: "A"},
			{X: 55, Y: 20, Diameter: 12, Label: "B"},
		},
		Cuts: []Cut{
			{Orientation: "horizontal", Position: 40, Width: 4, Label: "H1"},
		},
		Labels: []Label{
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalBottom, Horizontal: HorizontalCenter},
				},
				Text: api.Text{Content: "Grid Test"},
			},
		},
	}

	// Generate SVG content
	svgBytes, err := testSVGBox.GenerateSVG()
	if err != nil {
		return fmt.Errorf("failed to generate test SVG: %w", err)
	}

	// Create temporary SVG file
	tempSVG, err := os.CreateTemp("", "grid_test_*.svg")
	if err != nil {
		return fmt.Errorf("failed to create temp SVG file: %w", err)
	}
	defer os.Remove(tempSVG.Name())

	if _, err := tempSVG.Write(svgBytes); err != nil {
		tempSVG.Close()
		return fmt.Errorf("failed to write SVG data: %w", err)
	}
	tempSVG.Close()

	// Get all supported formats
	supportedFormats := GetSupportedFormats()

	// Process formats in groups of 3 for 3-column layout
	for i := 0; i < len(supportedFormats); i += 3 {
		// Get up to 3 formats for this row
		rowFormats := supportedFormats[i:]
		if len(rowFormats) > 3 {
			rowFormats = rowFormats[:3]
		}

		// Create columns for this row
		var columns []core.Col

		for j, format := range rowFormats {
			column := createFormatGridCell(tempSVG.Name(), format, j)
			columns = append(columns, column)
		}

		// Fill remaining columns if needed (for last row)
		for len(columns) < 3 {
			emptyCol := col.New(4)
			columns = append(columns, emptyCol)
		}

		// Add row with fixed height
		rowHeight := 50.0 // mm
		builder.GetMaroto().AddRow(rowHeight, columns[0], columns[1], columns[2])
	}

	return nil
}

// createFormatGridCell creates a single cell in the format grid
func createFormatGridCell(svgPath, format string, columnIndex int) core.Col {
	// Create temporary output file
	outputPath := fmt.Sprintf("out/grid_%s.%s", format, format)

	// Try conversion without fallback
	ctx := context.Background()
	options := &ConvertOptions{
		Format: format,
		DPI:    288,
		Width:  200, // Small size for grid
		Height: 150,
	}

	// Perform conversion
	convertErr := Convert(ctx, svgPath, outputPath, options)

	// Create column content based on conversion result
	gridCell := col.New(4)

	if convertErr != nil {
		// Show error in red box
		errorWidget := createErrorWidget(format, convertErr)
		gridCell.Add(errorWidget)
	} else {
		// For PDF format, we won't try to embed here since it requires Draw() method
		// Instead, we'll show a simple success message and let PDF embedding fail elsewhere if needed
		successWidget := createSuccessWidget(format, outputPath)
		gridCell.Add(successWidget)
	}

	return gridCell
}

// createErrorWidget creates a widget showing conversion error
func createErrorWidget(format string, err error) core.Component {
	errorText := fmt.Sprintf("❌ %s\nConversion Failed\n%v",
		strings.ToUpper(format),
		err.Error())

	// Truncate long error messages
	if len(errorText) > 100 {
		errorText = errorText[:97] + "..."
	}

	return text.New(errorText, props.Text{
		Size:  8,
		Style: fontstyle.Normal,
		Align: align.Center,
		Color: &props.Color{Red: 200, Green: 0, Blue: 0}, // Red text
	})
}

// createSuccessWidget creates a widget showing successful conversion
func createSuccessWidget(format, outputPath string) core.Component {
	// For PDF format, show success but note that embedding will fail elsewhere
	if format == "pdf" {
		var fileSize string
		if stat, err := os.Stat(outputPath); err == nil {
			fileSize = formatFileSize(stat.Size())
		} else {
			fileSize = "Unknown size"
		}

		successText := fmt.Sprintf("✓ %s\nPDF Created\n%s",
			strings.ToUpper(format),
			fileSize)

		return text.New(successText, props.Text{
			Size:  8,
			Style: fontstyle.Normal,
			Align: align.Center,
			Color: &props.Color{Red: 0, Green: 150, Blue: 0}, // Green text
		})
	}

	// For raster formats, embed the actual converted image
	if _, err := os.Stat(outputPath); err == nil {
		// Try to embed the actual image
		imageComponent := marotoimagecomponent.NewFromFile(outputPath, props.Rect{
			Center:  true,
			Percent: 80, // Use 80% of available space
		})
		return imageComponent
	}

	// Fallback to text if image file doesn't exist
	var fileSize string
	if stat, err := os.Stat(outputPath); err == nil {
		fileSize = formatFileSize(stat.Size())
	} else {
		fileSize = "File not found"
	}

	successText := fmt.Sprintf("✓ %s\nRaster Format\n%s",
		strings.ToUpper(format),
		fileSize)

	return text.New(successText, props.Text{
		Size:  8,
		Style: fontstyle.Normal,
		Align: align.Center,
		Color: &props.Color{Red: 0, Green: 150, Blue: 0}, // Green text
	})
}

// addFontFamilyFeaturesPage demonstrates font-family-{Name} Tailwind classes
func addFontFamilyFeaturesPage(builder *Builder) {
	builder.AddPage()

	// Page title
	titleWidget := Text{
		Text: api.Text{
			Content: "Font Family Features",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	titleWidget.Draw(builder)

	// Description
	descWidget := Text{
		Text: api.Text{
			Content: "Demonstration of font-family-{Name} Tailwind classes with Unicode compatibility testing",
			Class:   api.ResolveStyles("text-md text-center text-gray-600 mb-6"),
		},
	}
	descWidget.Draw(builder)

	// Section 1: Font Family Comparison
	sectionHeader := Text{
		Text: api.Text{
			Content: "Font Family Comparison",
			Class:   api.ResolveStyles("text-xl font-semibold mt-4 mb-3"),
		},
	}
	sectionHeader.Draw(builder)

	// Font families to demonstrate
	fontFamilies := []struct {
		name        string
		className   string
		description string
		sample      string
	}{
		{"Arial", "font-family-Arial", "Sans-serif, excellent Unicode support", "The quick brown fox jumps over lazy dog π²³°±µΩ"},
		{"Times", "font-family-Times", "Serif, traditional readability", "The quick brown fox jumps over lazy dog π²³°±µΩ"},
		{"Courier", "font-family-Courier", "Monospace, ideal for code and data", "The quick brown fox jumps π²³°±µΩ"},
		{"Helvetica", "font-family-Helvetica", "Sans-serif, clean modern appearance", "The quick brown fox jumps π²³°±µΩ"},
	}

	for _, font := range fontFamilies {
		// Font name and description
		nameText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s Font Family", font.name),
				Class:   api.ResolveStyles("text-lg font-semibold mt-3 mb-1"),
			},
		}
		nameText.Draw(builder)

		descText := Text{
			Text: api.Text{
				Content: font.description,
				Class:   api.ResolveStyles("text-sm text-gray-600 mb-2"),
			},
		}
		descText.Draw(builder)

		// Sample text with font applied
		sampleText := Text{
			Text: api.Text{
				Content: font.sample,
				Class:   api.ResolveStyles(fmt.Sprintf("%s text-base", font.className)),
			},
		}
		sampleText.Draw(builder)

		// Class name reference
		classText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Usage: %s", font.className),
				Class:   api.ResolveStyles("text-xs text-gray-500 mb-3 font-mono"),
			},
		}
		classText.Draw(builder)
	}

	// Section 2: Font Weight Combinations
	sectionHeader2 := Text{
		Text: api.Text{
			Content: "Font Family + Weight Combinations",
			Class:   api.ResolveStyles("text-xl font-semibold mt-6 mb-3"),
		},
	}
	sectionHeader2.Draw(builder)

	// Weight combinations
	weightCombos := []struct {
		family string
		weight string
		class  string
	}{
		{"Arial", "Normal", "font-family-Arial font-normal"},
		{"Arial", "Bold", "font-family-Arial font-bold"},
		{"Times", "Normal", "font-family-Times font-normal"},
		{"Times", "Bold", "font-family-Times font-bold"},
		{"Courier", "Normal", "font-family-Courier font-normal"},
		{"Courier", "Bold", "font-family-Courier font-bold"},
	}

	for _, combo := range weightCombos {
		comboText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s %s: Sample text with Unicode π²³°±µΩπ√∞≤", combo.family, combo.weight),
				Class:   api.ResolveStyles(fmt.Sprintf("%s text-base", combo.class)),
			},
		}
		comboText.Draw(builder)

		classRefText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Class: %s", combo.class),
				Class:   api.ResolveStyles("text-xs text-gray-500 mb-2 font-mono"),
			},
		}
		classRefText.Draw(builder)
	}

	// Section 3: Unicode Compatibility Table
	sectionHeader3 := Text{
		Text: api.Text{
			Content: "Unicode Compatibility Matrix",
			Class:   api.ResolveStyles("text-xl font-semibold mt-6 mb-3"),
		},
	}
	sectionHeader3.Draw(builder)

	// Create Unicode test table
	unicodeChars := []string{"²", "³", "°", "±", "µ", "Ω", "π", "√", "∞", "≤"}
	testFonts := []string{"Arial", "Times", "Courier"}

	// Table headers
	headers := []string{"Font Family"}
	for _, char := range unicodeChars {
		headers = append(headers, char)
	}

	// Table rows
	var rows [][]any
	for _, font := range testFonts {
		row := []any{font}
		for _, char := range unicodeChars {
			row = append(row, char)
		}
		rows = append(rows, row)
	}

	// Create columns
	columns := []Column{
		{Label: "Font Family", Style: "w-[20%] text-left align-middle font-medium"},
	}

	charWidth := fmt.Sprintf("w-[%d%%]", 80/len(unicodeChars))
	for _, char := range unicodeChars {
		columns = append(columns, Column{
			Label: char,
			Style: fmt.Sprintf("%s text-center align-middle", charWidth),
		})
	}

	unicodeTable := Table{
		BaseTable: BaseTable{
			Columns:           columns,
			Rows:              rows,
			HeaderStyle:       "bg-gray-800 text-white font-bold text-sm text-center align-middle",
			RowStyle:          "text-lg text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}
	unicodeTable.Draw(builder)

	// Section 4: Practical Usage Examples
	sectionHeader4 := Text{
		Text: api.Text{
			Content: "Practical Usage Examples",
			Class:   api.ResolveStyles("text-xl font-semibold mt-6 mb-3"),
		},
	}
	sectionHeader4.Draw(builder)

	usageExamples := []struct {
		purpose string
		class   string
		example string
	}{
		{"Headers", "font-family-Arial font-bold text-xl", "Document Title in Arial Bold"},
		{"Body Text", "font-family-Times text-base", "Main content in readable Times font"},
		{"Code/Data", "font-family-Courier text-sm", "monospace_code_example = true"},
		{"Captions", "font-family-Helvetica text-xs", "Small caption text in Helvetica"},
		{"Technical", "font-family-Arial text-sm", "Formula: E = mc² (±0.1%)"},
		{"Math", "font-family-Times text-base", "Mathematical expression: π ≈ 3.14159, √2 ≈ 1.414"},
	}

	for _, example := range usageExamples {
		purposeText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s:", example.purpose),
				Class:   api.ResolveStyles("text-sm font-medium text-gray-700 mt-2"),
			},
		}
		purposeText.Draw(builder)

		exampleText := Text{
			Text: api.Text{
				Content: example.example,
				Class:   api.ResolveStyles(example.class),
			},
		}
		exampleText.Draw(builder)

		classText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Class: %s", example.class),
				Class:   api.ResolveStyles("text-xs text-gray-500 mb-3 font-mono"),
			},
		}
		classText.Draw(builder)
	}

	// Summary
	summaryText := Text{
		Text: api.Text{
			Content: "Font families are specified using font-family-{Name} classes where {Name} can be Arial, Times, Courier, Helvetica, Georgia, or Verdana. All fonts support Unicode characters for international and mathematical content.",
			Class:   api.ResolveStyles("text-sm text-gray-600 italic mt-4 p-4 bg-gray-100"),
		},
	}
	summaryText.Draw(builder)
}

// floatPtr is a helper to create a pointer to a float64
func floatPtr(f float64) *float64 {
	return &f
}
