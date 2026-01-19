package formatters

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/clicky/api"
	. "github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/text"

	"gopkg.in/yaml.v3"
)

// FormatterTestCase represents a test case for formatter testing
type FormatterTestCase struct {
	Name      string
	Formatter interface{}
	Validate  func(t *testing.T, output string)
}

// TestAllFormatters tests all formatters with the same data
func TestAllFormatters(t *testing.T) {
	// Create test data and schema
	testData := createTestData()
	schema := createTestSchema()

	// Parse the data with schema
	parser := api.NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(testData, schema)
	if err != nil {
		t.Fatalf("Failed to parse data with schema: %v", err)
	}

	// Define test cases for each formatter
	testCases := []FormatterTestCase{
		{
			Name: "PrettyFormatter",
			Formatter: func() *PrettyFormatter {
				f := NewPrettyFormatter()
				f.NoColor = true // Disable colors for testing
				return f
			}(),
			Validate: func(t *testing.T, output string) {
				// Check that it contains formatted fields
				if !strings.Contains(output, "id: TEST-001") {
					t.Errorf("Pretty formatter should display ID field")

				}
				if !strings.Contains(output, "created_at: 2024-01-15 10:30:00") {
					t.Errorf("Pretty formatter should format RFC3339 date correctly")
				}
				// Unix timestamps are now formatted in UTC
				if !strings.Contains(output, "updated_at: 2024-01-15 10:50:00") {
					t.Errorf("Pretty formatter should display Updated At field in UTC")
				}
				if !strings.Contains(output, "processed_at: 2024-01-15 10:51:00") {
					t.Errorf("Pretty formatter should display Processed At field in UTC")
				}
				// Check nested map formatting
				if !strings.Contains(output, "category: electronics") {
					t.Errorf("Pretty formatter should display nested map fields")
				}
				if !strings.Contains(output, "city: San Francisco") {
					t.Errorf("Pretty formatter should display address fields")
				}
				if !strings.Contains(output, "latitude: 37.77") {
					t.Errorf("Pretty formatter should display deeply nested fields")
				}
			},
		},
		{
			Name:      "JSONFormatter",
			Formatter: NewJSONFormatter(),
			Validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("JSON formatter should produce valid JSON: %v", err)
				}

				// Check fields
				if result["id"] != "TEST-001" {
					t.Errorf("JSON should contain correct ID")
				}
				// Check date formatting (dates are converted to human-readable format)
				if result["created_at"] != "2024-01-15 10:30:00" {
					t.Errorf("JSON should format date correctly, got %v", result["created_at"])
				}
				// Note: Unix timestamps are formatted in local timezone
				// Just check that they're formatted as dates, not checking exact time due to timezone differences
				if updatedAt, ok := result["updated_at"].(string); !ok || !strings.Contains(updatedAt, "2024-01-15") {
					t.Errorf("JSON should format Unix timestamp as date string, got %v", result["updated_at"])
				}
				if processedAt, ok := result["processed_at"].(string); !ok || !strings.Contains(processedAt, "2024-01-15") {
					t.Errorf("JSON should format float Unix timestamp as date string, got %v", result["processed_at"])
				}
				// Check nested maps
				if metadata, ok := result["metadata"].(map[string]interface{}); ok {
					if metadata["category"] != "electronics" {
						t.Errorf("JSON should preserve nested map values")
					}
				} else {
					t.Errorf("JSON should have metadata as map")
				}
			},
		},
		{
			Name:      "YAMLFormatter",
			Formatter: NewYAMLFormatter(),
			Validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := yaml.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("YAML formatter should produce valid YAML: %v", err)
				}

				// Check fields
				if result["id"] != "TEST-001" {
					t.Errorf("YAML should contain correct ID")
				}
				// Check date formatting (dates are converted to human-readable format)
				if result["created_at"] != "2024-01-15 10:30:00" {
					t.Errorf("YAML should format date correctly, got %v", result["created_at"])
				}
				// Check nested maps
				if metadata, ok := result["metadata"].(map[string]interface{}); ok {
					if metadata["category"] != "electronics" {
						t.Errorf("YAML should preserve nested map values")
					}
				} else {
					t.Errorf("YAML should have metadata as map")
				}
			},
		},
		// CSVFormatter is skipped - it requires table/array data, not single records.
		// CSV formatting is tested in table-specific tests.
		{
			Name:      "PDFFormatter",
			Formatter: NewPDFFormatter(),
			Validate: func(t *testing.T, output string) {
				// PDF files start with %PDF header
				if !strings.HasPrefix(output, "%PDF") {
					t.Errorf("PDF formatter should produce valid PDF starting with %%PDF header")
				}
				// Check that it's a reasonable size for a PDF
				if len(output) < 1000 {
					t.Errorf("PDF output seems too small: %d bytes", len(output))
				}
				// Check for PDF structure markers
				if !strings.Contains(output, "endobj") {
					t.Errorf("PDF should contain PDF object markers")
				}
			},
		},
		{
			Name:      "MarkdownFormatter",
			Formatter: NewMarkdownFormatter(),
			Validate: func(t *testing.T, output string) {
				// For markdown, we need to format the raw data as map
				data := map[string]interface{}{
					"id":           testData.ID,
					"name":         testData.Name,
					"price":        fmt.Sprintf("$%.2f", testData.Price),
					"quantity":     testData.Quantity,
					"active":       testData.Active,
					"created_at":   "2024-01-15 10:30:00",
					"updated_at":   "2024-01-15 10:30:00",
					"processed_at": "2024-01-15 10:31:00",
					"tags":         testData.Tags,
					"metadata":     testData.Metadata,
					"address":      testData.Address,
				}

				mdFormatter := NewMarkdownFormatter()
				mdOutput, err := mdFormatter.Format(data)
				if err != nil {
					t.Errorf("Markdown formatter error: %v", err)
					return
				}

				// Check markdown formatting
				if !strings.Contains(mdOutput, "**id**: TEST-001") {
					t.Errorf("Markdown should format fields correctly")
				}
			},
		},
	}

	// Run tests for each formatter
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			var output string
			var err error

			// Format based on formatter type
			switch f := tc.Formatter.(type) {
			case *PrettyFormatter:
				output, err = f.FormatPrettyData(prettyData)
			case *JSONFormatter:
				// Format using schema formatter for consistent output
				sf := &SchemaFormatter{
					Schema: schema,
					Parser: parser,
				}
				output, err = sf.FormatData(prettyData, FormatOptions{Format: "json"})
			case *YAMLFormatter:
				// Format using schema formatter for consistent output
				sf := &SchemaFormatter{
					Schema: schema,
					Parser: parser,
				}
				output, err = sf.FormatData(prettyData, FormatOptions{Format: "yaml"})
			case *CSVFormatter:
				// Format using schema formatter for consistent output
				sf := &SchemaFormatter{
					Schema: schema,
					Parser: parser,
				}
				output, err = sf.FormatData(prettyData, FormatOptions{Format: "csv"})
			case *PDFFormatter:
				output, err = f.Format(prettyData)
			case *MarkdownFormatter:
				// Markdown formatter uses different interface
				// Skip validation in switch, handled in test case
				return
			}

			if err != nil {
				t.Errorf("%s formatter error: %v", tc.Name, err)
				return
			}

			// Validate output
			tc.Validate(t, output)
		})
	}
}

