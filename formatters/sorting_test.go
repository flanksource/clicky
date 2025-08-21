package formatters

import (
	"reflect"
	"testing"

	"github.com/flanksource/clicky/api"
)

func TestSortRows(t *testing.T) {
	// Create test rows
	rows := []api.PrettyDataRow{
		{"name": api.FieldValue{Value: "zebra"}, "language": api.FieldValue{Value: "go"}, "version": api.FieldValue{Value: "1.0"}},
		{"name": api.FieldValue{Value: "apple"}, "language": api.FieldValue{Value: "python"}, "version": api.FieldValue{Value: "2.0"}},
		{"name": api.FieldValue{Value: "banana"}, "language": api.FieldValue{Value: "go"}, "version": api.FieldValue{Value: "1.1"}},
		{"name": api.FieldValue{Value: "cherry"}, "language": api.FieldValue{Value: "javascript"}, "version": api.FieldValue{Value: "3.0"}},
		{"name": api.FieldValue{Value: "date"}, "language": api.FieldValue{Value: "go"}, "version": api.FieldValue{Value: "1.2"}},
	}
	
	// Define sort fields (language first, then name)
	sortFields := []SortField{
		{Name: "language", Priority: 1, Direction: "asc"},
		{Name: "name", Priority: 2, Direction: "asc"},
	}
	
	// Sort the rows
	SortRows(rows, sortFields)
	
	// Check the order
	expected := []string{
		"banana", // go, banana
		"date",   // go, date
		"zebra",  // go, zebra
		"cherry", // javascript, cherry
		"apple",  // python, apple
	}
	
	for i, exp := range expected {
		actualName := rows[i]["name"].Value.(string)
		if actualName != exp {
			t.Errorf("Row %d: expected name=%s, got %s", i, exp, actualName)
		}
	}
}

func TestExtractSortFields(t *testing.T) {
	// Define a test struct
	type TestStruct struct {
		ID       int    `json:"id" pretty:"hide"`
		Name     string `json:"name" pretty:"label=Name,sort=2"`
		Language string `json:"language" pretty:"label=Language,sort=1"`
		Version  string `json:"version" pretty:"label=Version"`
	}
	
	// Get the type
	typ := reflect.TypeOf(TestStruct{})
	
	// Extract sort fields
	sortFields := ExtractSortFields(typ)
	
	// Verify we got 2 sort fields
	if len(sortFields) != 2 {
		t.Fatalf("Expected 2 sort fields, got %d", len(sortFields))
	}
	
	// Check they're in the right order (sorted by priority)
	if sortFields[0].Name != "language" || sortFields[0].Priority != 1 {
		t.Errorf("First sort field should be language with priority 1, got %+v", sortFields[0])
	}
	
	if sortFields[1].Name != "name" || sortFields[1].Priority != 2 {
		t.Errorf("Second sort field should be name with priority 2, got %+v", sortFields[1])
	}
}