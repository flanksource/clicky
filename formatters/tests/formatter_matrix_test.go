package formatters

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	. "github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/text"

	"gopkg.in/yaml.v3"
)

// TestFormatterMatrix tests all formatters with a matrix of test cases
func TestFormatterMatrix(t *testing.T) {
	// Create test data with nested maps and various date formats
	testData := map[string]interface{}{
		"id":           "TEST-001",
		"name":         "Test Product",
		"price":        299.99,
		"active":       true,
		"created_at":   "2024-01-15T10:30:00Z", // RFC3339
		"updated_at":   "1705315800",           // Unix timestamp as string
		"processed_at": 1705315860,             // Unix timestamp as int
		"tags":         []string{"new", "featured"},
		"metadata": map[string]interface{}{
			"category": "electronics",
			"brand":    "TechCorp",
			"rating":   4.5,
		},
		"address": map[string]interface{}{
			"street": "123 Test St",
			"city":   "San Francisco",
			"location": map[string]interface{}{
				"latitude":  37.7749,
				"longitude": -122.4194,
			},
		},
	}

	// Create schema
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{Name: "id", Type: "string"},
			{Name: "name", Type: "string"},
			{Name: "price", Type: "float", Format: "currency"},
			{Name: "active", Type: "boolean"},
			{Name: "created_at", Type: "date", Format: "date"},
			{Name: "updated_at", Type: "date", Format: "date"},
			{Name: "processed_at", Type: "date", Format: "date"},
			{Name: "tags", Type: "array"},
			{
				Name:   "metadata",
				Type:   "map",
				Format: "map",
				Fields: []api.PrettyField{
					{Name: "category", Type: "string"},
					{Name: "brand", Type: "string"},
					{Name: "rating", Type: "float"},
				},
			},
			{
				Name:   "address",
				Type:   "map",
				Format: "map",
				Fields: []api.PrettyField{
					{Name: "street", Type: "string"},
					{Name: "city", Type: "string"},
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

	// Parse data
	parser := api.NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(testData, schema)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}

	// Test matrix of formatters
	testCases := []struct {
		name      string
		formatter func() (string, error)
		validate  func(t *testing.T, output string)
	}{
		{
			name: "PrettyFormatter",
			formatter: func() (string, error) {
				f := NewPrettyFormatter()
				return f.FormatPrettyData(prettyData)
			},
			validate: func(t *testing.T, output string) {
				// Basic field presence
				if !strings.Contains(output, "TEST-001") {
					t.Error("Should contain ID")
				}
				if !strings.Contains(output, "Test Product") {
					t.Error("Should contain name")
				}

				// Strip ANSI codes for content checks
				stripped := text.StripANSI(output)

				if !strings.Contains(stripped, "299.99") {
					t.Error("Should display price")
				}
				// Nested map formatting (field names are prettified)
				if !strings.Contains(stripped, "Category: electronics") {
					t.Error("Should display nested map fields")
				}
				if !strings.Contains(stripped, "Street: 123 Test St") {
					t.Error("Should display address fields")
				}
				if !strings.Contains(stripped, "Latitude: 37.77") {
					t.Error("Should display deeply nested fields")
				}
				// Date fields should be present (timezone-agnostic)
				if !strings.Contains(stripped, "Created At: 2024-01-15") {
					t.Error("Should display created_at field")
				}
			},
		},
		{
			name: "JSONFormatter",
			formatter: func() (string, error) {
				sf := &SchemaFormatter{Schema: schema, Parser: parser}
				return sf.FormatData(prettyData, FormatOptions{Format: "json"})
			},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Should produce valid JSON: %v", err)
					return
				}
				if result["id"] != "TEST-001" {
					t.Error("Should contain correct ID")
				}
				// Check nested structure
				if metadata, ok := result["metadata"].(map[string]interface{}); ok {
					if metadata["category"] != "electronics" {
						t.Error("Should preserve nested map structure")
					}
				} else {
					t.Error("Metadata should be a map")
				}
				// Check deeply nested structure
				if address, ok := result["address"].(map[string]interface{}); ok {
					// Location should be a nested map with proper structure
					if location, ok := address["location"].(map[string]interface{}); ok {
						if location["latitude"] == nil || !strings.Contains(location["latitude"].(string), "37.77") {
							t.Error("Should contain latitude value in nested location")
						}
						if location["longitude"] == nil || !strings.Contains(location["longitude"].(string), "-122.42") {
							t.Error("Should contain longitude value in nested location")
						}
					} else {
						t.Error("Location should be a nested map")
					}
				} else {
					t.Error("Address should be a map")
				}
			},
		},
		{
			name: "YAMLFormatter",
			formatter: func() (string, error) {
				sf := &SchemaFormatter{Schema: schema, Parser: parser}
				return sf.FormatData(prettyData, FormatOptions{Format: "yaml"})
			},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := yaml.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Should produce valid YAML: %v", err)
					return
				}
				if result["id"] != "TEST-001" {
					t.Error("Should contain correct ID")
				}
				// Check nested structure in YAML
				if metadata, ok := result["metadata"].(map[string]interface{}); ok {
					if metadata["category"] != "electronics" {
						t.Error("YAML should preserve nested map structure")
					}
				}
			},
		},

		// CSV formatter is skipped because it requires tabular data (arrays/tables),
		// not a single record. CSV is tested separately in TestAllFormatters with appropriate data.
	}

	// Run all tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := tc.formatter()
			if err != nil {
				t.Fatalf("Formatter failed: %v", err)
			}

			tc.validate(t, output)

			// Log output for debugging
			t.Logf("%s Output:\n%s\n", tc.name, output)
		})
	}
}