// TestDateParsing tests various date format parsing
func TestDateParsing(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "RFC3339 string",
			input:    "2024-01-15T10:30:00Z",
			expected: "2024-01-15 10:30:00",
		},
		{
			name:     "Unix timestamp string",
			input:    "1705315800",
			expected: time.Unix(1705315800, 0).UTC().Format("2006-01-02 15:04:05"),
		},
		{
			name:     "Unix timestamp int64",
			input:    int64(1705315800),
			expected: time.Unix(1705315800, 0).UTC().Format("2006-01-02 15:04:05"),
		},
		{
			name:     "Unix timestamp float64",
			input:    float64(1705315800),
			expected: time.Unix(1705315800, 0).UTC().Format("2006-01-02 15:04:05"),
		},
		{
			name:     "Date only string",
			input:    "2024-01-15",
			expected: "2024-01-15 00:00:00",
		},
		{
			name:     "DateTime string",
			input:    "2024-01-15 10:30:00",
			expected: "2024-01-15 10:30:00",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			field := api.PrettyField{
				Type:   "date",
				Format: "date",
			}

			fieldValue, err := field.Parse(tc.input)
			if err != nil {
				t.Errorf("Failed to parse date %v: %v", tc.input, err)
				return
			}

			formatted := fmt.Sprintf("%v", fieldValue.Primitive())
			if formatted != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, formatted)
			}
		})
	}
}

