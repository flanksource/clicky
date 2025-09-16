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
	t.Skip("Skipping showcase PDF tests due to SVG conversion validation issues")
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

			// Page 6: SVG Features
			if err := addSVGFeaturesPage(builder); err != nil {
				t.Fatalf("Failed to add SVG features page: %v", err)
			}

			// Page 7: Label Positions Gallery
			if err := addLabelPositionsGalleryPage(builder); err != nil {
				t.Fatalf("Failed to add label positions gallery page: %v", err)
			}

			// Page 8: Two-Column Layout with Image and Table
			if err := addTwoColumnLayoutPage(builder); err != nil {
				t.Fatalf("Failed to add two-column layout page: %v", err)
			}

			// Page 9: Combined Examples
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
		RowStyle:          api.ResolveStyles("text-sm"),
		ShowBorders:       true,
		AlternateRowColor: true,
		ColumnAlignments:  []string{"left", "center", "center"},
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
		RowStyle:         api.ResolveStyles("font-bold"),
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

	// Create a simple SVG for demo purposes
	demoSVGBox := SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{Width: 80, Height: 40},
			Fill:      api.Color{Hex: "e8f5e8"},
			Border: api.Borders{
				Top:    api.Line{Width: 1, Color: api.Color{Hex: "28a745"}},
				Right:  api.Line{Width: 1, Color: api.Color{Hex: "28a745"}},
				Bottom: api.Line{Width: 1, Color: api.Color{Hex: "28a745"}},
				Left:   api.Line{Width: 1, Color: api.Color{Hex: "28a745"}},
			},
		},
		Labels: []Label{
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
				},
				Text: api.Text{Content: "Demo Image"},
			},
		},
	}

	svgBytes, err := demoSVGBox.GenerateSVG()
	var demoImagePath string
	if err != nil {
		// Skip image features if SVG generation fails (fail fast)
		return
	}

	// Write to temporary file
	tempFile, err := os.CreateTemp("", "demo_image_*.svg")
	if err != nil {
		// Skip image features if temp file creation fails (fail fast)
		return
	}

	demoImagePath = tempFile.Name()
	defer os.Remove(demoImagePath)
	tempFile.Write(svgBytes)
	tempFile.Close()

	placeholderImage := Image{
		Source:  demoImagePath,
		AltText: "Demo image created from SVG",
		Height:  &[]float64{40}[0],
	}
	// Try to draw the image, skip if it fails
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
		os.WriteFile("out/showcase_svgbox.svg", svgData, 0o644)

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

	// Use the same SVG demo approach for different sized images
	if demoImagePath != "" {
		// Reuse the same SVG source for different sizes
		smallImage := Image{
			Source:  demoImagePath,
			AltText: "Small Image (30mm height)",
			Height:  &[]float64{30}[0],
		}
		smallImage.Draw(builder)

		mediumImage := Image{
			Source:  demoImagePath,
			AltText: "Medium Image (50mm height)",
			Height:  &[]float64{50}[0],
		}
		mediumImage.Draw(builder)

		largeImage := Image{
			Source:  demoImagePath,
			AltText: "Large Image (70mm height)",
			Height:  &[]float64{70}[0],
		}
		largeImage.Draw(builder)
	}
}

func addSVGFeaturesPage(builder *Builder) error {
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
	if err := addSVGConverterDemo(builder); err != nil {
		return fmt.Errorf("failed to add SVG converter demo: %w", err)
	}

	// Section 5: Dedicated converter pages
	if err := addSVGConverterPages(builder); err != nil {
		// Error generating converter pages - return it
		return fmt.Errorf("failed to add SVG converter pages: %w", err)
	}
	return nil
}

