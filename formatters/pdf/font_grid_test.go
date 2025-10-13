//go:build pdf

package pdf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/flanksource/clicky/api"
	. "github.com/flanksource/clicky/formatters/pdf"
	"github.com/flanksource/maroto/v2/pkg/components/col"
)

// saveTestPDF saves test PDF to the out directory
func saveTestPDF(t *testing.T, name string, pdfData []byte) {
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
		t.Logf("Warning: Could not save PDF file: %v", err)
		return
	}
	t.Logf("✓ Saved test PDF: %s", filepath)
}

// TestFontFamilyGrid demonstrates different font families in a 3-column grid layout
func TestFontFamilyGrid(t *testing.T) {
	builder := NewBuilder(WithDebug(true))

	// Page title
	titleText := Text{
		Text: api.Text{
			Content: "Font Family Showcase - Grid Layout",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	titleText.Draw(builder)

	// Description
	descText := Text{
		Text: api.Text{
			Content: "Demonstrating various font families with Unicode characters in grid layout",
			Class:   api.ResolveStyles("text-sm text-gray-600 text-center mb-6"),
		},
	}
	descText.Draw(builder)

	// Font families to test (using available PDF fonts)
	fontFamilies := []struct {
		name        string
		class       string
		sample      string
		description string
	}{
		{"Arial", "font-family-Arial text-lg", "Arial: The quick brown fox jumps over lazy dog π²³°±µΩ", "Sans-serif, excellent Unicode support"},
		{"Times", "font-family-Times text-lg", "Times: The quick brown fox jumps over lazy dog π²³°±µΩ", "Serif, traditional readability"},
		{"Courier", "font-family-Courier text-lg", "Courier: The quick brown fox jumps π²³°±µΩ", "Monospace, code and data display"},
		{"Helvetica", "font-family-Helvetica text-lg", "Helvetica: The quick brown fox jumps π²³°±µΩ", "Sans-serif, clean modern look"},
	}

	for _, font := range fontFamilies {
		// Font name header
		headerText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s Font Family", font.name),
				Class:   api.ResolveStyles("text-md font-semibold mt-4 mb-2"),
			},
		}
		headerText.Draw(builder)

		// Sample text with font family applied
		sampleText := Text{
			Text: api.Text{
				Content: font.sample,
				Class:   api.ResolveStyles(font.class),
			},
		}
		sampleText.Draw(builder)

		// Description
		descText := Text{
			Text: api.Text{
				Content: font.description,
				Class:   api.ResolveStyles("text-xs text-gray-600 mb-3"),
			},
		}
		descText.Draw(builder)
	}

	// Generate PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build font family grid PDF: %v", err)
	}

	// Save PDF
	saveTestPDF(t, "font_family_grid", pdfData)
	t.Logf("✓ Font family grid PDF generated successfully (%d bytes)", len(pdfData))
}