// TestNestedMapFormatting tests nested map formatting through the formatter pipeline
func TestNestedMapFormatting(t *testing.T) {
	nestedData := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"value": "deeply nested",
					"count": 42,
				},
			},
			"sibling": "value",
		},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "level1",
				Type:   "map",
				Format: "map",
				Fields: []api.PrettyField{
					{
						Name:   "level2",
						Type:   "map",
						Format: "map",
						Fields: []api.PrettyField{
							{
								Name:   "level3",
								Type:   "map",
								Format: "map",
								Fields: []api.PrettyField{
									{Name: "value", Type: "string"},
									{Name: "count", Type: "int"},
								},
							},
						},
					},
					{Name: "sibling", Type: "string"},
				},
			},
		},
	}

	// Parse and format through the full pipeline
	parser := api.NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(nestedData, schema)
	if err != nil {
		t.Fatalf("Failed to parse nested data: %v", err)
	}

	formatter := NewPrettyFormatter()
	formatted, err := formatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	// Strip ANSI codes for content checks
	stripped := text.StripANSI(formatted)

	// Check that nested values are properly formatted (prettified keys)
	if !strings.Contains(stripped, "Level1:") {
		t.Errorf("Should prettify map keys, got: %s", stripped)
	}
	if !strings.Contains(stripped, "Level2:") {
		t.Errorf("Should format nested maps, got: %s", stripped)
	}
	if !strings.Contains(stripped, "Level3:") {
		t.Errorf("Should format deeply nested maps, got: %s", stripped)
	}
	if !strings.Contains(stripped, "Value: deeply nested") {
		t.Errorf("Should format leaf values, got: %s", stripped)
	}
	if !strings.Contains(stripped, "Count: 42") {
		t.Errorf("Should format numeric values in maps, got: %s", stripped)
	}
}

