package pdf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	. "github.com/flanksource/clicky/formatters/pdf"
	"github.com/flanksource/maroto/v2/pkg/components/col"
)

// TestUnicodeSupport specifically tests Unicode character support in PDF generation and extraction
func XTestUnicodeSupport(t *testing.T) {
	// Create a builder
	builder := NewBuilder()

	// Add a table with various Unicode characters
	tableComponent := NewTableComponent(
		[]string{"Symbol", "Description", "Unicode", "Usage"},
		[][]any{
			{"²", "Squared", "U+00B2", "60cm²"},
			{"³", "Cubed", "U+00B3", "8m³"},
			{"°", "Degree", "U+00B0", "25°C"},
			{"±", "Plus-minus", "U+00B1", "±0.1"},
			{"µ", "Micro", "U+00B5", "µg/ml"},
			{"Ω", "Ohm", "U+03A9", "10Ω"},
			{"π", "Pi", "U+03C0", "π ≈ 3.14"},
			{"√", "Square root", "U+221A", "√16 = 4"},
			{"∞", "Infinity", "U+221E", "lim → ∞"},
			{"≤", "Less or equal", "U+2264", "x ≤ 100"},
		},
	)

	// Add the table using maroto's column system (full width)
	builder.GetMaroto().AddRow(100.0, // Sufficient height for the table
		col.New(12).Add(tableComponent))

	// Generate PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build PDF: %v", err)
	}

	// Extract text from the PDF
	extractedText, err := ExtractTextFromPDF(pdfData)
	if err != nil {
		t.Fatalf("Failed to extract text from PDF: %v", err)
	}

	t.Logf("Extracted text length: %d characters", len(extractedText))

	// Test Unicode characters (expecting raw extracted text with possible mojibake)
	unicodeTestCases := []struct {
		symbol          string
		description     string
		example         string
		mojibakeSymbol  string // Expected mojibake encoding
		mojibakeExample string // Expected mojibake example
	}{
		{"²", "squared symbol", "60cm²", "Â²", "60cmÂ²"},
		{"³", "cubed symbol", "8m³", "Â³", "8mÂ³"},
		{"°", "degree symbol", "25°C", "Â°", "25Â°C"},
		{"±", "plus-minus symbol", "±0.1", "Â±", "Â±0.1"},
		{"µ", "micro symbol", "µg/ml", "Âµ", "Âµg/ml"},
		{"Ω", "omega symbol", "10Ω", "Î©", "10Î©"},
		{"π", "pi symbol", "π ≈ 3.14", "Ï€", "Ï€ â‰ˆ 3.14"},
		{"√", "square root symbol", "√16 = 4", "âˆš", "âˆš16 = 4"},
		{"∞", "infinity symbol", "lim → ∞", "âˆž", "lim â†' âˆž"},
		{"≤", "less-or-equal symbol", "x ≤ 100", "â‰¤", "x â‰¤ 100"},
	}

	foundSymbols := 0
	foundExamples := 0
	missingSymbols := []string{}
	missingExamples := []string{}
	mojibakeSymbols := []string{}
	mojibakeExamples := []string{}

	for _, testCase := range unicodeTestCases {
		// Test for the symbol itself (either correct Unicode or mojibake)
		symbolFound := strings.Contains(extractedText, testCase.symbol) || strings.Contains(extractedText, testCase.mojibakeSymbol)
		if symbolFound {
			if strings.Contains(extractedText, testCase.mojibakeSymbol) && !strings.Contains(extractedText, testCase.symbol) {
				mojibakeSymbols = append(mojibakeSymbols, fmt.Sprintf("%s -> %s", testCase.symbol, testCase.mojibakeSymbol))
			}
			foundSymbols++
		} else {
			missingSymbols = append(missingSymbols, testCase.symbol)
		}

		// Test for the example usage (either correct Unicode or mojibake)
		exampleFound := strings.Contains(extractedText, testCase.example) || strings.Contains(extractedText, testCase.mojibakeExample)
		if exampleFound {
			if strings.Contains(extractedText, testCase.mojibakeExample) && !strings.Contains(extractedText, testCase.example) {
				mojibakeExamples = append(mojibakeExamples, fmt.Sprintf("%s -> %s", testCase.example, testCase.mojibakeExample))
			}
			foundExamples++
		} else {
			missingExamples = append(missingExamples, testCase.example)
		}
	}

	// Calculate coverage
	symbolCoverage := float64(foundSymbols) / float64(len(unicodeTestCases)) * 100
	exampleCoverage := float64(foundExamples) / float64(len(unicodeTestCases)) * 100

	// Print summaries only
	if len(mojibakeSymbols) > 0 {
		t.Logf("Mojibake symbols found: %v", mojibakeSymbols)
	}
	if len(missingSymbols) > 0 {
		t.Logf("Unicode symbols missing: %v", missingSymbols)
	}

	// Check for mojibake symbols (fail if any found)
	if len(mojibakeSymbols) > 0 {
		t.Errorf("Mojibake symbols detected - Unicode encoding is not working correctly: %v", mojibakeSymbols)
	}

	// Check for high coverage (at least 80% of symbols should be visible)
	if symbolCoverage < 80.0 {
		t.Errorf("Low Unicode symbol coverage (%.1f%%) - Unicode support may be insufficient", symbolCoverage)
	}

	if exampleCoverage < 80.0 {
		t.Errorf("Low Unicode example coverage (%.1f%%) - text rendering may have issues", exampleCoverage)
	}
}

