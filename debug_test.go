package clicky

import (
	"fmt"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

func TestDebugFormatting(t *testing.T) {
	// Create simple test data
	testData := TestData{
		ID:          "TEST-001",
		Name:        "Test Product",
		Price:       299.99,
		CreatedAt:   "2024-01-15T10:30:00Z",     // RFC3339 format
		UpdatedAt:   1705315800,                 // Unix timestamp (int64)
		ProcessedAt: 1705315860.5,               // Unix timestamp with milliseconds (float64)
	}
	
	// Create schema
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{Name: "id", Type: "string"},
			{Name: "name", Type: "string"},
			{Name: "price", Type: "float", Format: "currency"},
			{Name: "created_at", Type: "date", Format: "date", DateFormat: "2006-01-02 15:04:05"},
			{Name: "updated_at", Type: "date", Format: "date", DateFormat: "2006-01-02 15:04:05"},
			{Name: "processed_at", Type: "date", Format: "date", DateFormat: "2006-01-02 15:04:05"},
		},
	}
	
	// Parse data
	parser := NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(testData, schema)
	if err != nil {
		t.Fatalf("Failed to parse data: %v", err)
	}
	
	// Format with pretty formatter
	formatter := formatters.NewPrettyFormatter()
	output, err := formatter.Format(prettyData)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}
	
	fmt.Printf("Pretty Output:\n%s\n", output)
	
	// Test JSON formatter
	sf := &SchemaFormatter{
		Schema: schema,
		Parser: parser,
	}
	jsonOutput, err := sf.formatJSONWithPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format JSON: %v", err)
	}
	
	fmt.Printf("JSON Output:\n%s\n", jsonOutput)
}