package formatters

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
)

// JSON is a custom type alias for json.RawMessage
type JSON json.RawMessage

// TestDataWithRawMessage represents a test struct with json.RawMessage field
type TestDataWithRawMessage struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
	Metadata JSON            `json:"metadata"`
	Status   string          `json:"status"`
	Priority int             `json:"priority"`
}

func TestMarkdownFormatter_SimpleTable(t *testing.T) {
	configData := map[string]any{
		"timeout": 30,
		"retries": 3,
		"enabled": true,
	}
	configJSON, err := json.Marshal(configData)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	testData := []TestDataWithRawMessage{
		{
			ID:       "TASK-001",
			Name:     "Deploy Application",
			Config:   configJSON,
			Metadata: JSON(`{"author":"alice","version":"1.0"}`),
			Status:   "active",
			Priority: 1,
		},
		{
			ID:       "TASK-002",
			Name:     "Run Tests",
			Config:   json.RawMessage(`{"parallel":true,"workers":4}`),
			Metadata: JSON(`{"author":"bob","version":"2.0"}`),
			Status:   "pending",
			Priority: 2,
		},
		{
			ID:       "TASK-003",
			Name:     "Update Documentation",
			Config:   json.RawMessage(`{"format":"markdown","publish":true}`),
			Metadata: JSON(`{"author":"charlie","version":"3.0"}`),
			Status:   "completed",
			Priority: 3,
		},
	}

	// Create schema with table format
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "tasks",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						{Name: "id", Type: "string", Label: "ID"},
						{Name: "name", Type: "string", Label: "Task Name"},
						{Name: "config", Type: "string", Label: "Configuration"},
						{Name: "metadata", Type: "string", Label: "Metadata"},
						{Name: "status", Type: "string", Label: "Status"},
						{Name: "priority", Type: "int", Label: "Priority"},
					},
				},
			},
		},
	}

	// Parse data with schema
	parser := api.NewStructParser()
	data := map[string]any{
		"tasks": testData,
	}

	prettyData, err := parser.ParseDataWithSchema(data, schema)
	if err != nil {
		t.Fatalf("Failed to parse data with schema: %v", err)
	}

	// Format as markdown
	formatter := NewMarkdownFormatter()
	output, err := formatter.FormatPrettyData(prettyData, FormatOptions{})
	if err != nil {
		t.Fatalf("Failed to format markdown: %v", err)
	}

	t.Logf("Markdown output:\n%s", output)

	// Validate markdown table structure (headers are field names in lowercase, sorted alphabetically)
	if !strings.Contains(output, "| config |") {
		t.Errorf("Markdown table should contain config header")
	}
	if !strings.Contains(output, "| id |") {
		t.Errorf("Markdown table should contain id header")
	}
	if !strings.Contains(output, "| metadata |") {
		t.Errorf("Markdown table should contain metadata header")
	}
	if !strings.Contains(output, "| name |") {
		t.Errorf("Markdown table should contain name header")
	}
	if !strings.Contains(output, "| status |") {
		t.Errorf("Markdown table should contain status header")
	}
	if !strings.Contains(output, "| priority |") {
		t.Errorf("Markdown table should contain priority header")
	}

	// Validate table separator
	if !strings.Contains(output, "| --- |") {
		t.Errorf("Markdown table should contain separator row")
	}

	// Validate data rows
	if !strings.Contains(output, "TASK-001") {
		t.Errorf("Markdown table should contain first task ID")
	}
	if !strings.Contains(output, "Deploy Application") {
		t.Errorf("Markdown table should contain first task name")
	}
	if !strings.Contains(output, "TASK-002") {
		t.Errorf("Markdown table should contain second task ID")
	}
	if !strings.Contains(output, "Run Tests") {
		t.Errorf("Markdown table should contain second task name")
	}
	if !strings.Contains(output, "TASK-003") {
		t.Errorf("Markdown table should contain third task ID")
	}
	if !strings.Contains(output, "Update Documentation") {
		t.Errorf("Markdown table should contain third task name")
	}

	// Validate json.RawMessage is formatted as JSON string
	if !strings.Contains(output, "\"timeout\"") || !strings.Contains(output, "\"retries\"") {
		t.Errorf("Markdown table should contain config data from json.RawMessage as JSON string")
	}
	if !strings.Contains(output, `{"enabled":true,"retries":3,"timeout":30}`) {
		t.Errorf("Markdown table should contain properly formatted JSON from RawMessage")
	}

	// Validate custom JSON type (type alias of json.RawMessage) is also formatted as JSON string
	if !strings.Contains(output, "\"author\"") || !strings.Contains(output, "\"version\"") {
		t.Errorf("Markdown table should contain metadata from custom JSON type as JSON string")
	}
	if !strings.Contains(output, `{"author":"alice","version":"1.0"}`) {
		t.Errorf("Markdown table should contain properly formatted JSON from custom JSON type")
	}

	// Validate status values
	if !strings.Contains(output, "active") {
		t.Errorf("Markdown table should contain active status")
	}
	if !strings.Contains(output, "pending") {
		t.Errorf("Markdown table should contain pending status")
	}
	if !strings.Contains(output, "completed") {
		t.Errorf("Markdown table should contain completed status")
	}
}
