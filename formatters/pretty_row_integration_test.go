package formatters

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
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