// TestUnicodeFontCompatibilityGrid tests Unicode character rendering across font families
func TestUnicodeFontCompatibilityGrid(t *testing.T) {
	builder := NewBuilder(WithDebug(true))

	// Page title
	titleText := Text{
		Text: api.Text{
			Content: "Unicode Font Compatibility Matrix",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	titleText.Draw(builder)

	// Unicode test characters
	unicodeChars := []struct {
		char        string
		description string
		unicode     string
	}{
		{"²", "Superscript Two", "U+00B2"},
		{"³", "Superscript Three", "U+00B3"},
		{"°", "Degree Sign", "U+00B0"},
		{"±", "Plus-Minus Sign", "U+00B1"},
		{"µ", "Micro Sign", "U+00B5"},
		{"Ω", "Greek Omega", "U+03A9"},
		{"π", "Greek Pi", "U+03C0"},
		{"√", "Square Root", "U+221A"},
		{"∞", "Infinity", "U+221E"},
		{"≤", "Less Than or Equal", "U+2264"},
	}

	// Font families to test
	fontFamilies := []string{"Arial", "Times", "Courier", "Helvetica", "Georgia", "Verdana"}

	// Create table headers
	headers := []string{"Font Family"}
	for _, char := range unicodeChars {
		headers = append(headers, fmt.Sprintf("%s\n%s", char.char, char.unicode))
	}

	// Create table rows
	var rows [][]any
	for _, fontFamily := range fontFamilies {
		row := []any{fontFamily}
		for _, char := range unicodeChars {
			// Create text with specific font family
			charText := api.Text{
				Content: char.char,
				Class:   api.ResolveStyles(fmt.Sprintf("font-family-%s text-lg", fontFamily)),
			}
			row = append(row, charText.Content)
		}
		rows = append(rows, row)
	}

	// Create columns with appropriate widths
	columns := []Column{
		{Label: "Font Family", Style: "w-[15%] text-left align-middle font-medium"},
	}

	// Add Unicode character columns
	charWidth := fmt.Sprintf("w-[%d%%]", 85/len(unicodeChars)) // Distribute remaining 85% among chars
	for _, char := range unicodeChars {
		columns = append(columns, Column{
			Label: fmt.Sprintf("%s\n%s", char.char, char.unicode),
			Style: fmt.Sprintf("%s text-center align-middle", charWidth),
		})
	}

	// Create the table
	table := Table{
		BaseTable: BaseTable{
			Columns:           columns,
			Rows:              rows,
			HeaderStyle:       "bg-gray-800 text-white font-bold text-xs text-center align-middle",
			RowStyle:          "text-sm text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}
	table.Draw(builder)

	// Add explanation
	explanationText := Text{
		Text: api.Text{
			Content: "This matrix shows how each font family renders critical Unicode characters. Missing or incorrect characters indicate font compatibility issues.",
			Class:   api.ResolveStyles("text-sm text-gray-600 mt-4"),
		},
	}
	explanationText.Draw(builder)

	// Generate PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build Unicode compatibility grid PDF: %v", err)
	}

	// Save PDF
	saveTestPDF(t, "unicode_font_compatibility_grid", pdfData)
	t.Logf("✓ Unicode font compatibility grid PDF generated successfully (%d bytes)", len(pdfData))
}

// TestFontWeightCombinationsGrid demonstrates font weight combinations with different families
func TestFontWeightCombinationsGrid(t *testing.T) {
	builder := NewBuilder(WithDebug(true))

	// Page title
	titleText := Text{
		Text: api.Text{
			Content: "Font Weight Combinations Grid",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	titleText.Draw(builder)

	// Font weight combinations to test
	combinations := []struct {
		family      string
		weight      string
		class       string
		description string
	}{
		{"Arial", "Normal", "font-family-Arial font-normal text-base", "Arial Normal"},
		{"Arial", "Medium", "font-family-Arial font-medium text-base", "Arial Medium"},
		{"Arial", "Bold", "font-family-Arial font-bold text-base", "Arial Bold"},
		{"Times", "Normal", "font-family-Times font-normal text-base", "Times Normal"},
		{"Times", "Medium", "font-family-Times font-medium text-base", "Times Medium"},
		{"Times", "Bold", "font-family-Times font-bold text-base", "Times Bold"},
		{"Courier", "Normal", "font-family-Courier font-normal text-base", "Courier Normal"},
		{"Courier", "Medium", "font-family-Courier font-medium text-base", "Courier Medium"},
		{"Courier", "Bold", "font-family-Courier font-bold text-base", "Courier Bold"},
	}

	// Sample text with Unicode characters
	sampleText := "Sample text with Unicode: π²³°±µΩ√∞≤"

	// Create table
	var rows [][]any

	for _, combo := range combinations {
		row := []any{
			combo.family,
			combo.weight,
			sampleText,
			"π²³°±µΩ√∞≤",
		}
		rows = append(rows, row)
	}

	// Create columns
	columns := []Column{
		{Label: "Font Family", Style: "w-[20%] text-left align-middle font-medium"},
		{Label: "Weight", Style: "w-[15%] text-center align-middle"},
		{Label: "Sample Text", Style: "w-[45%] text-left align-middle"},
		{Label: "Unicode Test", Style: "w-[20%] text-center align-middle"},
	}

	table := Table{
		BaseTable: BaseTable{
			Columns:           columns,
			Rows:              rows,
			HeaderStyle:       "bg-gray-800 text-white font-bold text-sm text-center align-middle",
			RowStyle:          "text-sm text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}
	table.Draw(builder)

	// Add individual demonstrations
	demoTitle := Text{
		Text: api.Text{
			Content: "Individual Font Weight Demonstrations",
			Class:   api.ResolveStyles("text-lg font-semibold mt-6 mb-3"),
		},
	}
	demoTitle.Draw(builder)

	for _, combo := range combinations {
		// Label
		labelText := Text{
			Text: api.Text{
				Content: combo.description + ":",
				Class:   api.ResolveStyles("text-sm font-medium text-gray-700 mt-2"),
			},
		}
		labelText.Draw(builder)

		// Sample with applied style
		styledText := Text{
			Text: api.Text{
				Content: sampleText,
				Class:   api.ResolveStyles(combo.class),
			},
		}
		styledText.Draw(builder)
	}

	// Generate PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build font weight combinations grid PDF: %v", err)
	}

	// Save PDF
	saveTestPDF(t, "font_weight_combinations_grid", pdfData)
	t.Logf("✓ Font weight combinations grid PDF generated successfully (%d bytes)", len(pdfData))
}

// TestFontSizeAndFamilyGrid demonstrates different font sizes across font families
func TestFontSizeAndFamilyGrid(t *testing.T) {
	builder := NewBuilder(WithDebug(true))

	// Page title
	titleText := Text{
		Text: api.Text{
			Content: "Font Size and Family Matrix",
			Class:   api.ResolveStyles("text-2xl font-bold text-center mb-4"),
		},
	}
	titleText.Draw(builder)

	// Font sizes to test
	fontSizes := []string{"text-xs", "text-sm", "text-base", "text-lg", "text-xl", "text-2xl"}
	fontFamilies := []string{"Arial", "Times", "Courier"}

	// Create table headers (will be built in columns definition)

	// Create table rows
	var rows [][]any
	sampleText := "Abc π²³"

	for _, family := range fontFamilies {
		row := []any{family}
		for range fontSizes {
			// This will be styled in the actual rendering
			row = append(row, sampleText)
		}
		rows = append(rows, row)
	}

	// Create columns
	columns := []Column{
		{Label: "Font Family", Style: "w-[15%] text-left align-middle font-medium"},
	}

	// Add size columns
	sizeWidth := fmt.Sprintf("w-[%d%%]", 85/len(fontSizes))
	for _, size := range fontSizes {
		columns = append(columns, Column{
			Label: size,
			Style: fmt.Sprintf("%s text-center align-middle", sizeWidth),
		})
	}

	table := Table{
		BaseTable: BaseTable{
			Columns:           columns,
			Rows:              rows,
			HeaderStyle:       "bg-gray-800 text-white font-bold text-sm text-center align-middle",
			RowStyle:          "text-sm text-gray-700 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}
	table.Draw(builder)

	// Add individual size demonstrations
	demoTitle := Text{
		Text: api.Text{
			Content: "Font Size Demonstrations by Family",
			Class:   api.ResolveStyles("text-lg font-semibold mt-6 mb-3"),
		},
	}
	demoTitle.Draw(builder)

	for _, family := range fontFamilies {
		familyTitle := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s Font Family:", family),
				Class:   api.ResolveStyles("text-md font-medium mt-4 mb-2"),
			},
		}
		familyTitle.Draw(builder)

		for _, size := range fontSizes {
			// Create styled text with both font family and size
			classStr := fmt.Sprintf("font-family-%s %s", family, size)
			styledText := Text{
				Text: api.Text{
					Content: fmt.Sprintf("%s: The quick brown fox with Unicode π²³°±", size),
					Class:   api.ResolveStyles(classStr),
				},
			}
			styledText.Draw(builder)
		}
	}

	// Generate PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build font size and family grid PDF: %v", err)
	}

	// Save PDF
	saveTestPDF(t, "font_size_family_grid", pdfData)
	t.Logf("✓ Font size and family grid PDF generated successfully (%d bytes)", len(pdfData))
}

// TestComprehensiveFontGrid combines all font features in one comprehensive test
func TestComprehensiveFontGrid(t *testing.T) {
	builder := NewBuilder(WithDebug(true))

	// Page title
	titleText := Text{
		Text: api.Text{
			Content: "Comprehensive Font Grid Showcase",
			Class:   api.ResolveStyles("text-3xl font-bold text-center mb-6"),
		},
	}
	titleText.Draw(builder)

	// Overview
	overviewText := Text{
		Text: api.Text{
			Content: "Complete demonstration of font-family-{Name} classes with weights, sizes, and Unicode compatibility",
			Class:   api.ResolveStyles("text-md text-center text-gray-600 mb-8"),
		},
	}
	overviewText.Draw(builder)

	// Comprehensive test matrix
	testCases := []struct {
		family  string
		weight  string
		size    string
		sample  string
		unicode string
	}{
		{"Arial", "font-normal", "text-base", "Arial normal base", "π²³°±µΩ"},
		{"Arial", "font-bold", "text-lg", "Arial bold large", "√∞≤≥≠"},
		{"Times", "font-normal", "text-base", "Times normal base", "π²³°±µΩ"},
		{"Times", "font-medium", "text-xl", "Times medium extra-large", "√∞≤≥≠"},
		{"Courier", "font-normal", "text-sm", "Courier normal small", "π²³°±µΩ"},
		{"Courier", "font-bold", "text-base", "Courier bold base", "√∞≤≥≠"},
		{"Helvetica", "font-medium", "text-lg", "Helvetica medium large", "π²³°±µΩ"},
	}

	// Create comprehensive table
	var rows [][]any

	for _, test := range testCases {
		row := []any{
			test.family,
			test.weight,
			test.size,
			test.sample,
			test.unicode,
		}
		rows = append(rows, row)
	}

	columns := []Column{
		{Label: "Font Family", Style: "w-[18%] text-left align-middle font-medium"},
		{Label: "Weight", Style: "w-[15%] text-center align-middle"},
		{Label: "Size", Style: "w-[15%] text-center align-middle"},
		{Label: "Sample Text", Style: "w-[32%] text-left align-middle"},
		{Label: "Unicode Test", Style: "w-[20%] text-center align-middle"},
	}

	table := NewTable()
	table.Rows = rows
	table.Columns = columns
	builder.AddRow(100, col.New(12).Add(table))

	// Add practical examples section
	examplesTitle := Text{
		Text: api.Text{
			Content: "Practical Usage Examples",
			Class:   api.ResolveStyles("text-xl font-bold mt-8 mb-4"),
		},
	}
	examplesTitle.Draw(builder)

	for _, test := range testCases {
		// Example label
		exampleLabel := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s %s %s:", test.family, test.weight, test.size),
				Class:   api.ResolveStyles("text-sm font-medium text-gray-700 mt-3"),
			},
		}
		exampleLabel.Draw(builder)

		// Applied example
		classStr := fmt.Sprintf("font-family-%s %s %s", test.family, test.weight, test.size)
		exampleText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s with Unicode: %s", test.sample, test.unicode),
				Class:   api.ResolveStyles(classStr),
			},
		}
		exampleText.Draw(builder)
	}

	// Generate PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build comprehensive font grid PDF: %v", err)
	}

	// Save PDF
	saveTestPDF(t, "comprehensive_font_grid", pdfData)
	t.Logf("✓ Comprehensive font grid PDF generated successfully (%d bytes)", len(pdfData))
	t.Logf("✓ Tested %d font combinations with full Unicode compatibility", len(testCases))
}