// TestGenerateUnicodePDF generates a focused Unicode PDF for debugging text rendering
func TestGenerateUnicodePDF(t *testing.T) {
	// Create a builder with Unicode font support
	builder := NewBuilder()

	// Add title section
	titleText := Text{
		Text: api.Text{
			Content: "Unicode Character Debug Test",
			Class:   api.ResolveStyles("text-2xl font-bold text-center text-blue-800 mb-4"),
		},
	}
	titleText.Draw(builder)

	// Add explanation
	explanationText := Text{
		Text: api.Text{
			Content: "Focused test with single Unicode character to debug rendering pipeline.",
			Class:   api.ResolveStyles("text-sm text-gray-700 text-center mb-6"),
		},
	}
	explanationText.Draw(builder)

	// Test Unicode character details
	testChar := "²"
	testCharBytes := []byte(testChar)
	t.Logf("🔍 Test character: '%s'", testChar)
	t.Logf("🔍 Character bytes: %v", testCharBytes)
	t.Logf("🔍 Character hex: %x", testCharBytes)
	t.Logf("🔍 Character length: %d", len(testChar))

	// Single row test data focusing on problematic Unicode
	testData := [][]any{
		{testChar, "Superscript 2", "U+00B2", "60cm" + testChar},
	}

	// TableComponent version with debug logging
	tableComponentHeaderText := Text{
		Text: api.Text{
			Content: "TableComponent Version:",
			Class:   api.ResolveStyles("text-sm font-medium text-gray-700 mb-2"),
		},
	}
	tableComponentHeaderText.Draw(builder)

	t.Logf("📋 Creating TableComponent with data: %+v", testData)
	tableComponentVersion := NewTableComponent(
		[]string{"Symbol", "Name", "Unicode", "Example"},
		testData,
	)

	// Log what TableComponent will render
	t.Logf("📋 TableComponent headers: %v", []string{"Symbol", "Name", "Unicode", "Example"})
	t.Logf("📋 TableComponent first row: %v", testData[0])

	builder.GetMaroto().AddRow(20.0, col.New(12).Add(tableComponentVersion))

	// Table version with debug logging
	tableHeaderText := Text{
		Text: api.Text{
			Content: "Table Version:",
			Class:   api.ResolveStyles("text-sm font-medium text-gray-700 mt-4 mb-2"),
		},
	}
	tableHeaderText.Draw(builder)

	// Convert data to [][]any for Table
	convertedData := make([][]any, len(testData))
	for i, row := range testData {
		convertedData[i] = make([]any, len(row))
		for j, cell := range row {
			convertedData[i][j] = cell
			t.Logf("🔄 Converting cell [%d][%d]: '%s' -> '%v'", i, j, cell, convertedData[i][j])
		}
	}

	// Create columns for Table
	tableColumns := []Column{
		{Label: "Symbol", Style: "w-[25%] text-sm text-gray-800 text-center align-middle"},
		{Label: "Name", Style: "w-[25%] text-sm text-gray-800 text-left align-middle"},
		{Label: "Unicode", Style: "w-[25%] text-sm text-gray-800 text-center align-middle"},
		{Label: "Example", Style: "w-[25%] text-sm text-gray-800 text-left align-middle"},
	}

	t.Logf("📊 Table columns: %+v", tableColumns)
	t.Logf("📊 Table converted data: %+v", convertedData)

	tableVersion := Table{
		BaseTable: BaseTable{
			Columns:           tableColumns,
			Rows:              convertedData,
			HeaderStyle:       "font-bold text-white bg-blue-600 text-center text-sm",
			RowStyle:          "text-sm text-gray-800 align-middle",
			AlternateRowStyle: "bg-gray-50",
			ShowBorders:       true,
		},
	}

	t.Logf("📊 Drawing Table with HeaderStyle: %s", tableVersion.HeaderStyle)
	t.Logf("📊 Drawing Table with RowStyle: %s", tableVersion.RowStyle)

	tableVersion.Draw(builder)

	// Generate the PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to generate Unicode PDF: %v", err)
	}

	// Validate PDF was generated
	if len(pdfData) == 0 {
		t.Fatal("Generated PDF is empty")
	}

	// Extract text and check for mojibake
	extractedText, err := ExtractTextFromPDF(pdfData)
	if err != nil {
		t.Fatalf("Failed to extract text from PDF: %v", err)
	}

	// Check for mojibake symbol
	if strings.Contains(extractedText, "Â²") {
		t.Errorf("Mojibake detected: found 'Â²' instead of '²' in extracted text")
	}

	// Save the PDF
	saveUnicodePDF(t, "unicode_test", pdfData)

	t.Logf("✓ Unicode PDF generated successfully (%d bytes)", len(pdfData))
}

