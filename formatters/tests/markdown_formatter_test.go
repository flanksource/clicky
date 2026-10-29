package formatters

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	clickyformatters "github.com/flanksource/clicky/formatters"
	"github.com/flanksource/clicky/markdown"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

func TestFormatters(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Formatters Suite")
}

func TestSlackFormatter(t *testing.T) {
	g := NewWithT(t)
	parseMarkdown := func(source string) api.Textable {
		doc, err := markdown.ParseString(source)
		g.Expect(err).NotTo(HaveOccurred())
		return doc
	}
	type Alert struct {
		Title   string          `pretty:"title"`
		Org     string          `pretty:"label=org"`
		Divider api.HtmlElement `pretty:"label=divider"`
		Summary api.Textable    `pretty:"title=Summary:"`
		Fixes   api.List        `pretty:"title=Recommended Fix:"`
		Also    api.List        `pretty:"title=Also Affected:"`
		Actions api.ButtonGroup `pretty:"label=Actions"`
	}

	message := Alert{
		Title:   "GitHub Action 'Create Release' Failing 🚨",
		Org:     "flanksource",
		Divider: api.HR,
		Summary: parseMarkdown("The Create Release GitHub Actions workflow for `config-db` is failing. A process within the workflow is exiting with code 2, which is blocking the creation of new software releases."),
		Fixes: api.List{
			Bullet: clicky.Text("• "),
			Items: []api.Textable{
				clicky.Text("Check GitHub Actions logs for the exact failing step.", "font-semibold"),
				parseMarkdown("Review the `.github/workflows/release.yml` script for errors."),
				parseMarkdown("Verify repository secrets and `GITHUB_TOKEN` permissions."),
			},
		},
		Also: api.List{
			Bullet: clicky.Text("- "),
			Items: []api.Textable{
				clicky.Text("/GitHubAction::Workflow/config-db/Create Release"),
			},
		},
		Actions: api.ButtonGroup{
			Buttons: []api.Button{
				{Label: "View Config", Href: "https://example.com/config", Variant: "primary"},
				{Label: "Silence", Href: "https://example.com/silence", Variant: "secondary"},
			},
		},
	}

	formatter := clickyformatters.NewSlackFormatter()
	out, err := formatter.Format(message, clickyformatters.FormatOptions{})
	g.Expect(err).NotTo(HaveOccurred())

	expectedPath := filepath.Join("testdata", "slack-msg.json")
	expectedBytes, err := os.ReadFile(expectedPath)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(out).To(MatchJSON(expectedBytes))
}
