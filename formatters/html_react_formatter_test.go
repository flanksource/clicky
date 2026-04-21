package formatters

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/icons"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTMLReactFormatter", func() {
	var formatter *HTMLReactFormatter

	BeforeEach(func() {
		formatter = &HTMLReactFormatter{}
	})

	Describe("Registration", func() {
		It("should be registered as html-react custom formatter", func() {
			fn, exists := GetCustomFormatter("html-react")
			Expect(exists).To(BeTrue())
			Expect(fn).NotTo(BeNil())
		})
	})

	Describe("HTML structure", func() {
		It("should contain the clicky-ui shell", func() {
			table := api.TextTable{
				Headers: api.TextList{api.Text{Content: "Name"}},
				Rows:    []api.TableRow{{"Name": api.TypedValue{Textable: api.Text{Content: "test"}}}},
			}
			output, err := formatter.Format(table, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("@flanksource/clicky-ui"))
			Expect(output).To(ContainSubstring("importmap"))
			Expect(output).To(ContainSubstring(`data-theme="light"`))
			Expect(output).To(ContainSubstring("tokens.css"))
		})

		It("should embed the versioned clicky payload", func() {
			table := api.TextTable{
				Headers: api.TextList{api.Text{Content: "Name"}},
				Rows:    []api.TableRow{{"Name": api.TypedValue{Textable: api.Text{Content: "hello"}}}},
			}
			output, err := formatter.Format(table, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(`id="clicky-data"`))
			Expect(output).To(ContainSubstring(`"version":1`))
			Expect(output).To(ContainSubstring(`"kind":"table"`))
			Expect(output).To(ContainSubstring("hello"))
		})

		It("should include root div", func() {
			output, err := formatter.Format(api.Text{Content: "test"}, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(`id="root"`))
		})
	})

	Describe("Data conversion", func() {
		It("should convert TextTable to clicky table data", func() {
			table := api.TextTable{
				Headers: api.TextList{
					api.Text{Content: "Name"},
					api.Text{Content: "Age"},
				},
				Rows: []api.TableRow{
					{
						"Name": api.TypedValue{Textable: api.Text{Content: "Alice"}},
						"Age":  api.TypedValue{Textable: api.Text{Content: "30"}},
					},
				},
			}
			output, err := formatter.Format(table, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("Alice"))
			Expect(output).To(ContainSubstring("30"))
		})

		It("should convert TextTree to clicky tree data", func() {
			tree := api.TextTree{
				Node: api.Text{Content: "root"},
				Children: []api.TextTree{
					{Node: api.Text{Content: "child1"}},
					{Node: api.Text{Content: "child2"}},
				},
			}
			output, err := formatter.Format(tree, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(`"kind":"tree"`))
			Expect(output).To(ContainSubstring("root"))
			Expect(output).To(ContainSubstring("child1"))
		})

		It("should convert simple text", func() {
			output, err := formatter.Format(api.Text{Content: "hello world"}, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("hello world"))
			Expect(output).To(ContainSubstring(`"kind":"text"`))
		})
	})

	Describe("Empty data", func() {
		It("should return valid HTML for empty text input", func() {
			output, err := formatter.Format(api.Text{Content: ""}, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(`id="root"`))
		})
	})

	Describe("Format manager integration", func() {
		It("should be accessible via FormatManager.Format", func() {
			fm := NewFormatManager()
			table := api.TextTable{
				Headers: api.TextList{api.Text{Content: "Col"}},
				Rows:    []api.TableRow{{"Col": api.TypedValue{Textable: api.Text{Content: "val"}}}},
			}
			output, err := fm.Format("html-react", table)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("@flanksource/clicky-ui"))
		})
	})
})

var _ = Describe("ClickyJSONFormatter", func() {
	var formatter *ClickyJSONFormatter

	BeforeEach(func() {
		formatter = &ClickyJSONFormatter{}
	})

	It("should be registered as clicky-json custom formatter", func() {
		fn, exists := GetCustomFormatter("clicky-json")
		Expect(exists).To(BeTrue())
		Expect(fn).NotTo(BeNil())
	})

	It("should emit the clicky document JSON without an HTML shell", func() {
		table := api.TextTable{
			Headers: api.TextList{api.Text{Content: "Name"}},
			Rows:    []api.TableRow{{"Name": api.TypedValue{Textable: api.Text{Content: "svc-a"}}}},
		}
		output, err := formatter.Format(table, FormatOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring(`"version": 1`))
		Expect(output).To(ContainSubstring(`"kind": "table"`))
		Expect(output).To(ContainSubstring("svc-a"))
		Expect(output).NotTo(ContainSubstring("<!doctype html>"))
		Expect(output).NotTo(ContainSubstring("@flanksource/clicky-ui"))
	})

	It("should produce the same payload that html-react embeds", func() {
		data := api.Text{Content: "hello"}
		jsonOut, err := formatter.Format(data, FormatOptions{})
		Expect(err).NotTo(HaveOccurred())

		htmlOut, err := (&HTMLReactFormatter{}).Format(data, FormatOptions{})
		Expect(err).NotTo(HaveOccurred())

		// The html-react shell embeds the compact (unindented) form, while
		// clicky-json is indented. Compare the parsed structure instead.
		var fromJSON, fromHTML map[string]any
		Expect(json.Unmarshal([]byte(jsonOut), &fromJSON)).To(Succeed())

		start := strings.Index(htmlOut, `<script id="clicky-data" type="application/json">`)
		Expect(start).To(BeNumerically(">=", 0))
		start += len(`<script id="clicky-data" type="application/json">`)
		end := strings.Index(htmlOut[start:], "</script>")
		Expect(end).To(BeNumerically(">", 0))
		Expect(json.Unmarshal([]byte(htmlOut[start:start+end]), &fromHTML)).To(Succeed())

		Expect(fromJSON).To(Equal(fromHTML))
	})
})