// TestMultiFontUnicodeSupport tests Unicode character support across multiple font families
func XTestMultiFontUnicodeSupport(t *testing.T) {
	builder := NewBuilder()

	// Add title section
	titleText := Text{
		Text: api.Text{
			Content: "Multi-Font Unicode Support Test",
			Class:   api.ResolveStyles("text-2xl font-bold text-center text-blue-800 mb-4"),
		},
	}
	titleText.Draw(builder)

	// Font families to test (using available PDF fonts)
	fontFamilies := []string{"Arial", "Times", "Courier", "Helvetica"}

	// Unicode test characters with their expected mojibake variants
	unicodeTestCases := []struct {
		symbol          string
		description     string
		example         string
		mojibakeSymbol  string
		mojibakeExample string
	}{
		{"²", "squared symbol", "60cm²", "Â²", "60cmÂ²"},
		{"³", "cubed symbol", "8m³", "Â³", "8mÂ³"},
		{"°", "degree symbol", "25°C", "Â°", "25Â°C"},
		{"±", "plus-minus symbol", "±0.1", "Â±", "Â±0.1"},
		{"µ", "micro symbol", "µg/ml", "Âµ", "Âµg/ml"},
		{"Ω", "omega symbol", "10Ω", "Î©", "10Î©"},
		{"π", "pi symbol", "π ≈ 3.14", "Ï€", "Ï€ â‰ˆ 3.14"},
		{"√", "square root symbol", "√16 = 4", "âˆš", "âˆš16 = 4"},
		{"∞", "infinity symbol", "lim → ∞", "âˆž", "lim â†' âˆž"},
		{"≤", "less-or-equal symbol", "x ≤ 100", "â‰¤", "x â‰¤ 100"},
	}

	// Test each font family
	for _, fontFamily := range fontFamilies {
		// Font family section header
		fontHeaderText := Text{
			Text: api.Text{
				Content: fmt.Sprintf("%s Font Family Unicode Test", fontFamily),
				Class:   api.ResolveStyles(fmt.Sprintf("font-family-%s text-lg font-bold mt-4 mb-2", fontFamily)),
			},
		}
		fontHeaderText.Draw(builder)

		// Create table data for this font family
		var tableData [][]any
		for _, testCase := range unicodeTestCases {
			tableData = append(tableData, []any{
				testCase.symbol,
				testCase.description,
				testCase.example,
				"Unicode: " + testCase.example,
			})
		}

		// Create table with font family applied
		fontTable := NewTableComponent(
			[]string{"Symbol", "Description", "Example", "Test"},
			tableData,
		)

		// Add table using maroto's column system
		builder.GetMaroto().AddRow(60.0, col.New(12).Add(fontTable))

		// Add spacing between font families
		spacerText := Text{
			Text: api.Text{
				Content: " ",
				Class:   api.ResolveStyles("text-xs"),
			},
		}
		spacerText.Draw(builder)
	}

	// Generate the PDF
	pdfData, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to generate multi-font Unicode PDF: %v", err)
	}

	// Validate PDF was generated
	if len(pdfData) == 0 {
		t.Fatal("Generated multi-font Unicode PDF is empty")
	}

	// Extract text and analyze for each font family
	extractedText, err := ExtractTextFromPDF(pdfData)
	if err != nil {
		t.Fatalf("Failed to extract text from multi-font PDF: %v", err)
	}

	// Track Unicode support across font families
	fontResults := make(map[string]map[string]bool)
	for _, fontFamily := range fontFamilies {
		fontResults[fontFamily] = make(map[string]bool)
	}

	// Test Unicode characters
	foundSymbols := 0
	totalSymbols := len(unicodeTestCases) * len(fontFamilies)
	mojibakeCount := 0

	for _, testCase := range unicodeTestCases {
		// Test for the symbol itself (either correct Unicode or mojibake)
		symbolFound := strings.Contains(extractedText, testCase.symbol)
		mojibakeFound := strings.Contains(extractedText, testCase.mojibakeSymbol)

		if symbolFound {
			foundSymbols++
		}
		if mojibakeFound && !symbolFound {
			mojibakeCount++
			t.Logf("Mojibake detected for '%s': found '%s' instead", testCase.symbol, testCase.mojibakeSymbol)
		}
	}

	// Calculate overall coverage
	symbolCoverage := float64(foundSymbols) / float64(totalSymbols) * 100

	// Log results
	t.Logf("Multi-font Unicode test results:")
	t.Logf("  Total symbols tested: %d", totalSymbols)
	t.Logf("  Symbols found correctly: %d", foundSymbols)
	t.Logf("  Mojibake instances: %d", mojibakeCount)
	t.Logf("  Unicode coverage: %.1f%%", symbolCoverage)

	// Check for mojibake symbols (fail if any found)
	if mojibakeCount > 0 {
		t.Errorf("Mojibake symbols detected in %d cases - Unicode encoding issues across font families", mojibakeCount)
	}

	// Check for reasonable coverage (at least 60% across all fonts)
	if symbolCoverage < 60.0 {
		t.Errorf("Low Unicode symbol coverage (%.1f%%) across font families - Unicode support may be insufficient", symbolCoverage)
	}

	// Save the PDF
	saveUnicodePDF(t, "multi_font_unicode_test", pdfData)
	t.Logf("✓ Multi-font Unicode PDF generated successfully (%d bytes)", len(pdfData))
}

