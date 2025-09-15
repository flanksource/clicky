package formatters

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
)

// TestCSVTableColumnLabels tests that CSV formatter uses Label field for headers
func TestCSVTableColumnLabels(t *testing.T) {
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
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
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

	// Test CSV formatter
	csvFormatter := NewCSVFormatter()
	csvOutput, err := csvFormatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format CSV: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(csvOutput), "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected at least header + 1 data row, got %d lines", len(lines))
	}

	// Check header line uses Labels
	headerLine := lines[0]
	t.Logf("CSV header: %s", headerLine)

	// Should contain the Label values, not the Name values
	if !strings.Contains(headerLine, "Product ID") {
		t.Errorf("Header should contain 'Product ID' label, got: %s", headerLine)
	}
	if !strings.Contains(headerLine, "Product Name") {
		t.Errorf("Header should contain 'Product Name' label, got: %s", headerLine)
	}
	if !strings.Contains(headerLine, "Unit Price ($)") {
		t.Errorf("Header should contain 'Unit Price ($)' label, got: %s", headerLine)
	}
	if !strings.Contains(headerLine, "Quantity") {
		t.Errorf("Header should contain 'Quantity' label, got: %s", headerLine)
	}

	// Should NOT contain the raw field names
	if strings.Contains(headerLine, "\"id\"") || strings.Contains(headerLine, ",id,") {
		t.Errorf("Header should not contain raw field name 'id', got: %s", headerLine)
	}
	if strings.Contains(headerLine, "\"qty\"") || strings.Contains(headerLine, ",qty,") {
		t.Errorf("Header should not contain raw field name 'qty', got: %s", headerLine)
	}

	// Check data is still extracted correctly using field Names
	dataLine := lines[1]
	t.Logf("CSV data: %s", dataLine)
	if !strings.Contains(dataLine, "PROD-001") {
		t.Errorf("Data should contain 'PROD-001', got: %s", dataLine)
	}
	if !strings.Contains(dataLine, "Test Product") {
		t.Errorf("Data should contain 'Test Product', got: %s", dataLine)
	}
}

// TestCSVTableMixedLabels tests CSV with some fields having Labels and others not
func TestCSVTableMixedLabels(t *testing.T) {
	tableData := []map[string]interface{}{
		{
			"id":     "ROW-001",
			"status": "active",
			"count":  42,
		},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "data",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						{Name: "id", Label: "Identifier", Type: "string"}, // Has Label
						{Name: "status", Type: "string"},                  // No Label - should use Name
						{Name: "count", Label: "Item Count", Type: "int"}, // Has Label
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

	csvFormatter := NewCSVFormatter()
	csvOutput, err := csvFormatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format CSV: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(csvOutput), "\n")
	headerLine := lines[0]
	t.Logf("CSV header: %s", headerLine)

	// Should contain Label where available
	if !strings.Contains(headerLine, "Identifier") {
		t.Errorf("Header should contain 'Identifier' label, got: %s", headerLine)
	}
	if !strings.Contains(headerLine, "Item Count") {
		t.Errorf("Header should contain 'Item Count' label, got: %s", headerLine)
	}

	// Should use Name when Label is empty
	if !strings.Contains(headerLine, "status") {
		t.Errorf("Header should contain 'status' (no label defined), got: %s", headerLine)
	}
}

// TestCSVTableColumnOrder tests that CSV respects the order of TableOptions.Fields
func TestCSVTableColumnOrder(t *testing.T) {
	tableData := []map[string]interface{}{
		{
			"zebra":   "Z",
			"alpha":   "A",
			"bravo":   "B",
			"charlie": "C",
		},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "items",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						// Intentionally not in alphabetical order
						{Name: "charlie", Label: "Charlie", Type: "string"},
						{Name: "alpha", Label: "Alpha", Type: "string"},
						{Name: "zebra", Label: "Zebra", Type: "string"},
						{Name: "bravo", Label: "Bravo", Type: "string"},
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

	csvFormatter := NewCSVFormatter()
	csvOutput, err := csvFormatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format CSV: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(csvOutput), "\n")
	headerLine := lines[0]
	dataLine := lines[1]

	t.Logf("CSV header: %s", headerLine)
	t.Logf("CSV data: %s", dataLine)

	// Headers should be in the order specified in TableOptions.Fields
	expectedHeaderOrder := []string{"Charlie", "Alpha", "Zebra", "Bravo"}
	expectedDataOrder := []string{"C", "A", "Z", "B"}

	// Parse CSV manually to check order
	headerParts := parseCSVLine(headerLine)
	dataParts := parseCSVLine(dataLine)

	if len(headerParts) != len(expectedHeaderOrder) {
		t.Fatalf("Expected %d header columns, got %d", len(expectedHeaderOrder), len(headerParts))
	}

	for i, expected := range expectedHeaderOrder {
		if headerParts[i] != expected {
			t.Errorf("Header column %d: expected '%s', got '%s'", i, expected, headerParts[i])
		}
	}

	for i, expected := range expectedDataOrder {
		if dataParts[i] != expected {
			t.Errorf("Data column %d: expected '%s', got '%s'", i, expected, dataParts[i])
		}
	}
}

// TestCSVTableSpecialCharacters tests CSV with special characters in labels
func TestCSVTableSpecialCharacters(t *testing.T) {
	tableData := []map[string]interface{}{
		{
			"field1": "value1",
			"field2": "value2",
		},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "data",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						{Name: "field1", Label: "Label with, comma", Type: "string"},
						{Name: "field2", Label: "Label with \"quotes\"", Type: "string"},
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

	csvFormatter := NewCSVFormatter()
	csvOutput, err := csvFormatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format CSV: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(csvOutput), "\n")
	headerLine := lines[0]
	t.Logf("CSV header: %s", headerLine)

	// CSV should properly escape special characters
	if !strings.Contains(csvOutput, "Label with, comma") {
		t.Errorf("CSV should contain label with comma")
	}
	// The CSV library escapes quotes by doubling them: "Label with ""quotes"""
	if !strings.Contains(csvOutput, "Label with \"\"quotes\"\"") {
		t.Errorf("CSV should contain properly escaped label with quotes")
	}
}

// TestCSVTableEmptyData tests CSV with no data rows (headers only)
func TestCSVTableEmptyData(t *testing.T) {
	// Empty table data
	tableData := []map[string]interface{}{}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "items",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						{Name: "id", Label: "ID", Type: "string"},
						{Name: "name", Label: "Name", Type: "string"},
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

	csvFormatter := NewCSVFormatter()
	csvOutput, err := csvFormatter.FormatPrettyData(prettyData)
	if err != nil {
		t.Fatalf("Failed to format CSV: %v", err)
	}

	// Empty table should produce empty CSV output (no headers when no data)
	if strings.TrimSpace(csvOutput) != "" {
		t.Errorf("Empty table should produce empty CSV, got: %s", csvOutput)
	}
}

// parseCSVLine is a simple CSV line parser for testing
func parseCSVLine(line string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false

	for i, char := range line {
		switch char {
		case '"':
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				// Escaped quote
				current.WriteByte('"')
				i++ // Skip next quote
			} else {
				// Toggle quote state
				inQuotes = !inQuotes
			}
		case ',':
			if inQuotes {
				current.WriteByte(',')
			} else {
				result = append(result, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	result = append(result, current.String())
	return result
}
