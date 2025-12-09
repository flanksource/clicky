package formatters

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/api"
)

func TestMarkdownFormatter_SimpleTable(t *testing.T) {
	g := NewWithT(t)

	// Create schema with table format
	schema := &api.PrettyObject{
		Fields: []api.PrettyField{
			{
				Name:   "tasks",
				Type:   "array",
				Format: api.FormatTable,
				TableOptions: api.TableOptions{
					Columns: []api.PrettyField{
						{Name: "id", Type: "string", Label: "ID"},
						{Name: "name", Type: "string", Label: "Name"},
						{Name: "priority", Type: "int", Label: "Priority"},
					},
				},
			},
		},
	}

	// Parse data with schema
	parser := api.NewStructParser()
	data := map[string]any{
		"tasks": []map[string]any{
			{"id": "TASK-001", "name": "First Task", "priority": 1},
			{"id": "TASK-002", "name": "Second Task", "priority": 2},
		},
	}

	prettyData, err := parser.ParseDataWithSchema(data, schema)
	g.Expect(err).ToNot(HaveOccurred())

	// Get table and check markdown
	table := prettyData.FirstTable()
	g.Expect(table).ToNot(BeNil())

	output := table.Markdown()

	// Should use Labels for headers
	g.Expect(output).To(ContainSubstring("ID"))
	g.Expect(output).To(ContainSubstring("NAME"))
	g.Expect(output).To(ContainSubstring("PRIORITY"))

	// Should contain data values
	g.Expect(output).To(ContainSubstring("TASK-001"))
	g.Expect(output).To(ContainSubstring("First Task"))
	g.Expect(output).To(ContainSubstring("TASK-002"))
	g.Expect(output).To(ContainSubstring("Second Task"))
}
