package formatters

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/api"
)

// JSON is a custom type alias for json.RawMessage
type JSON json.RawMessage

// TestDataWithRawMessage represents a test struct with json.RawMessage field
type TestDataWithRawMessage struct {
	ID       string          `json:"id"`
	Config   json.RawMessage `json:"config"`
	Metadata JSON            `json:"metadata"`
	Priority int             `json:"priority"`
}

func TestMarkdownFormatter_SimpleTable(t *testing.T) {
	g := NewWithT(t)

	configData := map[string]any{
		"timeout": 30,
		"retries": 3,
		"enabled": true,
	}
	configJSON, err := json.Marshal(configData)
	g.Expect(err).ToNot(HaveOccurred())

	testData := []struct {
		testStruct TestDataWithRawMessage
		expected   string
	}{
		{
			testStruct: TestDataWithRawMessage{
				ID:       "TASK-001",
				Config:   configJSON,
				Metadata: JSON(`{"author":"alice","version":"1.0"}`),
				Priority: 1,
			},
			expected: `| config | id | metadata | priority | 
| --- | --- | --- | --- | 
| {"enabled":true,"retries":3,"timeout":30} | TASK-001 | {"author":"alice","version":"1.0"} | 1 | 
`,
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
						{Name: "config", Type: "string", Label: "Configuration"},
						{Name: "metadata", Type: "string", Label: "Metadata"},
						{Name: "priority", Type: "int", Label: "Priority"},
					},
				},
			},
		},
	}

	for _, tc := range testData {
		// Parse data with schema
		parser := api.NewStructParser()
		data := map[string]any{
			"tasks": []TestDataWithRawMessage{tc.testStruct},
		}

		prettyData, err := parser.ParseDataWithSchema(data, schema)
		g.Expect(err).ToNot(HaveOccurred())

		// Format as markdown
		formatter := NewMarkdownFormatter()
		output, err := formatter.FormatPrettyData(prettyData, FormatOptions{})
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(output).To(Equal(tc.expected))
	}
}
