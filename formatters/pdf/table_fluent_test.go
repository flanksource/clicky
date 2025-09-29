package pdf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTable(t *testing.T) {
	table := NewTable()

	// Test default values
	assert.NotNil(t, table)
	assert.Empty(t, table.Columns)
	assert.Empty(t, table.Rows)
	assert.True(t, table.ShowBorders)
	assert.True(t, table.TopAlign)
	assert.Equal(t, "font-bold text-white bg-blue-600 text-center text-sm p-2", table.HeaderStyle)
	assert.Equal(t, "text-sm text-gray-800 bg-white p-2", table.RowStyle)
	assert.Equal(t, "bg-gray-50", table.AlternateRowStyle)
	assert.NotNil(t, table.styleConverter)
	assert.NotNil(t, table.Component.RenderFunc)
}

func TestWithHeader(t *testing.T) {
	table := NewTable()

	// Test adding header with custom style
	table.WithHeader("Name", "w-1/3 text-left")

	assert.Len(t, table.Columns, 1)
	assert.Equal(t, "Name", table.Columns[0].Label)
	assert.Equal(t, "Name", table.Columns[0].DataKey)
	assert.Equal(t, "w-1/3 text-left", table.Columns[0].Style)

	// Test adding header without style (should use default)
	table.WithHeader("Status", "")

	assert.Len(t, table.Columns, 2)
	assert.Equal(t, "Status", table.Columns[1].Label)
	assert.Equal(t, "Status", table.Columns[1].DataKey)
	assert.Equal(t, "text-sm text-gray-600 text-left align-middle", table.Columns[1].Style)
}

func TestWithHeaders(t *testing.T) {
	table := NewTable()

	// Test adding multiple headers
	table.WithHeaders("Name", "Status", "Created", "Updated")

	assert.Len(t, table.Columns, 4)

	expectedHeaders := []string{"Name", "Status", "Created", "Updated"}
	for i, expected := range expectedHeaders {
		assert.Equal(t, expected, table.Columns[i].Label)
		assert.Equal(t, expected, table.Columns[i].DataKey)
		assert.Equal(t, "text-sm text-gray-600 text-left align-middle", table.Columns[i].Style)
	}
}

func TestWithRows(t *testing.T) {
	table := NewTable().
		WithHeaders("Name", "Status", "Created")

	// Test data that matches all columns
	data := []map[string]any{
		{"Name": "User1", "Status": "Active", "Created": "2024-01-01"},
		{"Name": "User2", "Status": "Inactive", "Created": "2024-01-02"},
	}

	table.WithRows(data)

	assert.Len(t, table.Rows, 2)
	assert.Equal(t, []any{"User1", "Active", "2024-01-01"}, table.Rows[0])
	assert.Equal(t, []any{"User2", "Inactive", "2024-01-02"}, table.Rows[1])
}

func TestWithRowsPartialData(t *testing.T) {
	table := NewTable().
		WithHeaders("Name", "Status", "Created", "Updated")

	// Test data that only has some columns
	data := []map[string]any{
		{"Name": "User1", "Status": "Active"},
		{"Name": "User2", "Created": "2024-01-02"},
	}

	table.WithRows(data)

	assert.Len(t, table.Rows, 2)
	// Missing values should default to empty string
	assert.Equal(t, []any{"User1", "Active", "", ""}, table.Rows[0])
	assert.Equal(t, []any{"User2", "", "2024-01-02", ""}, table.Rows[1])
}

func TestWithRowsWithDataKey(t *testing.T) {
	table := NewTable()

	// Add headers with custom data keys
	table.WithHeader("User Name", "text-left").
		WithHeader("User Status", "text-center")

	// Manually set different data keys
	table.Columns[0].DataKey = "name"
	table.Columns[1].DataKey = "status"

	data := []map[string]any{
		{"name": "John Doe", "status": "Active"},
		{"name": "Jane Smith", "status": "Inactive"},
	}

	table.WithRows(data)

	assert.Len(t, table.Rows, 2)
	assert.Equal(t, []any{"John Doe", "Active"}, table.Rows[0])
	assert.Equal(t, []any{"Jane Smith", "Inactive"}, table.Rows[1])
}

