package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/clicky/api"
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
			addLayoutFeaturesPage(builder)
			
			// Page 4: Styling Features
			addStylingFeaturesPage(builder)
			
			// Page 5: Image Features
			addImageFeaturesPage(builder)
			
			// Page 6: SVG Features
			addSVGFeaturesPage(builder)
			
			// Page 7: Combined Examples
			addCombinedExamplesPage(builder)
			
			// Generate PDF
			pdfData, err := builder.Build()
			if err != nil {
				t.Fatalf("Failed to build PDF: %v", err)
			}
			
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
	
	table1 := TableImproved{
		Headers: []string{"Name", "Age", "City"},
		Rows: [][]any{
			{"Alice Johnson", 28, "New York"},
			{"Bob Smith", 35, "Los Angeles"},
			{"Charlie Brown", 42, "Chicago"},
		},
		ShowBorders:       true,
		AlternateRowColor: true,
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
	
	table2 := TableImproved{
		Headers: []string{"Product", "Description", "Price", "Stock"},
		Rows: [][]any{
			{"Laptop", "High-performance laptop with 16GB RAM", "$1,299", 15},
			{"Mouse", "Wireless ergonomic mouse", "$29.99", 150},
			{"Keyboard", "Mechanical keyboard with RGB", "$89.99", 45},
		},
		ColumnWidths:      []int{2, 6, 2, 2}, // Custom widths totaling 12
		ColumnAlignments:  []string{"left", "left", "right", "center"},
		ShowBorders:       true,
		AlternateRowColor: true,
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
	
	headers12 := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"}
	rows12 := [][]any{
		{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"},
		{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"},
	}
	
	table3 := TableImproved{
		Headers:           headers12,
		Rows:              rows12,
		ShowBorders:       true,
		AlternateRowColor: false,
		HeaderStyle:       api.ResolveStyles("bg-blue-100 font-bold text-xs"),
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
	
	table4 := TableImproved{
		Headers: []string{"Task", "Status", "Priority"},
		Rows: [][]any{
			{"Complete documentation", "Done", "High"},
			{"Review pull requests", "In Progress", "Medium"},
			{"Deploy to production", "Pending", "High"},
			{"Update dependencies", "Pending", "Low"},
		},
		HeaderStyle:       api.ResolveStyles("bg-gray-800 text-white font-bold"),
		RowStyle:         api.ResolveStyles("text-sm"),
		ShowBorders:      true,
		AlternateRowColor: true,
		ColumnAlignments: []string{"left", "center", "center"},
	}
	table4.Draw(b)
}

func addLayoutFeaturesPage(b *Builder) {
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
		table TableImproved
	}{
		{
			title: "Equal columns (4-4-4)",
			table: TableImproved{
				Headers:      []string{"Column 1", "Column 2", "Column 3"},
				Rows:         [][]any{{"4 units", "4 units", "4 units"}},
				ColumnWidths: []int{4, 4, 4},
				ShowBorders:  true,
			},
		},
		{
			title: "Asymmetric columns (2-8-2)",
			table: TableImproved{
				Headers:      []string{"Side", "Main Content", "Side"},
				Rows:         [][]any{{"2 units", "8 units (main content area)", "2 units"}},
				ColumnWidths: []int{2, 8, 2},
				ShowBorders:  true,
			},
		},
		{
			title: "Progressive columns (1-2-3-6)",
			table: TableImproved{
				Headers:      []string{"1", "2", "3", "6"},
				Rows:         [][]any{{"Tiny", "Small", "Medium", "Large content area"}},
				ColumnWidths: []int{1, 2, 3, 6},
				ShowBorders:  true,
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
					Class: api.ResolveStyles("text-center"),
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
					Class: api.ResolveStyles("text-center bg-blue-100"),
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
	invoiceTable := TableImproved{
		Headers: []string{"Item", "Description", "Qty", "Price", "Total"},
		Rows: [][]any{
			{"PRD-001", "Professional Services", 40, "$150.00", "$6,000.00"},
			{"PRD-002", "Software License", 5, "$299.00", "$1,495.00"},
			{"PRD-003", "Support Package", 1, "$500.00", "$500.00"},
		},
		ColumnWidths:      []int{2, 5, 1, 2, 2},
		ColumnAlignments:  []string{"left", "left", "center", "right", "right"},
		ShowBorders:       true,
		AlternateRowColor: true,
		HeaderStyle:       api.ResolveStyles("bg-gray-700 text-white font-bold"),
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
	totalTable := TableImproved{
		Headers: []string{"", ""},
		Rows: [][]any{
			{"Subtotal:", "$7,995.00"},
			{"Tax (8%):", "$639.60"},
			{"Total:", "$8,634.60"},
		},
		ColumnWidths:     []int{10, 2},
		ColumnAlignments: []string{"right", "right"},
		ShowBorders:      false,
		RowStyle:        api.ResolveStyles("font-bold"),
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
	dataTable := TableImproved{
		Headers: []string{"Quarter", "Revenue", "Growth", "Status"},
		Rows: [][]any{
			{"Q1 2024", "$2.5M", "+15%", "✓ Target Met"},
			{"Q2 2024", "$3.1M", "+24%", "✓ Target Exceeded"},
			{"Q3 2024", "$2.8M", "-10%", "⚠ Below Target"},
			{"Q4 2024", "$3.5M", "+25%", "✓ Target Exceeded"},
		},
		ShowBorders:       true,
		AlternateRowColor: true,
		ColumnAlignments:  []string{"left", "right", "center", "center"},
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
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Logf("Warning: Could not create output directory: %v", err)
		return
	}
	
	// Save PDF
	filename := fmt.Sprintf("%s.pdf", name)
	filepath := filepath.Join(outDir, filename)
	if err := os.WriteFile(filepath, pdfData, 0644); err != nil {
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
	
	// Placeholder image
	sectionHeader := Text{
		Text: api.Text{
			Content: "Image Placeholder",
			Style:   "text-xl font-semibold mt-4",
			Class:   api.ResolveStyles("text-xl font-semibold"),
		},
	}
	sectionHeader.Draw(builder)
	
	placeholderImage := Image{
		AltText: "This is an image placeholder with alt text",
		Height:  &[]float64{40}[0],
	}
	placeholderImage.Draw(builder)
	
	// SVG Box as image example
	sectionHeader = Text{
		Text: api.Text{
			Content: "SVG Box Example",
			Style:   "text-xl font-semibold mt-4",
			Class:   api.ResolveStyles("text-xl font-semibold"),
		},
	}
	sectionHeader.Draw(builder)
	
	// Create an SVG box
	svgBox := SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{
				Width:  150,
				Height: 100,
			},
			Fill: api.Color{Hex: "#e3f2fd"},
			Border: api.Borders{
				Top:    api.Line{Color: api.Color{Hex: "#2196f3"}, Width: 2},
				Bottom: api.Line{Color: api.Color{Hex: "#2196f3"}, Width: 2},
				Left:   api.Line{Color: api.Color{Hex: "#2196f3"}, Width: 2},
				Right:  api.Line{Color: api.Color{Hex: "#2196f3"}, Width: 2},
			},
		},
		Labels: []Label{
			{
				Text: api.Text{Content: "SVG Box"},
				Positionable: Positionable{
					Position: &LabelPosition{
						Vertical:   VerticalCenter,
						Horizontal: HorizontalCenter,
					},
				},
			},
		},
		Circles: []CircleShape{
			{X: 30, Y: 30, Diameter: 15, Label: "A"},
			{X: 120, Y: 30, Diameter: 15, Label: "B"},
		},
		ShowDimensions: true,
		ActualWidth:    150,
		ActualHeight:   100,
		DimensionUnit:  "mm",
	}
	
	// Generate SVG and save for reference
	svgData, err := svgBox.GenerateSVG()
	if err == nil {
		os.WriteFile("out/showcase_svgbox.svg", svgData, 0644)
		
		// Note about SVG
		note := Text{
			Text: api.Text{
				Content: "Note: SVG box saved to out/showcase_svgbox.svg",
				Style:   "text-sm text-gray-600 italic",
				Class:   api.ResolveStyles("text-sm text-gray-600 italic"),
			},
		}
		note.Draw(builder)
	}
	
	// Multiple placeholder images with different sizes
	sectionHeader = Text{
		Text: api.Text{
			Content: "Different Image Sizes",
			Style:   "text-xl font-semibold mt-4",
			Class:   api.ResolveStyles("text-xl font-semibold"),
		},
	}
	sectionHeader.Draw(builder)
	
	smallImage := Image{
		AltText: "Small Image (30mm height)",
		Height:  &[]float64{30}[0],
	}
	smallImage.Draw(builder)
	
	mediumImage := Image{
		AltText: "Medium Image (50mm height)",
		Height:  &[]float64{50}[0],
	}
	mediumImage.Draw(builder)
	
	largeImage := Image{
		AltText: "Large Image (70mm height)",
		Height:  &[]float64{70}[0],
	}
	largeImage.Draw(builder)
}

func addSVGFeaturesPage(builder *Builder) {
	// Page title
	titleWidget := Text{
		Text: api.Text{
			Content: "SVG Features",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	titleWidget.Draw(builder)
	
	// Section 1: Basic SVG Box
	sectionHeader := Text{
		Text: api.Text{
			Content: "Basic SVG Box with Circles and Cuts",
			Class:   api.ResolveStyles("text-xl font-semibold mt-4 mb-2"),
		},
	}
	sectionHeader.Draw(builder)
	
	// Create SVG box with basic elements
	basicSVGBox := SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{Width: 200, Height: 150},
			Fill:      api.Color{Hex: "f0f0f0"},
			Border: api.Borders{
				Top:    api.Line{Width: 2, Color: api.Color{Hex: "333333"}},
				Right:  api.Line{Width: 2, Color: api.Color{Hex: "333333"}},
				Bottom: api.Line{Width: 2, Color: api.Color{Hex: "333333"}},
				Left:   api.Line{Width: 2, Color: api.Color{Hex: "333333"}},
			},
		},
		Circles: []CircleShape{
			{X: 50, Y: 40, Diameter: 30, Label: "C1"},
			{X: 150, Y: 40, Diameter: 25, Label: "C2"},
			{X: 100, Y: 110, Diameter: 35, Label: "C3"},
		},
		Cuts: []Cut{
			{Orientation: "horizontal", Position: 75, Width: 8, Label: "Cut1"},
			{Orientation: "vertical", Position: 100, Width: 6, Label: "Cut2"},
		},
		Labels: []Label{
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalCenter},
				},
				Text: api.Text{Content: "Basic SVG Box"},
			},
		},
		EnableCollisionAvoidance: true,
	}
	
	// Create SVG widget
	basicSVGWidget := NewSVGWidget(basicSVGBox).WithHeight(80)
	basicSVGWidget.Draw(builder)
	
	// Section 2: SVG Import from Content
	sectionHeader = Text{
		Text: api.Text{
			Content: "SVG Import from External Content",
			Class:   api.ResolveStyles("text-xl font-semibold mt-4 mb-2"),
		},
	}
	sectionHeader.Draw(builder)
	
	// Create SVG from external content
	externalSVGContent := `<?xml version="1.0"?>
<svg width="300" height="200" xmlns="http://www.w3.org/2000/svg">
    <circle cx="75" cy="75" r="35" fill="rgba(255,0,0,0.3)" id="red-circle"/>
    <circle cx="225" cy="75" r="40" fill="rgba(0,255,0,0.3)" id="green-circle"/>
    <circle cx="150" cy="125" r="30" fill="rgba(0,0,255,0.3)" id="blue-circle"/>
    <rect width="50" height="10" id="horizontal-cut"/>
    <rect width="8" height="80" id="vertical-cut"/>
</svg>`
	
	importedBox := api.Box{
		Rectangle: api.Rectangle{Width: 300, Height: 200},
		Fill:      api.Color{Hex: "ffffff"},
		Border: api.Borders{
			Top:    api.Line{Width: 1, Color: api.Color{Hex: "888888"}},
			Right:  api.Line{Width: 1, Color: api.Color{Hex: "888888"}},
			Bottom: api.Line{Width: 1, Color: api.Color{Hex: "888888"}},
			Left:   api.Line{Width: 1, Color: api.Color{Hex: "888888"}},
		},
	}
	
	importedSVGWidget, err := FromSVGContent(externalSVGContent, importedBox)
	if err != nil {
		// Fallback text if import fails
		errorText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("SVG Import Error: %v", err),
				Class:   api.ResolveStyles("text-red-600"),
			},
		}
		errorText.Draw(builder)
	} else {
		importedSVGWidget.WithHeight(60).Draw(builder)
	}
	
	// Section 3: Aspect Ratio Preservation Demo
	sectionHeader = Text{
		Text: api.Text{
			Content: "Aspect Ratio Preservation",
			Class:   api.ResolveStyles("text-xl font-semibold mt-4 mb-2"),
		},
	}
	sectionHeader.Draw(builder)
	
	// Landscape SVG (2:1 aspect ratio)
	landscapeSVGBox := SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{Width: 400, Height: 200},
			Fill:      api.Color{Hex: "e6f3ff"},
			Border: api.Borders{
				Top:    api.Line{Width: 1, Color: api.Color{Hex: "0066cc"}},
				Right:  api.Line{Width: 1, Color: api.Color{Hex: "0066cc"}},
				Bottom: api.Line{Width: 1, Color: api.Color{Hex: "0066cc"}},
				Left:   api.Line{Width: 1, Color: api.Color{Hex: "0066cc"}},
			},
		},
		Circles: []CircleShape{
			{X: 100, Y: 100, Diameter: 40, Label: "L1"},
			{X: 200, Y: 100, Diameter: 40, Label: "L2"},
			{X: 300, Y: 100, Diameter: 40, Label: "L3"},
		},
		Labels: []Label{
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalCenter},
				},
				Text: api.Text{Content: "Landscape (2:1 aspect)"},
			},
		},
	}
	
	landscapeWidget := NewSVGWidget(landscapeSVGBox).WithHeight(40)
	landscapeWidget.Draw(builder)
	
	// Portrait SVG (1:2 aspect ratio)
	portraitSVGBox := SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{Width: 150, Height: 300},
			Fill:      api.Color{Hex: "fff0e6"},
			Border: api.Borders{
				Top:    api.Line{Width: 1, Color: api.Color{Hex: "cc6600"}},
				Right:  api.Line{Width: 1, Color: api.Color{Hex: "cc6600"}},
				Bottom: api.Line{Width: 1, Color: api.Color{Hex: "cc6600"}},
				Left:   api.Line{Width: 1, Color: api.Color{Hex: "cc6600"}},
			},
		},
		Circles: []CircleShape{
			{X: 75, Y: 80, Diameter: 35, Label: "P1"},
			{X: 75, Y: 150, Diameter: 35, Label: "P2"},
			{X: 75, Y: 220, Diameter: 35, Label: "P3"},
		},
		Labels: []Label{
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalTop, Horizontal: HorizontalCenter},
				},
				Text: api.Text{Content: "Portrait (1:2 aspect)"},
			},
		},
	}
	
	portraitWidget := NewSVGWidget(portraitSVGBox).WithHeight(60)
	portraitWidget.Draw(builder)
	
	// Technical Note
	noteText := Text{
		Text: api.Text{
			Content: "Note: SVG widgets are converted to PNG with preserved aspect ratios using oksvg library before embedding in PDF.",
			Class:   api.ResolveStyles("text-sm text-gray-600 italic mt-4"),
		},
	}
	noteText.Draw(builder)
	
	// Section 4: SVG Converter Integration
	addSVGConverterDemo(builder)
	
	// Section 5: Dedicated converter pages
	if err := addSVGConverterPages(builder); err != nil {
		// Add error to PDF
		errorText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Error generating converter pages: %v", err),
				Class:   api.ResolveStyles("text-red-600 font-bold"),
			},
		}
		errorText.Draw(builder)
	}
}