// TestFontFamilyUnicodeCompatibility tests specific font families for Unicode compatibility
func XTestFontFamilyUnicodeCompatibility(t *testing.T) {
	// Test individual font families for Unicode support (using available PDF fonts)
	fontFamilies := []string{"Arial", "Times", "Courier", "Helvetica"}
	unicodeChars := "²³°±µΩπ√∞≤"

	for _, fontFamily := range fontFamilies {
		t.Run(fmt.Sprintf("Font_%s", fontFamily), func(t *testing.T) {
			builder := NewBuilder()

			// Create simple test with specific font
			testText := Text{
				Text: api.Text{
					Content: fmt.Sprintf("%s Font Unicode Test: %s", fontFamily, unicodeChars),
					Class:   api.ResolveStyles(fmt.Sprintf("font-family-%s text-lg", fontFamily)),
				},
			}
			testText.Draw(builder)

			// Generate PDF
			pdfData, err := builder.Build()
			if err != nil {
				t.Fatalf("Failed to build %s font PDF: %v", fontFamily, err)
			}

			// Extract and check text
			extractedText, err := ExtractTextFromPDF(pdfData)
			if err != nil {
				t.Fatalf("Failed to extract text from %s font PDF: %v", fontFamily, err)
			}

			// Check for font family name and Unicode characters
			if !strings.Contains(extractedText, fontFamily) {
				t.Errorf("Font family name '%s' not found in extracted text", fontFamily)
			}

			// Count successfully rendered Unicode characters
			unicodeFound := 0
			for _, char := range unicodeChars {
				if strings.Contains(extractedText, string(char)) {
					unicodeFound++
				}
			}

			unicodeCoverage := float64(unicodeFound) / float64(len([]rune(unicodeChars))) * 100
			t.Logf("%s font Unicode coverage: %.1f%% (%d/%d characters)",
				fontFamily, unicodeCoverage, unicodeFound, len([]rune(unicodeChars)))

			// Expect at least 70% Unicode coverage for each font
			if unicodeCoverage < 70.0 {
				t.Errorf("%s font has low Unicode coverage (%.1f%%)", fontFamily, unicodeCoverage)
			}
		})
	}
}

// saveUnicodePDF saves the Unicode test PDF to the out directory
func saveUnicodePDF(t *testing.T, name string, pdfData []byte) {
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
		t.Logf("Unicode PDF saved to: %s", filepath)
	}
}
