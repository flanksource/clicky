package formatters

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	. "github.com/flanksource/clicky/formatters"
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
		CreatedAt:   "2024-01-15T10:30:00Z", // RFC3339 format
		UpdatedAt:   1705315800,             // Unix timestamp (int64)
		ProcessedAt: 1705315860.5,           // Unix timestamp with milliseconds (float64)
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

// TestHTMLFormatter_FormatWithSchema tests HTML formatter with schema
func TestHTMLFormatter_FormatWithSchema(t *testing.T) {
	// Create test data and schema
	testData := createTestData()
	schema := createTestSchema()

	// Parse the data with schema
	parser := api.NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(testData, schema)
	if err != nil {
		t.Fatalf("Failed to parse data with schema: %v", err)
	}

	// Test HTML formatter
	formatter := NewHTMLFormatter()
	formatter.IncludeCSS = false // Simplify output for testing
	output, err := formatter.Format(prettyData, formatters.FormatOptions{})
	if err != nil {
		t.Fatalf("HTMLFormatter.Format failed: %v", err)
	}

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

	t.Logf("HTML output:\n%s", output)
}

// TestHTMLFormatter_WithMapFields tests HTML formatter with nested maps
func TestHTMLFormatter_WithMapFields(t *testing.T) {
	// Create test data with nested maps
	testData := map[string]interface{}{
		"name": "John Doe",
		"age":  30,
		"address": map[string]interface{}{
			"street":  "123 Main St",
			"city":    "New York",
			"country": "USA",
		},
		"metadata": map[string]interface{}{
			"created_at": "2023-01-01",
			"source":     "api",
		},
	}

	// Create schema that includes map fields
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{Name: "name", Type: "string"},
			{Name: "age", Type: "int"},
			{Name: "address", Type: "map"},
			{Name: "metadata", Type: "map"},
		},
	}

	parser := api.NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(testData, schema)
	if err != nil {
		t.Fatalf("ParseDataWithSchema failed: %v", err)
	}

	formatter := NewHTMLFormatter()
	formatter.IncludeCSS = false // Simplify output for testing
	output, err := formatter.Format(prettyData, formatters.FormatOptions{})
	if err != nil {
		t.Fatalf("HTMLFormatter.Format failed: %v", err)
	}

	// Check that output contains map field content
	if !strings.Contains(output, "Address") {
		t.Error("HTML output doesn't contain address field")
	}
	if !strings.Contains(output, "Metadata") {
		t.Error("HTML output doesn't contain metadata field")
	}
	if !strings.Contains(output, "123 Main St") {
		t.Error("HTML output doesn't contain address content")
	}

	t.Logf("HTML output:\n%s", output)
}

// TestHTMLTableColumnLabels tests that HTML formatter uses Label field for headers
func TestHTMLTableColumnLabels(t *testing.T) {
	// Create test data
	tableData := []map[string]interface{}{
		{
			"id":    "PROD-001",
			"name":  "Test Product",
			"price": 99.99,
			"qty":   10,
		},
		{
			"id":    "PROD-002",
			"name":  "Another Product",
			"price": 149.99,
			"qty":   5,
		},
	}

	// Create schema with Labels defined
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "items",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.TableOptions{
					Columns: []api.PrettyField{
						{Name: "id", Label: "Product ID", Type: "string"},
						{Name: "name", Label: "Product Name", Type: "string"},
						{Name: "price", Label: "Unit Price ($)", Type: "float", Format: "currency"},
						{Name: "qty", Label: "Quantity", Type: "int"},
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

	// Test HTML formatter in PDF mode for static table headers
	htmlFormatter := NewHTMLFormatter()
	htmlFormatter.IsPDFMode = true // Use static HTML tables for testing headers
	htmlOutput, err := htmlFormatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format HTML: %v", err)
	}

	t.Logf("HTML output length: %d characters", len(htmlOutput))

	// Check header contains Labels, not Names
	if !strings.Contains(htmlOutput, "Product ID") {
		t.Errorf("HTML should contain 'Product ID' label")
	}
	if !strings.Contains(htmlOutput, "Product Name") {
		t.Errorf("HTML should contain 'Product Name' label")
	}
	if !strings.Contains(htmlOutput, "Unit Price ($)") {
		t.Errorf("HTML should contain 'Unit Price ($)' label")
	}
	if !strings.Contains(htmlOutput, "Quantity") {
		t.Errorf("HTML should contain 'Quantity' label")
	}

	// Check that raw field names are NOT used in headers
	// They should only appear in data cells, not in <th> tags
	if strings.Contains(htmlOutput, ">id<") {
		t.Errorf("HTML should not contain raw 'id' field name in header")
	}
	if strings.Contains(htmlOutput, ">name<") {
		t.Errorf("HTML should not contain raw 'name' field name in header")
	}
	if strings.Contains(htmlOutput, ">price<") {
		t.Errorf("HTML should not contain raw 'price' field name in header")
	}
	if strings.Contains(htmlOutput, ">qty<") {
		t.Errorf("HTML should not contain raw 'qty' field name in header")
	}

	// Verify data is still present
	if !strings.Contains(htmlOutput, "PROD-001") {
		t.Errorf("HTML should contain data 'PROD-001'")
	}
	if !strings.Contains(htmlOutput, "Test Product") {
		t.Errorf("HTML should contain data 'Test Product'")
	}
}

// TestHTMLTableMixedLabels tests HTML with some fields having Labels and others not
func TestHTMLTableMixedLabels(t *testing.T) {
	tableData := []map[string]interface{}{
		{
			"id":     "ITEM-001",
			"name":   "Test Item",
			"status": "active",
		},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "data",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.TableOptions{
					Columns: []api.PrettyField{
						{Name: "id", Label: "Item ID", Type: "string"},            // Has Label
						{Name: "name", Type: "string"},                            // No Label - should use Name
						{Name: "status", Label: "Current Status", Type: "string"}, // Has Label
					},
				},
			},
		},
	}

	parser := api.NewStructParser()
	data := map[string]interface{}{
		"data": tableData,
	}

	prettyData, err := parser.ParseDataWithSchema(data, schema)
	if err != nil {
		t.Fatalf("Failed to parse table data: %v", err)
	}

	// Test with PDF mode to check static HTML table headers
	htmlFormatter := NewHTMLFormatter()
	htmlFormatter.IsPDFMode = true // Use static HTML tables for testing headers
	htmlOutput, err := htmlFormatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format HTML: %v", err)
	}

	// Should contain Labels where defined
	if !strings.Contains(htmlOutput, "Item ID") {
		t.Errorf("HTML should contain 'Item ID' label")
	}
	if !strings.Contains(htmlOutput, "Current Status") {
		t.Errorf("HTML should contain 'Current Status' label")
	}

	// Should use prettified Name where Label is not defined
	if !strings.Contains(htmlOutput, ">Name<") {
		t.Errorf("HTML should contain 'Name' prettified field name as header (no label defined)")
	}
}