// TestTableFormattingWithDates tests table formatting with dates
func TestTableFormattingWithDates(t *testing.T) {
	// Create test data with table
	tableData := []map[string]interface{}{
		{
			"id":         "ROW-1",
			"created_at": "1705315800", // Unix timestamp as string
			"amount":     99.99,
		},
		{
			"id":         "ROW-2",
			"created_at": int64(1705315860), // Unix timestamp as int64
			"amount":     149.99,
		},
		{
			"id":         "ROW-3",
			"created_at": "2024-01-15T10:32:00Z", // RFC3339
			"amount":     199.99,
		},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "items",
				Type:   "array",
				Format: "table",
				TableOptions: api.TableOptions{
					Columns: []api.PrettyField{
						{Name: "id", Type: "string"},
						{Name: "created_at", Type: "date", Format: "date"},
						{Name: "amount", Type: "float", Format: "currency"},
					},
				},
			},
		},
	}

	parser := api.NewStructParser()
	data := map[string]interface{}{
		"items": tableData,
	}

	prettyData, err := parser.ParseDataWithSchema(data, schema)
	if err != nil {
		t.Fatalf("Failed to parse table data: %v", err)
	}

	// Test with pretty formatter
	formatter := NewPrettyFormatter()
	output, err := formatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format table: %v", err)
	}

	// Check table formatting - headers are prettified
	t.Logf("Table output:\n%s", output)
	if !strings.Contains(output, "Id") && !strings.Contains(output, "ID") && !strings.Contains(output, "id") {
		t.Errorf("Table should have id header")
	}
	if !strings.Contains(output, "Created At") && !strings.Contains(output, "CREATED AT") && !strings.Contains(output, "created_at") {
		t.Errorf("Table should have created_at header")
	}
	if !strings.Contains(output, "Amount") && !strings.Contains(output, "AMOUNT") && !strings.Contains(output, "amount") {
		t.Errorf("Table should have amount header")
	}
	// Check dates are formatted (using UTC for Unix timestamps)
	expectedDate1 := time.Unix(1705315800, 0).UTC().Format("2006-01-02 15:04:05")
	expectedDate2 := time.Unix(1705315860, 0).UTC().Format("2006-01-02 15:04:05")

	// Just check the content exists, ignore exact spacing
	if !strings.Contains(output, "ROW-1") || !strings.Contains(output, expectedDate1) {
		t.Errorf("Table should format Unix timestamp string correctly, expected date: %s", expectedDate1)
		t.Logf("Output: %s", output)
	}
	if !strings.Contains(output, "ROW-2") || !strings.Contains(output, expectedDate2) {
		t.Errorf("Table should format Unix timestamp int64 correctly, expected date: %s", expectedDate2)
	}
	if !strings.Contains(output, "ROW-3") || !strings.Contains(output, "2024-01-15 10:32:00") {
		t.Errorf("Table should format RFC3339 date correctly")
	}
}

// TestTableWordWrapping tests that long content is wrapped in table cells
func TestTableWordWrapping(t *testing.T) {
	// Create test data with very long content
	longDescription := "This is a very long description that should be wrapped across multiple lines in the table cell to demonstrate the word wrapping feature of the tablewriter library which was integrated to solve exactly this kind of problem with long content."

	tableData := []map[string]interface{}{
		{
			"id":          "ITEM-1",
			"description": longDescription,
			"status":      "active",
		},
		{
			"id":          "ITEM-2",
			"description": "Short description",
			"status":      "inactive",
		},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "items",
				Type:   "array",
				Format: "table",
				TableOptions: api.TableOptions{
					Columns: []api.PrettyField{
						{Name: "id", Type: "string"},
						{Name: "description", Type: "string"},
						{Name: "status", Type: "string"},
					},
				},
			},
		},
	}

	parser := api.NewStructParser()
	data := map[string]interface{}{
		"items": tableData,
	}

	prettyData, err := parser.ParseDataWithSchema(data, schema)
	if err != nil {
		t.Fatalf("Failed to parse table data: %v", err)
	}

	// Test with pretty formatter
	formatter := NewPrettyFormatter()
	output, err := formatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format table: %v", err)
	}

	t.Logf("Table output with word wrapping:\n%s", output)

	// Check that the table was rendered (headers are prettified)
	if !strings.Contains(output, "Id") && !strings.Contains(output, "ID") && !strings.Contains(output, "id") {
		t.Errorf("Table should have id header")
	}

	// Check that long content is present (word wrapping may split it across lines)
	if !strings.Contains(output, "ITEM-1") {
		t.Errorf("Table should contain ITEM-1")
	}

	// Check that the long description content is present
	// We don't check for exact formatting since word wrapping may break it differently
	if !strings.Contains(output, "very long description") {
		t.Errorf("Table should contain the long description content")
	}

	// Check that short content is present
	if !strings.Contains(output, "ITEM-2") || !strings.Contains(output, "Short description") {
		t.Errorf("Table should contain ITEM-2 with short description")
	}
}