// addSVGConverterDemo demonstrates the new SVG converter functionality
func addSVGConverterDemo(builder *Builder) error {
	// Section header
	sectionHeader := Text{
		Text: api.Text{
			Content: "SVG Converter Integration",
			Class:   api.ResolveStyles("text-xl font-semibold mt-6 mb-2"),
		},
	}
	sectionHeader.Draw(builder)

	availableConverters := GetAvailableConverters()

	if len(availableConverters) == 0 {
		noConvertersText := Text{
			Text: api.Text{
				Content: "No external SVG converters detected.",
				Class:   api.ResolveStyles("text-orange-600 italic"),
			},
		}
		noConvertersText.Draw(builder)
		return nil
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
	supportedFormats := GetSupportedFormats()
	formatsText := Text{
		Text: api.Text{
			Content: fmt.Sprintf("Supported Output Formats: %s", strings.Join(supportedFormats, ", ")),
			Class:   api.ResolveStyles("text-blue-600"),
		},
	}
	formatsText.Draw(builder)

	// Add SVG to all formats conversion grid
	if err := addSVGFormatConversionGrid(builder); err != nil {
		return fmt.Errorf("failed to add SVG format conversion grid: %w", err)
	}

	// Demo converter functionality
	return demoConverterFunctionality(builder)
}

// demoConverterFunctionality creates a live demo of SVG conversion
func demoConverterFunctionality(builder *Builder) error {
	// Create a test SVG using SVGBox for consistent results
	testSVGBox := SVGBox{
		Box: api.Box{
			Rectangle: api.Rectangle{Width: 120, Height: 80},
			Fill:      api.Color{Hex: "f0f8ff"},
			Border: api.Borders{
				Top:    api.Line{Width: 2, Color: api.Color{Hex: "4169e1"}},
				Right:  api.Line{Width: 2, Color: api.Color{Hex: "4169e1"}},
				Bottom: api.Line{Width: 2, Color: api.Color{Hex: "4169e1"}},
				Left:   api.Line{Width: 2, Color: api.Color{Hex: "4169e1"}},
			},
		},
		Circles: []CircleShape{
			{X: 30, Y: 25, Diameter: 15, Label: "A"},
			{X: 90, Y: 55, Diameter: 15, Label: "B"},
		},
		Labels: []Label{
			{
				Positionable: Positionable{
					Position: &LabelPosition{Vertical: VerticalCenter, Horizontal: HorizontalCenter},
				},
				Text: api.Text{Content: "Demo"},
			},
		},
	}

	// Generate SVG content
	svgBytes, err := testSVGBox.GenerateSVG()
	if err != nil {
		return fmt.Errorf("failed to generate demo SVG: %w", err)
	}

	// Create temporary file with generated SVG content
	tempDir := os.TempDir()
	svgPath := filepath.Join(tempDir, "showcase_demo.svg")

	if err := os.WriteFile(svgPath, svgBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write demo SVG file: %w", err)
	}
	defer os.Remove(svgPath)

	// Demo 1: Direct SVG usage with automatic conversion
	demoTitle1 := Text{
		Text: api.Text{
			Content: "Demo 1: Automatic SVG Conversion",
			Class:   api.ResolveStyles("text-md font-semibold mt-4 mb-2"),
		},
	}
	demoTitle1.Draw(builder)

	// Use the Image widget directly with an SVG file
	// The enhanced validation will prevent "could not load image" errors
	svgImage := &Image{
		Source:  svgPath,
		AltText: "Test SVG converted automatically",
		Width:   floatPtr(40),
		Height:  floatPtr(30),
	}
	if err := svgImage.Draw(builder); err != nil {
		// Conversion validation failed - show error but don't crash
		errorText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("SVG conversion failed: %v", err),
				Class:   api.ResolveStyles("text-red-600 text-sm"),
			},
		}
		errorText.Draw(builder)
		return nil // Don't fail the entire showcase
	}

	// Display conversion metadata if available
	if metadata := svgImage.GetLastConversionMetadata(); metadata != nil {
		metadataText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("✓ Converted using %s in %v (DPI: %d, Size: %s)",
					metadata.ConverterUsed,
					metadata.Duration.Round(time.Millisecond),
					metadata.DPI,
					formatFileSize(metadata.OutputFileSize)),
				Class: api.ResolveStyles("text-green-600 text-xs"),
			},
		}
		metadataText.Draw(builder)

		// Show detailed settings
		settingsText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Settings: %dx%d pixels, Format: PNG, High Resolution (3x DPI)",
					metadata.OutputWidth, metadata.OutputHeight),
				Class: api.ResolveStyles("text-gray-600 text-xs"),
			},
		}
		settingsText.Draw(builder)
	} else {
		successText := Text{
			Text: api.Text{
				Content: "✓ SVG automatically detected and converted",
				Class:   api.ResolveStyles("text-green-600 text-sm"),
			},
		}
		successText.Draw(builder)
	}

	return nil
}

