package api

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStruct implements PrettyRow interface for testing
type TestStruct struct {
	Name   string
	Count  int
	Status string
}

// PrettyRow implements the PrettyRow interface
func (t TestStruct) PrettyRow(opts interface{}) map[string]Text {
	result := make(map[string]Text)

	// Custom column name and content
	result["Name"] = Text{Content: t.Name, Style: "font-bold"}

	// Conditional styling based on format options
	countStyle := "text-blue-600"
	if opts != nil {
		// Simple check for NoColor - in real implementation, you'd type assert to FormatOptions
		if s := fmt.Sprintf("%+v", opts); s != "" && fmt.Sprintf("%v", opts) != "<nil>" {
			// For testing, assume NoColor is passed in a simple struct
			if noColorOpts, ok := opts.(struct{ NoColor bool }); ok && noColorOpts.NoColor {
				countStyle = ""
			}
		}
	}
	result["Count"] = Text{Content: fmt.Sprintf("%d", t.Count), Style: countStyle}

	// Status with conditional coloring
	statusStyle := "text-green-600"
	if t.Status == "error" {
		statusStyle = "text-red-600"
	}
	if opts != nil {
		if noColorOpts, ok := opts.(struct{ NoColor bool }); ok && noColorOpts.NoColor {
			statusStyle = ""
		}
	}
	result["Status"] = Text{Content: t.Status, Style: statusStyle}

	return result
}

func TestPrettyRowInterface(t *testing.T) {
	// Create a test struct that implements PrettyRow
	testStruct := TestStruct{
		Name:   "Test Item",
		Count:  5,
		Status: "success",
	}

	// Test basic PrettyRow functionality
	prettyRow := testStruct.PrettyRow(nil)
	assert.Equal(t, 3, len(prettyRow))
	assert.Equal(t, "Test Item", prettyRow["Name"].Content)
	assert.Equal(t, "font-bold", prettyRow["Name"].Style)
	assert.Equal(t, "5", prettyRow["Count"].Content)
	assert.Equal(t, "text-blue-600", prettyRow["Count"].Style)
	assert.Equal(t, "success", prettyRow["Status"].Content)
	assert.Equal(t, "text-green-600", prettyRow["Status"].Style)
}

func TestPrettyRowWithFormatOptions(t *testing.T) {
	testStruct := TestStruct{
		Name:   "Test Item",
		Count:  3,
		Status: "error",
	}

	// Mock FormatOptions with NoColor
	opts := struct{ NoColor bool }{NoColor: true}

	prettyRow := testStruct.PrettyRow(opts)

	// Verify that styles are disabled when NoColor is true
	assert.Equal(t, "", prettyRow["Count"].Style)
	assert.Equal(t, "", prettyRow["Status"].Style)
	assert.Equal(t, "font-bold", prettyRow["Name"].Style) // Name should still have style
}

func TestStructToRowWithOptionsUsesInterface(t *testing.T) {
	parser := NewStructParser()
	testStruct := TestStruct{
		Name:   "Interface Test",
		Count:  7,
		Status: "active",
	}

	val := reflect.ValueOf(testStruct)
	opts := struct{ NoColor bool }{NoColor: false}

	// Call StructToRowWithOptions which should detect and use the PrettyRow interface
	row, err := parser.StructToRowWithOptions(val, opts)
	assert.NoError(t, err)
	assert.NotNil(t, row)

	// Verify that the PrettyRow interface was used
	assert.Equal(t, 3, len(row))

	// Check that the custom implementation was used
	nameField, exists := row["Name"]
	assert.True(t, exists)
	assert.Equal(t, "Interface Test", nameField.Value)
	assert.NotNil(t, nameField.Text)
	assert.Equal(t, "Interface Test", nameField.Text.Content)
	assert.Equal(t, "font-bold", nameField.Text.Style)

	countField, exists := row["Count"]
	assert.True(t, exists)
	assert.Equal(t, "7", countField.Value)
	assert.NotNil(t, countField.Text)
	assert.Equal(t, "text-blue-600", countField.Text.Style)
}

func TestStructToRowFallbackWithoutInterface(t *testing.T) {
	parser := NewStructParser()

	// Regular struct without PrettyRow interface
	regularStruct := struct {
		Name  string
		Value int
	}{
		Name:  "Regular Struct",
		Value: 42,
	}

	val := reflect.ValueOf(regularStruct)
	opts := struct{ NoColor bool }{NoColor: false}

	// Should fall back to reflection-based approach
	row, err := parser.StructToRowWithOptions(val, opts)
	assert.NoError(t, err)
	assert.NotNil(t, row)

	// Verify fallback behavior
	nameField, exists := row["Name"]
	assert.True(t, exists)
	assert.Equal(t, "Regular Struct", nameField.Value)
}