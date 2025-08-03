package clicky

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	"gopkg.in/yaml.v3"
)

// TestData represents a test data structure with various field types
type TestData struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Price       float64                `json:"price" yaml:"price"`
	Quantity    int                    `json:"quantity" yaml:"quantity"`
	Active      bool                   `json:"active" yaml:"active"`
	CreatedAt   string                 `json:"created_at" yaml:"created_at"`
	UpdatedAt   int64                  `json:"updated_at" yaml:"updated_at"`
	ProcessedAt float64                `json:"processed_at" yaml:"processed_at"`
	Tags        []string               `json:"tags" yaml:"tags"`
	Metadata    map[string]interface{} `json:"metadata" yaml:"metadata"`
	Address     map[string]interface{} `json:"address" yaml:"address"`
}

// createTestData creates test data with nested maps and various date formats
func createTestData() TestData {
	return TestData{
		ID:          "TEST-001",
		Name:        "Test Product",
		Price:       299.99,
		Quantity:    42,
		Active:      true,
		CreatedAt:   "2024-01-15T10:30:00Z",     // RFC3339 format
		UpdatedAt:   1705315800,                 // Unix timestamp (int64)
		ProcessedAt: 1705315860.5,               // Unix timestamp with milliseconds (float64)
		Tags:        []string{"new", "featured", "sale"},
		Metadata: map[string]interface{}{
			"category":    "electronics",
			"subcategory": "computers",
			"brand":       "TechCorp",
			"rating":      4.5,
			"stock":       100,
		},
		Address: map[string]interface{}{
			"street":  "123 Test St",
			"city":    "San Francisco",
			"state":   "CA",
			"zip":     "94105",
			"country": "USA",
			"location": map[string]interface{}{
				"latitude":  37.7749,
				"longitude": -122.4194,
			},
		},
	}
}