// addSVGConverterDemo demonstrates the new SVG converter functionality
func addSVGConverterDemo(builder *Builder) {
	// Section header
	sectionHeader := Text{
		Text: api.Text{
			Content: "SVG Converter Integration",
			Class:   api.ResolveStyles("text-xl font-semibold mt-6 mb-2"),
		},
	}
	sectionHeader.Draw(builder)
	
	// Get available converters
	manager := NewSVGConverterManager()
	availableConverters := manager.GetAvailableConverters()
	
	if len(availableConverters) == 0 {
		noConvertersText := Text{
			Text: api.Text{
				Content: "No external SVG converters detected. Install inkscape, rsvg-convert, or playwright for enhanced SVG conversion capabilities.",
				Class:   api.ResolveStyles("text-orange-600 italic"),
			},
		}
		noConvertersText.Draw(builder)
		return
	}
	
	// Show available converters
	convertersText := Text{
		Text: api.Text{
			Content: fmt.Sprintf("Available SVG Converters: %s", strings.Join(availableConverters, ", ")),
			Class:   api.ResolveStyles("text-green-600 font-medium"),
		},
	}
	convertersText.Draw(builder)
	
	// Show supported formats
	supportedFormats := manager.GetSupportedFormats()
	formatsText := Text{
		Text: api.Text{
			Content: fmt.Sprintf("Supported Output Formats: %s", strings.Join(supportedFormats, ", ")),
			Class:   api.ResolveStyles("text-blue-600"),
		},
	}
	formatsText.Draw(builder)
	
	// Demo converter functionality (if available)
	if len(availableConverters) > 0 {
		demoConverterFunctionality(builder, manager)
	}
}

