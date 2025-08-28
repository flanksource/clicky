package formatters

import (
	"testing"

	"github.com/flanksource/clicky/api"
)

func TestStyleTagParsing(t *testing.T) {
	tests := []struct {
		name       string
		tag        string
		wantStyle  string
		wantHeader string
		wantRow    string
	}{
		{
			name:      "simple style",
			tag:       "format,style=text-red-500",
			wantStyle: "text-red-500",
		},
		{
			name:       "table styles",
			tag:        "table,header_style=text-blue-700 font-bold,row_style=text-gray-600",
			wantHeader: "text-blue-700 font-bold",
			wantRow:    "text-gray-600",
		},
		{
			name:      "multiple Tailwind classes",
			tag:       "format,style=text-green-600 bg-green-100 font-bold",
			wantStyle: "text-green-600 bg-green-100 font-bold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := api.ParsePrettyTag(tt.tag)

			if field.Style != tt.wantStyle {
				t.Errorf("ParsePrettyTag() style = %v, want %v", field.Style, tt.wantStyle)
			}

			if field.TableOptions.HeaderStyle != tt.wantHeader {
				t.Errorf("ParsePrettyTag() header_style = %v, want %v", field.TableOptions.HeaderStyle, tt.wantHeader)
			}

			if field.TableOptions.RowStyle != tt.wantRow {
				t.Errorf("ParsePrettyTag() row_style = %v, want %v", field.TableOptions.RowStyle, tt.wantRow)
			}
		})
	}
}

func TestTailwindIntegration(t *testing.T) {
	// Test struct with style tags
	type TestData struct {
		Name   string  `json:"name" pretty:"string,style=text-blue-600 font-bold"`
		Status string  `json:"status" pretty:"string,style=text-green-500"`
		Amount float64 `json:"amount" pretty:"currency,style=text-yellow-600"`
	}

	data := TestData{
		Name:   "Test Item",
		Status: "Active",
		Amount: 1234.56,
	}

	// Parse the struct
	parser := NewStructParser()
	result, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify style fields are set
	for _, field := range result.Fields {
		switch field.Name {
		case "Name":
			if field.Style != "text-blue-600 font-bold" {
				t.Errorf("Name field style = %v, want text-blue-600 font-bold", field.Style)
			}
		case "Status":
			if field.Style != "text-green-500" {
				t.Errorf("Status field style = %v, want text-green-500", field.Style)
			}
		case "Amount":
			if field.Style != "text-yellow-600" {
				t.Errorf("Amount field style = %v, want text-yellow-600", field.Style)
			}
		}
	}
}

func TestTableStyleTags(t *testing.T) {
	// Test struct with table and style tags
	type TableRow struct {
		ID    int     `json:"id" pretty:"int"`
		Name  string  `json:"name" pretty:"string"`
		Value float64 `json:"value" pretty:"float"`
	}

	type TestData struct {
		Title string     `json:"title" pretty:"string"`
		Items []TableRow `json:"items" pretty:"table,header_style=bg-blue-100 text-blue-800 font-bold,row_style=text-gray-700"`
	}

	data := TestData{
		Title: "Test Table",
		Items: []TableRow{
			{ID: 1, Name: "Item 1", Value: 100.50},
			{ID: 2, Name: "Item 2", Value: 200.75},
		},
	}

	// Parse the struct
	parser := NewStructParser()
	result, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Find the items field (using JSON tag name)
	var itemsField *api.PrettyField
	for _, field := range result.Fields {
		if field.Name == "items" {
			itemsField = &field
			break
		}
	}

	if itemsField == nil {
		t.Fatal("items field not found")
	}

	// Verify table styles
	expectedHeader := "bg-blue-100 text-blue-800 font-bold"
	expectedRow := "text-gray-700"

	if itemsField.TableOptions.HeaderStyle != expectedHeader {
		t.Errorf("Table header_style = %v, want %v", itemsField.TableOptions.HeaderStyle, expectedHeader)
	}

	if itemsField.TableOptions.RowStyle != expectedRow {
		t.Errorf("Table row_style = %v, want %v", itemsField.TableOptions.RowStyle, expectedRow)
	}
}