// createTestSchema creates a schema for the test data
func createTestSchema() *api.PrettyObject {
	return &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name: "id",
				Type: "string",
			},
			{
				Name: "name",
				Type: "string",
			},
			{
				Name:   "price",
				Type:   "float",
				Format: "currency",
			},
			{
				Name: "quantity",
				Type: "int",
			},
			{
				Name: "active",
				Type: "boolean",
			},
			{
				Name:       "created_at",
				Type:       "date",
				Format:     "date",
				DateFormat: "2006-01-02 15:04:05",
			},
			{
				Name:       "updated_at",
				Type:       "date",
				Format:     "date",
				DateFormat: "2006-01-02 15:04:05",
			},
			{
				Name:       "processed_at",
				Type:       "date",
				Format:     "date",
				DateFormat: "2006-01-02 15:04:05",
			},
			{
				Name:   "tags",
				Type:   "array",
				Format: "list",
			},
			{
				Name:   "metadata",
				Type:   "map",
				Format: "map",
				Fields: []api.PrettyField{
					{Name: "category", Type: "string"},
					{Name: "subcategory", Type: "string"},
					{Name: "brand", Type: "string"},
					{Name: "rating", Type: "float"},
					{Name: "stock", Type: "int"},
				},
			},
			{
				Name:   "address",
				Type:   "map",
				Format: "map",
				Fields: []api.PrettyField{
					{Name: "street", Type: "string"},
					{Name: "city", Type: "string"},
					{Name: "state", Type: "string"},
					{Name: "zip", Type: "string"},
					{Name: "country", Type: "string"},
					{
						Name:   "location",
						Type:   "map",
						Format: "map",
						Fields: []api.PrettyField{
							{Name: "latitude", Type: "float"},
							{Name: "longitude", Type: "float"},
						},
					},
				},
			},
		},
	}
}

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
	parser := NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(testData, schema)
	if err != nil {
		t.Fatalf("Failed to parse data with schema: %v", err)
	}
	
	// Define test cases for each formatter
	testCases := []FormatterTestCase{
		{
			Name:      "PrettyFormatter",
			Formatter: formatters.NewPrettyFormatter(),
			Validate: func(t *testing.T, output string) {
				// Check that it contains formatted fields
				if !strings.Contains(output, "Id: TEST-001") {
					t.Errorf("Pretty formatter should display ID field")
				}
				if !strings.Contains(output, "Price: $299.99") {
					t.Errorf("Pretty formatter should format currency correctly")
				}
				if !strings.Contains(output, "Created At: 2024-01-15 10:30:00") {
					t.Errorf("Pretty formatter should format RFC3339 date correctly")
				}
				// Note: Unix timestamps are formatted in local timezone
				if !strings.Contains(output, "Updated At: ") {
					t.Errorf("Pretty formatter should display Updated At field")
				}
				if !strings.Contains(output, "Processed At: ") {
					t.Errorf("Pretty formatter should display Processed At field")
				}
				// Check nested map formatting
				if !strings.Contains(output, "Category: electronics") {
					t.Errorf("Pretty formatter should display nested map fields")
				}
				if !strings.Contains(output, "City: San Francisco") {
					t.Errorf("Pretty formatter should display address fields")
				}
				if !strings.Contains(output, "Latitude: 37.7749") {
					t.Errorf("Pretty formatter should display deeply nested fields")
				}
			},
		},
		{
			Name:      "JSONFormatter",
			Formatter: formatters.NewJSONFormatter(),
			Validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("JSON formatter should produce valid JSON: %v", err)
				}
				
				// Check fields
				if result["id"] != "TEST-001" {
					t.Errorf("JSON should contain correct ID")
				}
				if result["price"] != "$299.99" {
					t.Errorf("JSON should format currency correctly, got %v", result["price"])
				}
				// Check date formatting
				if result["created_at"] != "2024-01-15 10:30:00" {
					t.Errorf("JSON should format RFC3339 date correctly, got %v", result["created_at"])
				}
				if result["updated_at"] != "2024-01-15 10:30:00" {
					t.Errorf("JSON should format Unix timestamp correctly, got %v", result["updated_at"])
				}
				if result["processed_at"] != "2024-01-15 10:31:00" {
					t.Errorf("JSON should format float Unix timestamp correctly, got %v", result["processed_at"])
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
			Formatter: formatters.NewYAMLFormatter(),
			Validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := yaml.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("YAML formatter should produce valid YAML: %v", err)
				}
				
				// Check fields
				if result["id"] != "TEST-001" {
					t.Errorf("YAML should contain correct ID")
				}
				if result["price"] != "$299.99" {
					t.Errorf("YAML should format currency correctly, got %v", result["price"])
				}
				// Check date formatting
				if result["created_at"] != "2024-01-15 10:30:00" {
					t.Errorf("YAML should format RFC3339 date correctly, got %v", result["created_at"])
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
		{
			Name:      "CSVFormatter",
			Formatter: formatters.NewCSVFormatter(),
			Validate: func(t *testing.T, output string) {
				lines := strings.Split(strings.TrimSpace(output), "\n")
				if len(lines) < 2 {
					t.Errorf("CSV should have header and data rows")
				}
				
				// Check header
				header := lines[0]
				if !strings.Contains(header, "id") {
					t.Errorf("CSV header should contain field names")
				}
				
				// Check data row
				if len(lines) > 1 {
					dataRow := lines[1]
					if !strings.Contains(dataRow, "TEST-001") {
						t.Errorf("CSV data should contain values")
					}
					if !strings.Contains(dataRow, "$299.99") {
						t.Errorf("CSV should format currency correctly")
					}
					if !strings.Contains(dataRow, "2024-01-15 10:30:00") {
						t.Errorf("CSV should format dates correctly")
					}
				}
			},
		},
		{
			Name:      "HTMLFormatter",
			Formatter: formatters.NewHTMLFormatter(),
			Validate: func(t *testing.T, output string) {
				// Check HTML structure
				if !strings.Contains(output, "<!DOCTYPE html>") {
					t.Errorf("HTML formatter should produce valid HTML document")
				}
				if !strings.Contains(output, "TEST-001") {
					t.Errorf("HTML should contain ID value")
				}
				if !strings.Contains(output, "$299.99") {
					t.Errorf("HTML should format currency correctly")
				}
				if !strings.Contains(output, "2024-01-15 10:30:00") {
					t.Errorf("HTML should format dates correctly")
				}
				// Check nested fields
				if !strings.Contains(output, "electronics") {
					t.Errorf("HTML should display nested map values")
				}
			},
		},
		{
			Name:      "MarkdownFormatter",
			Formatter: formatters.NewMarkdownFormatter(),
			Validate: func(t *testing.T, output string) {
				// For markdown, we need to format the raw data as map
				data := map[string]interface{}{
					"id":          testData.ID,
					"name":        testData.Name,
					"price":       fmt.Sprintf("$%.2f", testData.Price),
					"quantity":    testData.Quantity,
					"active":      testData.Active,
					"created_at":  "2024-01-15 10:30:00",
					"updated_at":  "2024-01-15 10:30:00",
					"processed_at": "2024-01-15 10:31:00",
					"tags":        testData.Tags,
					"metadata":    testData.Metadata,
					"address":     testData.Address,
				}
				
				mdFormatter := formatters.NewMarkdownFormatter()
				mdOutput, err := mdFormatter.Format(data)
				if err != nil {
					t.Errorf("Markdown formatter error: %v", err)
					return
				}
				
				// Check markdown formatting
				if !strings.Contains(mdOutput, "**id**: TEST-001") {
					t.Errorf("Markdown should format fields correctly")
				}
				if !strings.Contains(mdOutput, "**price**: $299.99") {
					t.Errorf("Markdown should display formatted values")
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
			case *formatters.PrettyFormatter:
				output, err = f.Format(prettyData)
			case *formatters.JSONFormatter:
				// Format using schema formatter for consistent output
				sf := &SchemaFormatter{
					Schema: schema,
					Parser: parser,
				}
				output, err = sf.formatJSONWithPrettyData(prettyData)
			case *formatters.YAMLFormatter:
				// Format using schema formatter for consistent output
				sf := &SchemaFormatter{
					Schema: schema,
					Parser: parser,
				}
				output, err = sf.formatYAMLWithPrettyData(prettyData)
			case *formatters.CSVFormatter:
				// Format using schema formatter for consistent output
				sf := &SchemaFormatter{
					Schema: schema,
					Parser: parser,
				}
				output, err = sf.formatCSVWithPrettyData(prettyData)
			case *formatters.HTMLFormatter:
				output, err = f.Format(prettyData)
			case *formatters.MarkdownFormatter:
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
			expected: "2024-01-15 10:30:00",
		},
		{
			name:     "Unix timestamp int64",
			input:    int64(1705315800),
			expected: "2024-01-15 10:30:00",
		},
		{
			name:     "Unix timestamp float64",
			input:    float64(1705315800),
			expected: "2024-01-15 10:30:00",
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
			
			formatted := fieldValue.Formatted()
			if formatted != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, formatted)
			}
		})
	}
}