func TestHTMLReactConvertTable(t *testing.T) {
	table := &api.TextTable{
		Headers: api.TextList{
			api.Text{Content: "Name"},
			api.Text{Content: "Status"},
		},
		Rows: []api.TableRow{
			{
				"Name":   api.TypedValue{Textable: api.Text{Content: "svc-a"}},
				"Status": api.TypedValue{Textable: api.Text{Content: "healthy"}},
			},
			{
				"Name":   api.TypedValue{Textable: api.Text{Content: "svc-b"}},
				"Status": api.TypedValue{Textable: api.Text{Content: "degraded"}},
			},
		},
		RowDetail: []api.Textable{
			api.Code{Content: "SELECT 1", Language: "sql"},
			nil,
		},
	}

	node := convertTable(table)
	if node.Kind != "table" {
		t.Fatalf("expected kind=table, got %s", node.Kind)
	}
	if len(node.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(node.Columns))
	}
	if len(node.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(node.Rows))
	}
	if node.Rows[0].Cells["Name"].Plain != "svc-a" {
		t.Errorf("expected row[0][Name]=svc-a, got %s", node.Rows[0].Cells["Name"].Plain)
	}
	if node.Rows[0].Detail == nil || node.Rows[0].Detail.Kind != "code" {
		t.Fatalf("expected row detail code node, got %#v", node.Rows[0].Detail)
	}
}

func TestHTMLReactConvertTree(t *testing.T) {
	tree := &api.TextTree{
		Node: api.Text{Content: "root"},
		Children: []api.TextTree{
			{Node: api.Text{Content: "child"}},
		},
	}

	node := convertTree(tree)
	if node.Kind != "tree" {
		t.Fatalf("expected kind=tree, got %s", node.Kind)
	}
	if len(node.Roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(node.Roots))
	}
	if node.Roots[0].Label.Plain != "root" {
		t.Fatalf("expected label=root, got %s", node.Roots[0].Label.Plain)
	}
	if len(node.Roots[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(node.Roots[0].Children))
	}
	if node.Roots[0].Children[0].Label.Plain != "child" {
		t.Errorf("expected child label=child, got %s", node.Roots[0].Children[0].Label.Plain)
	}
}

func TestHTMLReactConvertRichTextables(t *testing.T) {
	text := api.Text{
		Content: "status:",
		Style:   "font-bold text-green-600",
		Children: []api.Textable{
			icons.Check,
			api.Text{Content: " healthy"},
		},
	}.WithTooltip(api.Text{Content: "all checks passing"})

	textNode := convertTextable(text)
	if textNode.Kind != "text" {
		t.Fatalf("expected text node, got %s", textNode.Kind)
	}
	if textNode.Tooltip == nil || textNode.Tooltip.Plain != "all checks passing" {
		t.Fatalf("expected tooltip to be preserved, got %#v", textNode.Tooltip)
	}
	if len(textNode.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(textNode.Children))
	}

	groupNode := convertTextable(api.ButtonGroup{
		Buttons: []api.Button{
			{Label: "Docs", Href: "https://example.com/docs"},
			{Label: "Restart", ID: "restart"},
		},
	})
	if groupNode.Kind != "button-group" || len(groupNode.Items) != 2 {
		t.Fatalf("expected button-group with 2 items, got %#v", groupNode)
	}

	mapNode := convertTextable(api.DescriptionList{
		Items: []api.KeyValuePair{
			{Key: "owner", Value: "platform"},
			{Key: "region", Value: "eu-west-1"},
		},
	})
	if mapNode.Kind != "map" || len(mapNode.Fields) != 2 {
		t.Fatalf("expected 2 map fields, got %#v", mapNode)
	}
}

func TestHTMLReactBuildHTML(t *testing.T) {
	html := buildReactHTML(`{"version":1,"node":{"kind":"text","text":"hello"}}`)

	checks := []string{
		"clicky-data",
		`id="root"`,
		"@flanksource/clicky-ui",
		"importmap",
		`data-theme="light"`,
		"tokens.css",
		"tailwindcss.com",
		"iconify",
		"prismjs",
	}
	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("expected HTML to contain %q", check)
		}
	}
}