func TestWithRowSlice(t *testing.T) {
	table := NewTable().
		WithHeaders("Name", "Status", "Created")

	// Add individual rows
	table.WithRowSlice([]any{"User1", "Active", "2024-01-01"}).
		WithRowSlice([]any{"User2", "Inactive", "2024-01-02"})

	assert.Len(t, table.Rows, 2)
	assert.Equal(t, []any{"User1", "Active", "2024-01-01"}, table.Rows[0])
	assert.Equal(t, []any{"User2", "Inactive", "2024-01-02"}, table.Rows[1])
}

func TestFluentAPIChaining(t *testing.T) {
	// Test the complete fluent API in one chain
	table := NewTable().
		WithHeader("Name", "w-1/3 text-left").
		WithHeader("Status", "w-1/3 text-center").
		WithHeaders("Created", "Updated").
		WithRows([]map[string]any{
			{"Name": "User1", "Status": "Active", "Created": "2024-01-01", "Updated": "2024-01-15"},
			{"Name": "User2", "Status": "Inactive", "Created": "2024-01-02", "Updated": "2024-01-16"},
		}).
		WithRowSlice([]any{"User3", "Pending", "2024-01-03", "2024-01-17"}).
		WithHeaderStyle("font-bold text-white bg-green-600").
		WithBorders(true)

	// Verify structure
	assert.Len(t, table.Columns, 4)
	assert.Len(t, table.Rows, 3)

	// Verify headers
	assert.Equal(t, "Name", table.Columns[0].Label)
	assert.Equal(t, "w-1/3 text-left", table.Columns[0].Style)
	assert.Equal(t, "Status", table.Columns[1].Label)
	assert.Equal(t, "w-1/3 text-center", table.Columns[1].Style)
	assert.Equal(t, "Created", table.Columns[2].Label)
	assert.Equal(t, "Updated", table.Columns[3].Label)

	// Verify rows
	assert.Equal(t, []any{"User1", "Active", "2024-01-01", "2024-01-15"}, table.Rows[0])
	assert.Equal(t, []any{"User2", "Inactive", "2024-01-02", "2024-01-16"}, table.Rows[1])
	assert.Equal(t, []any{"User3", "Pending", "2024-01-03", "2024-01-17"}, table.Rows[2])

	// Verify styling
	assert.Equal(t, "font-bold text-white bg-green-600", table.HeaderStyle)
	assert.True(t, table.ShowBorders)
}

func TestEmptyTable(t *testing.T) {
	table := NewTable()

	// Test that empty table is valid
	assert.NotNil(t, table)
	assert.Empty(t, table.Columns)
	assert.Empty(t, table.Rows)
}

func TestWithRowsEmptyColumns(t *testing.T) {
	table := NewTable()

	// Adding rows without columns should work but result in empty rows
	data := []map[string]any{
		{"Name": "User1", "Status": "Active"},
	}

	table.WithRows(data)

	assert.Empty(t, table.Columns)
	assert.Len(t, table.Rows, 1)
	assert.Empty(t, table.Rows[0]) // Should be empty since no columns defined
}

func TestMethodChaining(t *testing.T) {
	// Test that all methods return the table for chaining
	table := NewTable()

	result := table.WithHeader("Test", "").
		WithHeaders("Col1", "Col2").
		WithRows([]map[string]any{}).
		WithRowSlice([]any{}).
		WithStyle("test-style").
		WithCellPadding("p-4").
		WithHeaderStyle("header-style").
		WithRowStyle("row-style").
		WithAlternateRowStyle("alt-style").
		WithCompactMode(true).
		WithBorders(false)

	// All methods should return the same table instance
	assert.Same(t, table, result)
}
