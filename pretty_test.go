package clicky

import (
	"testing"
)

func TestPrettyParser(t *testing.T) {
	parser := NewPrettyParser()

	// Test with the sample Invoice struct
	invoice := Invoice{
		ID:         "INV-001",
		Total:      125.50,
		CreatedAt:  "1640995200", // epoch timestamp
		CustomerID: "CUST-123",
		Status:     "paid",
		Items: []InvoiceItem{
			{
				ID:          "ITEM-001",
				Description: "Web Development",
				Amount:      100.0,
				Quantity:    2.5,
				Total:       250.0,
			},
			{
				ID:          "ITEM-002",
				Description: "Consulting",
				Amount:      50.0,
				Quantity:    1.0,
				Total:       50.0,
			},
		},
	}

	result, err := parser.Parse(invoice)
	if err != nil {
		t.Fatalf("Failed to parse invoice: %v", err)
	}

	t.Logf("Formatted invoice:\n%s", result)

	// Test with slice of items
	items := []InvoiceItem{
		{
			ID:          "ITEM-001",
			Description: "Web Development",
			Amount:      100.0,
			Quantity:    2.5,
			Total:       250.0,
		},
		{
			ID:          "ITEM-002",
			Description: "Consulting",
			Amount:      50.0,
			Quantity:    1.0,
			Total:       50.0,
		},
	}

	result2, err := parser.Parse(items)
	if err != nil {
		t.Fatalf("Failed to parse items slice: %v", err)
	}

	t.Logf("Formatted items:\n%s", result2)
}

func TestPrettyParserNoColor(t *testing.T) {
	parser := NewPrettyParser()
	parser.NoColor = true

	invoice := Invoice{
		ID:     "INV-001",
		Total:  125.50,
		Status: "paid",
	}

	result, err := parser.Parse(invoice)
	if err != nil {
		t.Fatalf("Failed to parse invoice: %v", err)
	}

	t.Logf("Formatted invoice (no color):\n%s", result)
}

func TestJSONParsing(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected bool // true if should parse successfully
	}{
		{
			name:     "valid JSON",
			input:    `{"name": "test", "value": 42}`,
			expected: true,
		},
		{
			name:     "JSON with comments",
			input:    `{"name": "test", // this is a comment\n"value": 42}`,
			expected: true,
		},
		{
			name:     "JSON with trailing comma",
			input:    `{"name": "test", "value": 42,}`,
			expected: true,
		},
		{
			name:     "quoted JSON string",
			input:    `"{\"name\": \"test\"}"`,
			expected: true,
		},
		{
			name:     "invalid JSON",
			input:    `{invalid json}`,
			expected: true, // should still return as string
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseJSON([]byte(tc.input))
			if tc.expected && err != nil {
				t.Errorf("Expected successful parsing, got error: %v", err)
			}
			if result == nil {
				t.Errorf("Expected non-nil result")
			}
			t.Logf("Input: %s\nResult: %+v", tc.input, result)
		})
	}
}

func TestColorConditions(t *testing.T) {
	parser := NewPrettyParser()

	// Test struct with color conditions
	type TestStruct struct {
		Status   string  `json:"status" pretty:"color,green=paid,red=unpaid,blue=pending"`
		Amount   float64 `json:"amount" pretty:"color,green=>0,red=<0"`
		Quantity int     `json:"quantity" pretty:"color,green=>=10,yellow=<10"`
	}

	testData := TestStruct{
		Status:   "paid",
		Amount:   100.50,
		Quantity: 5,
	}

	result, err := parser.Parse(testData)
	if err != nil {
		t.Fatalf("Failed to parse test struct: %v", err)
	}

	t.Logf("Formatted test struct with colors:\n%s", result)
}

func TestTableFormatting(t *testing.T) {
	parser := NewPrettyParser()

	// Test struct with nested table
	type Order struct {
		ID    string        `json:"id"`
		Items []InvoiceItem `json:"items" pretty:"table,sort=amount,dir=desc"`
		Total float64       `json:"total" pretty:"currency"`
	}

	order := Order{
		ID:    "ORD-001",
		Total: 300.0,
		Items: []InvoiceItem{
			{
				ID:          "ITEM-001",
				Description: "Consulting",
				Amount:      50.0,
				Quantity:    1.0,
				Total:       50.0,
			},
			{
				ID:          "ITEM-002",
				Description: "Web Development",
				Amount:      100.0,
				Quantity:    2.5,
				Total:       250.0,
			},
		},
	}

	result, err := parser.Parse(order)
	if err != nil {
		t.Fatalf("Failed to parse order: %v", err)
	}

	t.Logf("Formatted order with table:\n%s", result)
}