// demoConverterFunctionality creates a live demo of SVG conversion
func demoConverterFunctionality(builder *Builder, manager *SVGConverterManager) {
	// Create a test SVG
	testSVG := CreateTestSVG()
	
	// Try to create a temporary file
	tempDir := os.TempDir()
	svgPath := filepath.Join(tempDir, "showcase_demo.svg")
	
	// Write SVG to file
	if err := os.WriteFile(svgPath, []byte(testSVG), 0644); err != nil {
		errorText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Failed to create demo SVG file: %v", err),
				Class:   api.ResolveStyles("text-red-600"),
			},
		}
		errorText.Draw(builder)
		return
	}
	defer os.Remove(svgPath) // Clean up
	
	// Demo 1: Direct SVG usage with automatic conversion
	demoTitle1 := Text{
		Text: api.Text{
			Content: "Demo 1: Direct SVG file with automatic conversion",
			Class:   api.ResolveStyles("text-md font-semibold mt-4 mb-2"),
		},
	}
	demoTitle1.Draw(builder)
	
	// Use the Image widget directly with an SVG file
	// The widget will automatically detect and convert the SVG
	svgImage := Image{
		Source:  svgPath,
		AltText: "Test SVG converted automatically",
		Width:   floatPtr(50),
		Height:  floatPtr(50),
	}
	if err := svgImage.Draw(builder); err != nil {
		errorText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Failed to draw SVG image: %v", err),
				Class:   api.ResolveStyles("text-red-600"),
			},
		}
		errorText.Draw(builder)
	} else {
		successText := Text{
			Text: api.Text{
				Content: "✓ SVG automatically detected and converted",
				Class:   api.ResolveStyles("text-green-600 text-sm"),
			},
		}
		successText.Draw(builder)
	}
	
	// Demo 2: SVG with specific converter preference
	demoTitle2 := Text{
		Text: api.Text{
			Content: "Demo 2: SVG with preferred converter",
			Class:   api.ResolveStyles("text-md font-semibold mt-4 mb-2"),
		},
	}
	demoTitle2.Draw(builder)
	
	// Get first available converter
	availableConverters := manager.GetAvailableConverters()
	if len(availableConverters) > 0 {
		preferredConverter := availableConverters[0]
		
		svgImageWithPreference := Image{
			Source:             svgPath,
			AltText:            fmt.Sprintf("Using %s converter", preferredConverter),
			Width:              floatPtr(40),
			Height:             floatPtr(40),
			PreferredConverter: preferredConverter,
			ConverterOptions: &ConvertOptions{
				Format: "png",
				DPI:    150,
			},
		}
		
		if err := svgImageWithPreference.Draw(builder); err != nil {
			errorText := Text{
				Text: api.Text{
					Content: fmt.Sprintf("Failed with %s: %v", preferredConverter, err),
					Class:   api.ResolveStyles("text-red-600 text-sm"),
				},
			}
			errorText.Draw(builder)
		} else {
			successText := Text{
				Text: api.Text{
					Content: fmt.Sprintf("✓ Converted using %s converter", preferredConverter),
					Class:   api.ResolveStyles("text-green-600 text-sm"),
				},
			}
			successText.Draw(builder)
		}
	}
	
	// Technical details
	detailsText := Text{
		Text: api.Text{
			Content: "The Image widget now automatically detects SVG files and converts them using available converters (Inkscape, RSVG, Playwright) with automatic fallback.",
			Class:   api.ResolveStyles("text-sm text-gray-700 mt-2"),
		},
	}
	detailsText.Draw(builder)
}

