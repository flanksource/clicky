package formatters

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
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
		It("should contain import map with react and facet", func() {
			table := api.TextTable{
				Headers: api.TextList{api.Text{Content: "Name"}},
				Rows:    []api.TableRow{{"Name": api.TypedValue{Textable: api.Text{Content: "test"}}}},
			}
			output, err := formatter.Format(table, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("text/babel"))
			Expect(output).To(ContainSubstring("react.production.min.js"))
			Expect(output).To(ContainSubstring("babel.min.js"))
		})

		It("should embed data in clicky-data script tag", func() {
			table := api.TextTable{
				Headers: api.TextList{api.Text{Content: "Name"}},
				Rows:    []api.TableRow{{"Name": api.TypedValue{Textable: api.Text{Content: "hello"}}}},
			}
			output, err := formatter.Format(table, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(`id="clicky-data"`))
			Expect(output).To(ContainSubstring(`"type":"table"`))
			Expect(output).To(ContainSubstring("hello"))
		})

		It("should include root div", func() {
			output, err := formatter.Format(api.Text{Content: "test"}, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(`id="root"`))
		})
	})

	Describe("Data conversion", func() {
		It("should convert TextTable to react table data", func() {
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

		It("should convert TextTree to react tree data", func() {
			tree := api.TextTree{
				Node: api.Text{Content: "root"},
				Children: []api.TextTree{
					{Node: api.Text{Content: "child1"}},
					{Node: api.Text{Content: "child2"}},
				},
			}
			output, err := formatter.Format(tree, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(`"type":"tree"`))
			Expect(output).To(ContainSubstring("root"))
			Expect(output).To(ContainSubstring("child1"))
		})

		It("should convert simple text", func() {
			output, err := formatter.Format(api.Text{Content: "hello world"}, FormatOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("hello world"))
		})
	})

	Describe("Custom component", func() {
		It("should embed custom component source when provided", func() {
			customJSX := `function App({ data }) { return <div className="p-4"><h1>Custom</h1><pre>{JSON.stringify(data)}</pre></div>; }`
			output, err := formatter.Format(
				api.Text{Content: "test"},
				FormatOptions{ReactComponent: customJSX},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(customJSX))
			Expect(output).To(ContainSubstring("text/babel"))
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
			Expect(output).To(ContainSubstring("text/babel"))
		})
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
	}

	rd := convertTable(table, nil)
	if rd.Type != "table" {
		t.Fatalf("expected type=table, got %s", rd.Type)
	}
	if len(rd.Table.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(rd.Table.Columns))
	}
	if len(rd.Table.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rd.Table.Rows))
	}
	if rd.Table.Rows[0]["Name"].Text != "svc-a" {
		t.Errorf("expected row[0][Name]=svc-a, got %s", rd.Table.Rows[0]["Name"].Text)
	}
}

func TestHTMLReactConvertTree(t *testing.T) {
	tree := &api.TextTree{
		Node: api.Text{Content: "root"},
		Children: []api.TextTree{
			{Node: api.Text{Content: "child"}},
		},
	}

	rt := convertTree(tree)
	if rt.Label != "root" {
		t.Fatalf("expected label=root, got %s", rt.Label)
	}
	if len(rt.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(rt.Children))
	}
	if rt.Children[0].Label != "child" {
		t.Errorf("expected child label=child, got %s", rt.Children[0].Label)
	}
}

func TestHTMLReactBuildHTML(t *testing.T) {
	html := buildReactHTML(`{"type":"text","text":"hello"}`, "")

	checks := []string{
		"clicky-data",
		"text/babel",
		"react.production.min.js",
		"babel.min.js",
		`id="root"`,
		"tailwindcss.com",
		"iconify",
	}
	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("expected HTML to contain %q", check)
		}
	}
}
