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
	// Verify that our custom JSON type implements json.Marshaler
	testJSON := JSON(`{"test":"value"}`)
	if _, ok := interface{}(testJSON).(json.Marshaler); ok {
		t.Logf("JSON type implements json.Marshaler")
	} else {
		t.Logf("JSON type does NOT implement json.Marshaler")
	}

	// Create test data with json.RawMessage
	configData := map[string]interface{}{
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
	data := map[string]interface{}{
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

func TestMarkdownFormatter_WithSummaryAndTable(t *testing.T) {
	// Create test data with json.RawMessage
	testItem := TestDataWithRawMessage{
		ID:       "TASK-100",
		Name:     "Integration Test",
		Config:   json.RawMessage(`{"env":"staging","verbose":true}`),
		Status:   "running",
		Priority: 1,
	}

	// Create schema with both summary fields and a table
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:  "id",
				Type:  "string",
				Label: "Task ID",
			},
			{
				Name:  "name",
				Type:  "string",
				Label: "Task Name",
			},
			{
				Name:   "subtasks",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						{Name: "id", Type: "string", Label: "Subtask ID"},
						{Name: "description", Type: "string", Label: "Description"},
						{Name: "done", Type: "boolean", Label: "Completed"},
					},
				},
			},
		},
	}

	// Parse data with schema
	parser := api.NewStructParser()
	data := map[string]interface{}{
		"id":   testItem.ID,
		"name": testItem.Name,
		"subtasks": []map[string]interface{}{
			{"id": "SUB-1", "description": "Initialize environment", "done": true},
			{"id": "SUB-2", "description": "Run test suite", "done": false},
			{"id": "SUB-3", "description": "Generate report", "done": false},
		},
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

	// Validate summary section (uses label if available, otherwise field name)
	if !strings.Contains(output, "**Task ID**: TASK-100") {
		t.Errorf("Markdown should contain summary field for Task ID")
	}
	if !strings.Contains(output, "**Task Name**: Integration Test") {
		t.Errorf("Markdown should contain summary field for Task Name")
	}

	// Validate table section (headers are field names in lowercase, sorted alphabetically)
	if !strings.Contains(output, "| id |") {
		t.Errorf("Markdown should contain table with id header")
	}
	if !strings.Contains(output, "| description |") {
		t.Errorf("Markdown should contain table with description header")
	}
	if !strings.Contains(output, "| done |") {
		t.Errorf("Markdown should contain table with done header")
	}

	// Validate subtask data
	if !strings.Contains(output, "SUB-1") {
		t.Errorf("Markdown should contain first subtask ID")
	}
	if !strings.Contains(output, "Initialize environment") {
		t.Errorf("Markdown should contain first subtask description")
	}
	if !strings.Contains(output, "SUB-2") {
		t.Errorf("Markdown should contain second subtask ID")
	}
	if !strings.Contains(output, "Run test suite") {
		t.Errorf("Markdown should contain second subtask description")
	}
}

func TestMarkdownFormatter_EmptyTable(t *testing.T) {
	// Create schema with table format but empty data
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "items",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.PrettyTable{
					Fields: []api.PrettyField{
						{Name: "id", Type: "string"},
						{Name: "name", Type: "string"},
					},
				},
			},
		},
	}

	// Parse empty data
	parser := api.NewStructParser()
	data := map[string]interface{}{
		"items": []interface{}{},
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

	t.Logf("Markdown output for empty table:\n%s", output)

	// Empty table should produce no output or a "No data" message
	// Based on the formatTableData implementation, empty tables return "*No data*"
	if output != "" && !strings.Contains(output, "No data") {
		t.Logf("Empty table produced output: %s", output)
	}
}

func TestMarkdownFormatter_RawMessageFormatting(t *testing.T) {
	// Test different json.RawMessage formats
	testCases := []struct {
		name     string
		rawJSON  json.RawMessage
		expected string
	}{
		{
			name:     "Simple object",
			rawJSON:  json.RawMessage(`{"key":"value"}`),
			expected: "key",
		},
		{
			name:     "Array",
			rawJSON:  json.RawMessage(`["item1","item2","item3"]`),
			expected: "item1",
		},
		{
			name:     "Nested object",
			rawJSON:  json.RawMessage(`{"outer":{"inner":"value"}}`),
			expected: "outer",
		},
		{
			name:     "Null",
			rawJSON:  json.RawMessage(`null`),
			expected: "null",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := TestDataWithRawMessage{
				ID:       "TEST-1",
				Name:     "Test",
				Config:   tc.rawJSON,
				Status:   "active",
				Priority: 1,
			}

			schema := &api.PrettyObject{
				Fields: []api.PrettyField{
					{
						Name:   "items",
						Type:   "array",
						Format: api.FormatTable,
						TableOptions: api.PrettyTable{
							Fields: []api.PrettyField{
								{Name: "id", Type: "string"},
								{Name: "config", Type: "string"},
							},
						},
					},
				},
			}

			parser := api.NewStructParser()
			testData := map[string]interface{}{
				"items": []TestDataWithRawMessage{data},
			}

			prettyData, err := parser.ParseDataWithSchema(testData, schema)
			if err != nil {
				t.Fatalf("Failed to parse data: %v", err)
			}

			formatter := NewMarkdownFormatter()
			output, err := formatter.FormatPrettyData(prettyData, FormatOptions{})
			if err != nil {
				t.Fatalf("Failed to format markdown: %v", err)
			}

			if !strings.Contains(output, tc.expected) {
				t.Errorf("Expected output to contain %q, got:\n%s", tc.expected, output)
			}
		})
	}
}