// addSVGConverterPages creates dedicated pages for each available converter
func addSVGConverterPages(builder *Builder) error {
	manager := NewSVGConverterManager()
	availableConverters := manager.GetAvailableConverters()
	
	if len(availableConverters) == 0 {
		return nil // No converters available
	}
	
	// Create a complex SVG box with all features for testing
	complexSVGBox := createComplexSVGBoxForTesting()
	
	// Create a page for each converter
	for _, converterName := range availableConverters {
		converter, err := manager.GetConverter(converterName)
		if err != nil {
			continue
		}
		
		// Add a new page for this converter
		builder.AddPage()
		
		// Page title
		titleText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("SVG Converter: %s", converterName),
				Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
			},
		}
		titleText.Draw(builder)
		
		// Converter info
		infoText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Supported formats: %s", strings.Join(converter.SupportedFormats(), ", ")),
				Class:   api.ResolveStyles("text-sm text-gray-600 mb-4"),
			},
		}
		infoText.Draw(builder)
		
		// Create the SVG widget and draw it
		svgWidget := NewSVGWidget(complexSVGBox).WithHeight(100)
		
		// Add section for original SVG
		origText := Text{
			Text: api.Text{
				Content: "Original SVG (rendered with oksvg):",
				Class:   api.ResolveStyles("text-lg font-semibold mt-2 mb-2"),
			},
		}
		origText.Draw(builder)
		svgWidget.Draw(builder)
		
		// Generate SVG content directly from the box
		svgBytes, err := complexSVGBox.GenerateSVG()
		if err != nil {
			return fmt.Errorf("failed to generate SVG for converter %s: %w", converterName, err)
		}
		svgContent := string(svgBytes)
		
		// Save SVG to temp file
		tempDir := os.TempDir()
		svgPath := filepath.Join(tempDir, fmt.Sprintf("showcase_%s.svg", converterName))
		if err := os.WriteFile(svgPath, []byte(svgContent), 0644); err != nil {
			return fmt.Errorf("failed to write SVG file for converter %s: %w", converterName, err)
		}
		// Don't defer removal here - we'll clean up at the end
		// defer os.Remove(svgPath)
		
		// Keep track of files to clean up later
		var filesToCleanup []string
		filesToCleanup = append(filesToCleanup, svgPath)
		
		// Defer cleanup to the very end
		defer func() {
			for _, f := range filesToCleanup {
				os.Remove(f)
			}
		}()
		
		// First demonstrate direct SVG usage with the Image widget
		directSVGTitle := Text{
			Text: api.Text{
				Content: "Direct SVG File Usage (Automatic Conversion)",
				Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
			},
		}
		directSVGTitle.Draw(builder)
		
		// Use Image widget directly with SVG file
		directSVGImage := Image{
			Source:             svgPath,
			AltText:            fmt.Sprintf("Direct SVG using %s", converterName),
			Width:              floatPtr(60),
			Height:             floatPtr(45),
			PreferredConverter: converterName,
			ConverterOptions: &ConvertOptions{
				Format: "png",
				DPI:    96,
			},
		}
		
		if err := directSVGImage.Draw(builder); err != nil {
			errorText := Text{
				Text: api.Text{
					Content: fmt.Sprintf("Failed to render SVG directly: %v", err),
					Class:   api.ResolveStyles("text-red-600 text-sm"),
				},
			}
			errorText.Draw(builder)
		} else {
			successText := Text{
				Text: api.Text{
					Content: fmt.Sprintf("✓ SVG rendered directly using %s converter", converterName),
					Class:   api.ResolveStyles("text-green-600 text-sm"),
				},
			}
			successText.Draw(builder)
		}
		
		// Test different formats and sizes
		testConfigs := []struct {
			format string
			width  int
			height int
			scale  string
		}{
			{"png", 200, 150, "Small (200x150)"},
			{"png", 400, 300, "Medium (400x300)"},
			{"png", 800, 600, "Large (800x600)"},
		}
		
		// Add JPEG tests if supported
		if supportsFormat(converter, "jpeg") || supportsFormat(converter, "jpg") {
			testConfigs = append(testConfigs, 
				struct {
					format string
					width  int
					height int
					scale  string
				}{"jpeg", 400, 300, "JPEG (400x300)"})
		}
		
		ctx := context.Background()
		
		for _, config := range testConfigs {
			// Section header for this test
			sectionText := Text{
				Text: api.Text{
					Content: fmt.Sprintf("%s - %s", strings.ToUpper(config.format), config.scale),
					Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
				},
			}
			sectionText.Draw(builder)
			
			// Convert with timing
			startTime := time.Now()
			outputPath := strings.TrimSuffix(svgPath, ".svg") + fmt.Sprintf("_%dx%d.%s", config.width, config.height, config.format)
			// Don't defer removal - add to cleanup list
			filesToCleanup = append(filesToCleanup, outputPath)
			
			options := &ConvertOptions{
				Format:  config.format,
				Width:   config.width,
				Height:  config.height,
				DPI:     96,
				Quality: 90, // For JPEG
			}
			
			err := converter.Convert(ctx, svgPath, outputPath, options)
			conversionTime := time.Since(startTime)
			
			if err != nil {
				return fmt.Errorf("conversion failed for %s %s at %s: %w", converterName, config.format, config.scale, err)
			}
			
			// Get file size
			fileInfo, err := os.Stat(outputPath)
			var fileSize int64
			if err == nil {
				fileSize = fileInfo.Size()
			}
			
			// Display the converted image (if PNG)
			if config.format == "png" {
				// Verify file exists before trying to embed
				if _, err := os.Stat(outputPath); err != nil {
					return fmt.Errorf("converted image file not found at %s: %w", outputPath, err)
				}
				
				// Try to embed the image in the PDF
				imageWidget := Image{
					Source:  outputPath,
					AltText: fmt.Sprintf("%s render at %s", converterName, config.scale),
					Width:   floatPtr(float64(config.width) / 4), // Scale down for PDF display
					Height:  floatPtr(float64(config.height) / 4),
				}
				if err := imageWidget.Draw(builder); err != nil {
					return fmt.Errorf("failed to embed image %s: %w", outputPath, err)
				}
			} else {
				// For non-PNG formats, show a placeholder
				placeholderText := Text{
					Text: api.Text{
						Content: fmt.Sprintf("[%s Image: %s]", strings.ToUpper(config.format), config.scale),
						Class:   api.ResolveStyles("text-gray-500 italic border p-2"),
					},
				}
				placeholderText.Draw(builder)
			}
			
			// Show timing and size info
			statsText := Text{
				Text: api.Text{
					Content: fmt.Sprintf("⏱ Time: %v | 📦 Size: %s | ✅ Success", 
						conversionTime.Round(time.Millisecond),
						formatFileSize(fileSize)),
					Class: api.ResolveStyles("text-sm text-green-600 mt-1"),
				},
			}
			statsText.Draw(builder)
		}
		
		// Add converter-specific notes
		notesText := Text{
			Text: api.Text{
				Content: getConverterNotes(converterName),
				Class:   api.ResolveStyles("text-xs text-gray-600 italic mt-4"),
			},
		}
		notesText.Draw(builder)
	}
	
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
		ShowDimensions:          true,
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

// floatPtr is a helper to create a pointer to a float64
func floatPtr(f float64) *float64 {
	return &f
}