// TestDateFormatting specifically tests date parsing and formatting
func TestDateFormatting(t *testing.T) {
	testCases := []struct {
		name        string
		input       interface{}
		fieldType   string
		shouldParse bool
	}{
		{"RFC3339 string", "2024-01-15T10:30:00Z", "date", true},
		{"Unix timestamp string", "1705315800", "date", true},
		{"Unix timestamp int", 1705315800, "date", true},
		{"Unix timestamp float", 1705315800.0, "date", true},
		{"Date only", "2024-01-15", "date", true},
		{"DateTime", "2024-01-15 10:30:00", "date", true},
		{"Invalid date", "not-a-date", "date", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			field := api.PrettyField{
				Name:   "test_date",
				Type:   tc.fieldType,
				Format: "date",
			}

			fieldValue, err := field.Parse(tc.input)
			if err != nil && tc.shouldParse {
				t.Errorf("Should parse %v but got error: %v", tc.input, err)
				return
			}

			if tc.shouldParse {
				formatted := fieldValue.Text.String()
				// Check that it formats to a reasonable date format
				if !strings.Contains(formatted, "2024") {
					t.Errorf("Formatted date should contain year 2024, got: %s", formatted)
				}
				t.Logf("Input: %v -> Formatted: %s", tc.input, formatted)
			}
		})
	}
}

// TestNestedMaps specifically tests nested map handling
func TestNestedMaps(t *testing.T) {
	deeplyNestedData := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"value": "deeply nested",
					"date":  "1705315800",
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
									{Name: "date", Type: "date", Format: "date"},
								},
							},
						},
					},
					{Name: "sibling", Type: "string"},
				},
			},
		},
	}

	parser := api.NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(deeplyNestedData, schema)
	if err != nil {
		t.Fatalf("Failed to parse nested data: %v", err)
	}

	// Test pretty formatting
	formatter := NewPrettyFormatter()
	output, err := formatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	// Check proper nesting (strip ANSI codes for content checks)
	// Field names are prettified (Level1 instead of level1)
	stripped := text.StripANSI(output)
	if !strings.Contains(stripped, "Level1:") {
		t.Error("Should show level1 field")
	}
	if !strings.Contains(stripped, "deeply nested") {
		t.Error("Should show deeply nested value")
	}
	if !strings.Contains(stripped, "2024-01-15") {
		t.Error("Should format nested date")
	}
	if !strings.Contains(stripped, "Sibling: value") {
		t.Error("Should show sibling field")
	}

	t.Logf("Nested output:\n%s", output)
}
