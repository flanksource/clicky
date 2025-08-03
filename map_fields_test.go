package clicky

import (
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
	"strings"
	"testing"
)

func TestMapFieldsRendering(t *testing.T) {
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
		"items": []map[string]interface{}{
			{"product": "Widget", "price": 10.99, "quantity": 2},
			{"product": "Gadget", "price": 15.50, "quantity": 1},
		},
	}

	// Create schema that includes map fields
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{Name: "name", Type: "string"},
			{Name: "age", Type: "int"},
			{Name: "address", Type: "map"},
			{Name: "metadata", Type: "map"},
			{
				Name:   "items",
				Format: "table",
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						{Name: "product", Type: "string"},
						{Name: "price", Type: "float", Format: "currency"},
						{Name: "quantity", Type: "int"},
					},
				},
			},
		},
	}

	parser := NewStructParser()

	// Test ParseDataWithSchema
	t.Run("ParseDataWithSchema", func(t *testing.T) {
		prettyData, err := parser.ParseDataWithSchema(testData, schema)
		if err != nil {
			t.Fatalf("ParseDataWithSchema failed: %v", err)
		}

		// Check that scalar fields are parsed
		if _, exists := prettyData.Values["name"]; !exists {
			t.Error("name field not found in Values")
		}
		if _, exists := prettyData.Values["age"]; !exists {
			t.Error("age field not found in Values")
		}

		// Check that map fields are parsed
		if _, exists := prettyData.Values["address"]; !exists {
			t.Error("address map field not found in Values")
		}
		if _, exists := prettyData.Values["metadata"]; !exists {
			t.Error("metadata map field not found in Values")
		}

		// Check that table data is parsed
		if _, exists := prettyData.Tables["items"]; !exists {
			t.Error("items table not found in Tables")
		}

		if len(prettyData.Tables["items"]) != 2 {
			t.Errorf("Expected 2 items in table, got %d", len(prettyData.Tables["items"]))
		}
	})

	// Test PrettyFormatter rendering
	t.Run("PrettyFormatter", func(t *testing.T) {
		prettyData, err := parser.ParseDataWithSchema(testData, schema)
		if err != nil {
			t.Fatalf("ParseDataWithSchema failed: %v", err)
		}

		formatter := formatters.NewPrettyFormatter()
		output, err := formatter.Format(prettyData)
		if err != nil {
			t.Fatalf("PrettyFormatter.Format failed: %v", err)
		}

		// Check that output contains map field content
		if !strings.Contains(output, "Address") {
			t.Error("Output doesn't contain address field")
		}
		if !strings.Contains(output, "Metadata") {
			t.Error("Output doesn't contain metadata field")
		}
		if !strings.Contains(output, "123 Main St") {
			t.Error("Output doesn't contain address content")
		}
		if !strings.Contains(output, "Widget") {
			t.Error("Output doesn't contain table content")
		}

		t.Logf("Pretty output:\n%s", output)
	})

	// Test HTMLFormatter rendering
	t.Run("HTMLFormatter", func(t *testing.T) {
		prettyData, err := parser.ParseDataWithSchema(testData, schema)
		if err != nil {
			t.Fatalf("ParseDataWithSchema failed: %v", err)
		}

		formatter := formatters.NewHTMLFormatter()
		formatter.IncludeCSS = false // Simplify output for testing
		output, err := formatter.Format(prettyData)
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
		if !strings.Contains(output, "Widget") {
			t.Error("HTML output doesn't contain table content")
		}

		t.Logf("HTML output:\n%s", output)
	})
}

func TestNestedMapFieldFormatting(t *testing.T) {
	// Test data with deeply nested maps
	testData := map[string]interface{}{
		"config": map[string]interface{}{
			"database": map[string]interface{}{
				"host": "localhost",
				"port": 5432,
				"credentials": map[string]interface{}{
					"username": "admin",
					"password": "secret",
				},
			},
			"cache": map[string]interface{}{
				"type": "redis",
				"ttl":  3600,
			},
		},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{Name: "config", Type: "map"},
		},
	}

	parser := NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(testData, schema)
	if err != nil {
		t.Fatalf("ParseDataWithSchema failed: %v", err)
	}

	// Check that nested map is properly parsed
	configValue, exists := prettyData.Values["config"]
	if !exists {
		t.Fatal("config field not found")
	}

	// Test that the formatted output contains nested structure
	formatted := configValue.Formatted()
	if !strings.Contains(formatted, "localhost") {
		t.Error("Formatted output doesn't contain nested map content")
	}

	t.Logf("Nested map formatted: %s", formatted)
}

func TestMapFieldsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		schema   *api.PrettyObject
		expected []string // strings that should be present in output
	}{
		{
			name: "empty_map",
			data: map[string]interface{}{
				"empty_map": map[string]interface{}{},
			},
			schema: &api.PrettyObject{
				Fields: []api.PrettyField{
					{Name: "empty_map", Type: "map"},
				},
			},
			expected: []string{"Empty Map", "(empty)"},
		},
		{
			name: "mixed_types_in_map",
			data: map[string]interface{}{
				"mixed": map[string]interface{}{
					"string":  "hello",
					"number":  42,
					"boolean": true,
					"float":   3.14,
				},
			},
			schema: &api.PrettyObject{
				Fields: []api.PrettyField{
					{Name: "mixed", Type: "map"},
				},
			},
			expected: []string{"Mixed", "hello", "42", "true", "3.14"},
		},
		{
			name: "map_with_special_characters",
			data: map[string]interface{}{
				"special": map[string]interface{}{
					"key with spaces":      "value with spaces",
					"key-with-dashes":      "value-with-dashes",
					"key_with_underscores": "value_with_underscores",
				},
			},
			schema: &api.PrettyObject{
				Fields: []api.PrettyField{
					{Name: "special", Type: "map"},
				},
			},
			expected: []string{"Special", "Key With Spaces", "value with spaces", "Key With Dashes"},
		},
		{
			name: "deeply_nested_map",
			data: map[string]interface{}{
				"deep": map[string]interface{}{
					"level1": map[string]interface{}{
						"level2": map[string]interface{}{
							"level3": "deep value",
						},
					},
				},
			},
			schema: &api.PrettyObject{
				Fields: []api.PrettyField{
					{Name: "deep", Type: "map"},
				},
			},
			expected: []string{"Deep", "Level1", "Level2", "Level3", "deep value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewStructParser()

			// Test parsing
			prettyData, err := parser.ParseDataWithSchema(tt.data, tt.schema)
			if err != nil {
				t.Fatalf("ParseDataWithSchema failed: %v", err)
			}

			// Test pretty formatting
			formatter := formatters.NewPrettyFormatter()
			output, err := formatter.Format(prettyData)
			if err != nil {
				t.Fatalf("PrettyFormatter.Format failed: %v", err)
			}

			// Check all expected strings are present
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("Output doesn't contain expected string: %q\nOutput:\n%s", expected, output)
				}
			}

			t.Logf("Output for %s:\n%s", tt.name, output)

			// Test HTML formatting
			htmlFormatter := formatters.NewHTMLFormatter()
			htmlFormatter.IncludeCSS = false
			htmlOutput, err := htmlFormatter.Format(prettyData)
			if err != nil {
				t.Fatalf("HTMLFormatter.Format failed: %v", err)
			}

			// Check HTML output contains expected content
			for _, expected := range tt.expected {
				if !strings.Contains(htmlOutput, expected) {
					t.Errorf("HTML output doesn't contain expected string: %q", expected)
				}
			}
		})
	}
}

func TestMapInTableFields(t *testing.T) {
	// Test that maps inside table rows are properly formatted
	testData := map[string]interface{}{
		"events": []map[string]interface{}{
			{
				"id":   1,
				"name": "Event 1",
				"metadata": map[string]interface{}{
					"source":   "api",
					"priority": "high",
				},
			},
			{
				"id":   2,
				"name": "Event 2",
				"metadata": map[string]interface{}{
					"source":   "webhook",
					"priority": "low",
				},
			},
		},
	}

	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "events",
				Format: "table",
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						{Name: "id", Type: "int"},
						{Name: "name", Type: "string"},
						{Name: "metadata", Type: "map"},
					},
				},
			},
		},
	}

	parser := NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(testData, schema)
	if err != nil {
		t.Fatalf("ParseDataWithSchema failed: %v", err)
	}

	// Check that table data contains formatted maps
	events, exists := prettyData.Tables["events"]
	if !exists {
		t.Fatal("events table not found")
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Check first event metadata
	firstEvent := events[0]
	metadataField, exists := firstEvent["metadata"]
	if !exists {
		t.Fatal("metadata field not found in first event")
	}

	formatted := metadataField.Formatted()
	if !strings.Contains(formatted, "Source: api") {
		t.Errorf("Formatted metadata doesn't contain expected content: %s", formatted)
	}

	// Test table rendering
	formatter := formatters.NewPrettyFormatter()
	output, err := formatter.Format(prettyData)
	if err != nil {
		t.Fatalf("PrettyFormatter.Format failed: %v", err)
	}

	// Check that table contains formatted map content
	if !strings.Contains(output, "Source: api") {
		t.Error("Table output doesn't contain formatted map content")
	}

	t.Logf("Table with maps output:\n%s", output)
}

func TestSchemaTypeMismatch(t *testing.T) {
	// Test when schema says "struct" but data is actually a map (common with JSON)
	testData := map[string]interface{}{
		"user_info": map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"age":   30,
		},
		"settings": map[string]interface{}{
			"theme":     "dark",
			"language":  "en",
			"auto_save": true,
		},
	}

	// Schema incorrectly defines these as "struct" instead of "map"
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{Name: "user_info", Type: "struct"}, // Should be "map" but schema says "struct"
			{Name: "settings", Type: "struct"},  // Should be "map" but schema says "struct"
		},
	}

	parser := NewStructParser()
	prettyData, err := parser.ParseDataWithSchema(testData, schema)
	if err != nil {
		t.Fatalf("ParseDataWithSchema failed: %v", err)
	}

	// Test that the values are properly formatted as maps despite schema saying "struct"
	userInfo, exists := prettyData.Values["user_info"]
	if !exists {
		t.Fatal("user_info field not found")
	}

	formatted := userInfo.Formatted()
	// Should be formatted as a readable map, not raw Go representation
	if !strings.Contains(formatted, "Name: John Doe") {
		t.Errorf("user_info not formatted as map: %s", formatted)
	}
	if strings.Contains(formatted, "map[") {
		t.Errorf("user_info still shows raw Go map representation: %s", formatted)
	}

	// Test pretty formatter output
	formatter := formatters.NewPrettyFormatter()
	output, err := formatter.Format(prettyData)
	if err != nil {
		t.Fatalf("PrettyFormatter.Format failed: %v", err)
	}

	// Check that output contains properly formatted maps
	if !strings.Contains(output, "Name: John Doe") {
		t.Error("Pretty output doesn't contain properly formatted user_info")
	}
	if !strings.Contains(output, "Auto Save: true") {
		t.Error("Pretty output doesn't contain properly formatted settings")
	}
	if strings.Contains(output, "map[") {
		t.Error("Pretty output still contains raw Go map representation")
	}

	t.Logf("Schema type mismatch output:\n%s", output)
}