// TestNestedMapFormatting tests nested map formatting
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
	
	field := api.PrettyField{
		Name:   "nested",
		Type:   "map",
		Format: "map",
	}
	
	// Test formatting
	formatted := field.FormatMapValue(nestedData)
	
	// Check that nested values are properly formatted
	if !strings.Contains(formatted, "Level1:") {
		t.Errorf("Should prettify map keys")
	}
	if !strings.Contains(formatted, "Level2:") {
		t.Errorf("Should format nested maps")
	}
	if !strings.Contains(formatted, "Level3:") {
		t.Errorf("Should format deeply nested maps")
	}
	if !strings.Contains(formatted, "Value: deeply nested") {
		t.Errorf("Should format leaf values")
	}
	if !strings.Contains(formatted, "Count: 42") {
		t.Errorf("Should format numeric values in maps")
	}
	
	// Check indentation
	lines := strings.Split(formatted, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Level2:") {
			if !strings.HasPrefix(line, "\t") {
				t.Errorf("Nested fields should be indented with tabs")
			}
		}
		if strings.Contains(line, "Level3:") {
			if !strings.HasPrefix(line, "\t\t") {
				t.Errorf("Deeply nested fields should have multiple tabs")
			}
		}
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
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						{Name: "id", Type: "string"},
						{Name: "created_at", Type: "date", Format: "date"},
						{Name: "amount", Type: "float", Format: "currency"},
					},
				},
			},
		},
	}
	
	parser := NewStructParser()
	data := map[string]interface{}{
		"items": tableData,
	}
	
	prettyData, err := parser.ParseDataWithSchema(data, schema)
	if err != nil {
		t.Fatalf("Failed to parse table data: %v", err)
	}
	
	// Test with pretty formatter
	formatter := formatters.NewPrettyFormatter()
	output, err := formatter.Format(prettyData)
	if err != nil {
		t.Fatalf("Failed to format table: %v", err)
	}
	
	// Check table formatting
	if !strings.Contains(output, "│ id    │ created_at          │ amount   │") {
		t.Errorf("Table should have headers")
	}
	if !strings.Contains(output, "│ ROW-1 │ 2024-01-15 10:30:00 │ $99.99   │") {
		t.Errorf("Table should format Unix timestamp string correctly")
	}
	if !strings.Contains(output, "│ ROW-2 │ 2024-01-15 10:31:00 │ $149.99  │") {
		t.Errorf("Table should format Unix timestamp int64 correctly")
	}
	if !strings.Contains(output, "│ ROW-3 │ 2024-01-15 10:32:00 │ $199.99  │") {
		t.Errorf("Table should format RFC3339 date correctly")
	}
}