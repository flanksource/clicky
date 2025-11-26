package formatters

import (
	"fmt"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	. "github.com/flanksource/clicky/formatters"
	"github.com/stretchr/testify/assert"
)

// SampleUser demonstrates a struct implementing PrettyRow for custom table formatting
type SampleUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Active   bool   `json:"active"`
}

// PrettyRow implements the PrettyRow interface for custom table formatting
func (u SampleUser) PrettyRow(opts interface{}) map[string]api.Text {
	result := make(map[string]api.Text)

	// ID with blue styling
	result["ID"] = api.Text{Content: string(rune(u.ID + '0')), Style: "text-blue-600 font-mono"}

	// Username with bold styling
	result["Username"] = api.Text{Content: u.Username, Style: "font-bold"}

	// Email with subtle styling
	result["Email"] = api.Text{Content: u.Email, Style: "text-gray-600"}

	// Active status with conditional coloring
	statusText := "Active"
	statusStyle := "text-green-600 font-medium"
	if !u.Active {
		statusText = "Inactive"
		statusStyle = "text-red-600 font-medium"
	}

	// Check for NoColor option
	if opts != nil {
		if noColorOpts, ok := opts.(FormatOptions); ok && noColorOpts.NoColor {
			// Remove colors when NoColor is set
			result["ID"] = api.Text{Content: string(rune(u.ID + '0')), Style: "font-mono"}
			result["Username"] = api.Text{Content: u.Username, Style: "font-bold"}
			result["Email"] = api.Text{Content: u.Email, Style: ""}
			statusStyle = "font-medium"
		}
	}

	result["Status"] = api.Text{Content: statusText, Style: statusStyle}

	return result
}

func TestPrettyRowIntegrationWithMarkdown(t *testing.T) {
	users := []SampleUser{
		{ID: 1, Username: "alice", Email: "alice@example.com", Active: true},
		{ID: 2, Username: "bob", Email: "bob@example.com", Active: false},
		{ID: 3, Username: "charlie", Email: "charlie@example.com", Active: true},
	}

	// Format with markdown using the new PrettyRow interface
	manager := NewFormatManager()
	opts := FormatOptions{Format: "markdown", NoColor: false}

	result, err := manager.FormatWithOptions(opts, users)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	// Verify that the output contains the expected table structure
	assert.True(t, strings.Contains(result, "| ID |"))
	assert.True(t, strings.Contains(result, "| Username |"))
	assert.True(t, strings.Contains(result, "| Email |"))
	assert.True(t, strings.Contains(result, "| Status |"))

	// Verify that custom content appears (usernames and emails)
	assert.True(t, strings.Contains(result, "alice"))
	assert.True(t, strings.Contains(result, "bob"))
	assert.True(t, strings.Contains(result, "charlie"))
	assert.True(t, strings.Contains(result, "alice@example.com"))

	// Verify status formatting
	assert.True(t, strings.Contains(result, "Active"))
	assert.True(t, strings.Contains(result, "Inactive"))
}

func TestPrettyRowIntegrationWithNoColor(t *testing.T) {
	users := []SampleUser{
		{ID: 1, Username: "test", Email: "test@example.com", Active: false},
	}

	manager := NewFormatManager()
	opts := FormatOptions{Format: "markdown", NoColor: true}

	result, err := manager.FormatWithOptions(opts, users)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	// The output should still contain the data but with NoColor applied
	assert.True(t, strings.Contains(result, "test"))
	assert.True(t, strings.Contains(result, "test@example.com"))
	assert.True(t, strings.Contains(result, "Inactive"))
}

func TestRegularStructWithoutPrettyRowInterface(t *testing.T) {
	// Regular struct without PrettyRow interface should still work
	type RegularStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := []RegularStruct{
		{Name: "item1", Value: 100},
		{Name: "item2", Value: 200},
	}

	manager := NewFormatManager()
	opts := FormatOptions{Format: "markdown"}

	result, err := manager.FormatWithOptions(opts, data)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	// Should still generate a table with reflection-based approach
	assert.True(t, strings.Contains(result, "item1"))
	assert.True(t, strings.Contains(result, "item2"))
	assert.True(t, strings.Contains(result, "100"))
	assert.True(t, strings.Contains(result, "200"))
}

// OrderedProduct demonstrates column ordering using order-X Tailwind styles
type OrderedProduct struct {
	Name     string
	Price    float64
	Category string
	SKU      string
}

// PrettyRow implements PrettyRow with explicit column ordering
func (p OrderedProduct) PrettyRow(_ interface{}) map[string]api.Text {
	return map[string]api.Text{
		// SKU should appear first (no order = order-0)
		"SKU": {Content: p.SKU, Style: "font-mono"},
		// Name should appear second (order-1)
		"Name": {Content: p.Name, Style: "font-bold order-1"},
		// Category should appear third (order-2)
		"Category": {Content: p.Category, Style: "text-gray-600 order-2"},
		// Price should appear fourth (order-3)
		"Price": {Content: fmt.Sprintf("$%.2f", p.Price), Style: "text-green-600 order-3"},
	}
}

func TestPrettyRowColumnOrdering(t *testing.T) {
	products := []OrderedProduct{
		{SKU: "PROD-001", Name: "Widget", Category: "Tools", Price: 29.99},
		{SKU: "PROD-002", Name: "Gadget", Category: "Electronics", Price: 49.99},
	}

	manager := NewFormatManager()
	opts := FormatOptions{Format: "markdown", NoColor: false}

	result, err := manager.FormatWithOptions(opts, products)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	// Debug: print the actual result
	t.Logf("Formatted output:\n%s", result)

	// Extract the header line
	lines := strings.Split(result, "\n")
	var headerLine string
	for _, line := range lines {
		if strings.HasPrefix(line, "|") && strings.Contains(line, "SKU") {
			headerLine = line
			break
		}
	}

	assert.NotEmpty(t, headerLine, "Header line not found in output")
	t.Logf("Header line: %s", headerLine)

	// Verify column order: SKU (order-0), Name (order-1), Category (order-2), Price (order-3)
	// Find position of each column in the header (case-insensitive)
	headerUpper := strings.ToUpper(headerLine)
	skuPos := strings.Index(headerUpper, "SKU")
	namePos := strings.Index(headerUpper, "NAME")
	categoryPos := strings.Index(headerUpper, "CATEGORY")
	pricePos := strings.Index(headerUpper, "PRICE")

	assert.Greater(t, skuPos, -1, "SKU column not found")
	assert.Greater(t, namePos, -1, "Name column not found")
	assert.Greater(t, categoryPos, -1, "Category column not found")
	assert.Greater(t, pricePos, -1, "Price column not found")

	// Verify the order: SKU < Name < Category < Price
	assert.Less(t, skuPos, namePos, "SKU should appear before Name (got positions: SKU=%d, Name=%d)", skuPos, namePos)
	assert.Less(t, namePos, categoryPos, "Name should appear before Category (got positions: Name=%d, Category=%d)", namePos, categoryPos)
	assert.Less(t, categoryPos, pricePos, "Category should appear before Price (got positions: Category=%d, Price=%d)", categoryPos, pricePos)
}