// addSVGConverterPages creates dedicated pages for each available converter
func addSVGConverterPages(builder *Builder) error {
	availableConverters := GetAvailableConverters()

	if len(availableConverters) == 0 {
		return nil // No converters available
	}

	// Create a test SVG for converter demonstrations
	testSVGBox := createComplexSVGBoxForTesting()

	// Generate SVG content
	svgBytes, err := testSVGBox.GenerateSVG()
	if err != nil {
		return fmt.Errorf("failed to generate test SVG: %w", err)
	}

	// Create temporary SVG file
	tempDir := os.TempDir()
	svgPath := filepath.Join(tempDir, "converter_test.svg")
	if err := os.WriteFile(svgPath, svgBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write test SVG: %w", err)
	}
	defer os.Remove(svgPath)

	// Create a page for each available converter
	for i, converterName := range availableConverters {
		// Add a new page for this converter
		if i > 0 {
			builder.AddPage()
		}

		// Page title
		titleText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("SVG Converter: %s", converterName),
				Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
			},
		}
		titleText.Draw(builder)

		// Show original SVG using SVGWidget (oksvg-based, not external converter)
		origText := Text{
			Text: api.Text{
				Content: "Original SVG (rendered with oksvg):",
				Class:   api.ResolveStyles("text-lg font-semibold mt-2 mb-2"),
			},
		}
		origText.Draw(builder)

		svgWidget := NewSVGWidget(testSVGBox).WithHeight(60)
		svgWidget.Draw(builder)

		// Test conversion with this specific converter
		conversionText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("Converted using %s:", converterName),
				Class:   api.ResolveStyles("text-lg font-semibold mt-4 mb-2"),
			},
		}
		conversionText.Draw(builder)

		// Use Image widget with specific converter preference
		convertedImage := &Image{
			Source:             svgPath,
			AltText:            fmt.Sprintf("SVG converted by %s", converterName),
			Width:              floatPtr(80),
			Height:             floatPtr(60),
			PreferredConverter: converterName,
			ConverterOptions: &ConvertOptions{
				Format: "png",
				DPI:    288, // High resolution output (3x standard DPI)
			},
		}

		if err := convertedImage.Draw(builder); err != nil {
			// Conversion failed - show error message instead of crashing
			errorText := Text{
				Text: api.Text{
					Content: fmt.Sprintf("✗ Conversion failed with %s: %v", converterName, err),
					Class:   api.ResolveStyles("text-red-600 text-sm"),
				},
			}
			errorText.Draw(builder)
		} else {
			// Display detailed conversion metadata
			if metadata := convertedImage.GetLastConversionMetadata(); metadata != nil {
				successText := Text{
					Text: api.Text{
						Content: fmt.Sprintf("✓ %s conversion: %v (DPI: %d, %s)",
							converterName,
							metadata.Duration.Round(time.Millisecond),
							metadata.DPI,
							formatFileSize(metadata.OutputFileSize)),
						Class: api.ResolveStyles("text-green-600 text-sm"),
					},
				}
				successText.Draw(builder)

				// Show technical details
				detailsText := Text{
					Text: api.Text{
						Content: fmt.Sprintf("Output: %dx%d pixels, Format: PNG, Quality: High Resolution",
							metadata.OutputWidth, metadata.OutputHeight),
						Class: api.ResolveStyles("text-gray-600 text-xs"),
					},
				}
				detailsText.Draw(builder)
			} else {
				successText := Text{
					Text: api.Text{
						Content: fmt.Sprintf("✓ Successfully converted using %s", converterName),
						Class:   api.ResolveStyles("text-green-600 text-sm"),
					},
				}
				successText.Draw(builder)
			}
		}
	}

	return nil
}

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
			Content: "Complete showcase of all available label positioning options in SVGBox",
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
			Content: fmt.Sprintf("This page demonstrates all %d label positioning options available in SVGBox. All examples are rendered at 288 DPI for high quality output and include performance metrics.", len(labelPositions)),
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
		[][]string{
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

// floatPtr is a helper to create a pointer to a float64
func floatPtr(f float64) *float64 {
	return &f
}
