package formatters

import (
	"reflect"
	"testing"
)

// TestFlattenSlice tests the FlattenSlice function with various input types
func TestFlattenSlice(t *testing.T) {
	type TestStruct struct {
		Name  string
		Value int
	}

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
		isFlat   bool // true if flattening should occur
	}{
		{
			name: "slice of slice of structs",
			input: [][]TestStruct{
				{
					{Name: "a", Value: 1},
					{Name: "b", Value: 2},
				},
				{
					{Name: "c", Value: 3},
					{Name: "d", Value: 4},
				},
			},
			expected: []TestStruct{
				{Name: "a", Value: 1},
				{Name: "b", Value: 2},
				{Name: "c", Value: 3},
				{Name: "d", Value: 4},
			},
			isFlat: true,
		},
		{
			name: "slice of structs - no flattening",
			input: []TestStruct{
				{Name: "a", Value: 1},
				{Name: "b", Value: 2},
			},
			expected: []TestStruct{
				{Name: "a", Value: 1},
				{Name: "b", Value: 2},
			},
			isFlat: false,
		},
		{
			name: "empty slice",
			input: [][]TestStruct{},
			expected: [][]TestStruct{},
			isFlat: false,
		},
		{
			name: "slice of slice of maps",
			input: [][]map[string]interface{}{
				{
					{"key": "value1"},
					{"key": "value2"},
				},
				{
					{"key": "value3"},
				},
			},
			expected: []map[string]interface{}{
				{"key": "value1"},
				{"key": "value2"},
				{"key": "value3"},
			},
			isFlat: true,
		},
		{
			name: "slice of maps - no flattening",
			input: []map[string]interface{}{
				{"key": "value1"},
				{"key": "value2"},
			},
			expected: []map[string]interface{}{
				{"key": "value1"},
				{"key": "value2"},
			},
			isFlat: false,
		},
		{
			name: "slice with nil inner slice",
			input: [][]TestStruct{
				{
					{Name: "a", Value: 1},
				},
				nil,
				{
					{Name: "b", Value: 2},
				},
			},
			expected: []TestStruct{
				{Name: "a", Value: 1},
				{Name: "b", Value: 2},
			},
			isFlat: true,
		},
		{
			name: "slice of empty slices",
			input: [][]TestStruct{
				{},
				{},
			},
			expected: []TestStruct{},
			isFlat: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputVal := reflect.ValueOf(tt.input)
			result := FlattenSlice(inputVal)

			// Check if result is valid
			if !result.IsValid() {
				t.Fatalf("FlattenSlice returned invalid value")
			}

			// For empty slices after flattening, check length
			expectedVal := reflect.ValueOf(tt.expected)
			if expectedVal.Len() == 0 && result.Len() == 0 {
				return // Both empty, test passes
			}

			// Check length matches expected
			if result.Len() != expectedVal.Len() {
				t.Errorf("Expected length %d, got %d", expectedVal.Len(), result.Len())
				return
			}

			// Verify each element matches
			for i := 0; i < result.Len(); i++ {
				resultElem := result.Index(i).Interface()
				expectedElem := expectedVal.Index(i).Interface()

				if !reflect.DeepEqual(resultElem, expectedElem) {
					t.Errorf("Element %d: expected %+v, got %+v", i, expectedElem, resultElem)
				}
			}
		})
	}
}

// TestFlattenSliceNonSliceInput tests FlattenSlice with non-slice inputs
func TestFlattenSliceNonSliceInput(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{
			name:  "string input",
			input: "not a slice",
		},
		{
			name:  "int input",
			input: 42,
		},
		{
			name: "struct input",
			input: struct {
				Name string
			}{Name: "test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputVal := reflect.ValueOf(tt.input)
			result := FlattenSlice(inputVal)

			// Should return input unchanged
			if result.Interface() != tt.input {
				t.Errorf("Expected unchanged input, got different value")
			}
		})
	}
}

// TestConvertSliceToPrettyDataWithSliceOfSlice tests the full conversion pipeline
func TestConvertSliceToPrettyDataWithSliceOfSlice(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	// Test slice of slice of structs
	input := [][]Person{
		{
			{Name: "Alice", Age: 30},
			{Name: "Bob", Age: 25},
		},
		{
			{Name: "Charlie", Age: 35},
		},
	}

	prettyData, err := ToPrettyData(input)
	if err != nil {
		t.Fatalf("ToPrettyData failed: %v", err)
	}

	// Should have a table field
	if len(prettyData.Tables) == 0 {
		t.Error("Expected tables to be populated")
	}

	// Get the table data
	tableName := prettyData.Schema.Fields[0].Name
	rows, exists := prettyData.Tables[tableName]
	if !exists {
		t.Fatalf("Table %s not found", tableName)
	}

	// Should have 3 rows after flattening
	if len(rows) != 3 {
		t.Errorf("Expected 3 rows after flattening, got %d", len(rows))
	}

	// Verify the data is correct
	expectedNames := []string{"Alice", "Bob", "Charlie"}
	for i, row := range rows {
		nameField, exists := row["Name"]
		if !exists {
			t.Errorf("Row %d missing Name field", i)
			continue
		}
		if nameField.Value != expectedNames[i] {
			t.Errorf("Row %d: expected name %s, got %v", i, expectedNames[i], nameField.Value)
		}
	}
}

// TestConvertSliceToPrettyDataWithSliceOfSliceOfMaps tests flattening maps
func TestConvertSliceToPrettyDataWithSliceOfSliceOfMaps(t *testing.T) {
	input := [][]map[string]interface{}{
		{
			{"name": "Alice", "age": 30},
			{"name": "Bob", "age": 25},
		},
		{
			{"name": "Charlie", "age": 35},
		},
	}

	prettyData, err := ToPrettyData(input)
	if err != nil {
		t.Fatalf("ToPrettyData failed: %v", err)
	}

	// Should have a table field
	if len(prettyData.Tables) == 0 {
		t.Error("Expected tables to be populated")
	}

	// Get the table data
	tableName := prettyData.Schema.Fields[0].Name
	rows, exists := prettyData.Tables[tableName]
	if !exists {
		t.Fatalf("Table %s not found", tableName)
	}

	// Should have 3 rows after flattening
	if len(rows) != 3 {
		t.Errorf("Expected 3 rows after flattening, got %d", len(rows))
	}
}

// TestConvertSliceWithFormatOptions tests slice flattening with format options
func TestConvertSliceWithFormatOptions(t *testing.T) {
	type Item struct {
		ID   int
		Name string
	}

	input := [][]Item{
		{{ID: 1, Name: "Item 1"}},
		{{ID: 2, Name: "Item 2"}, {ID: 3, Name: "Item 3"}},
	}

	opts := FormatOptions{Table: true}
	prettyData, err := ToPrettyDataWithOptions(input, opts)
	if err != nil {
		t.Fatalf("ToPrettyDataWithOptions failed: %v", err)
	}

	// Should have a table field
	if len(prettyData.Tables) == 0 {
		t.Error("Expected tables to be populated")
	}

	tableName := prettyData.Schema.Fields[0].Name
	rows, exists := prettyData.Tables[tableName]
	if !exists {
		t.Fatalf("Table %s not found", tableName)
	}

	// Should have 3 rows after flattening
	if len(rows) != 3 {
		t.Errorf("Expected 3 rows after flattening, got %d", len(rows))
	}
}
