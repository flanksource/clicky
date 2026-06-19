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

	linkNode := convertTextable(api.NewLink("https://example.com/docs").
		Append("docs", "text-blue-600 underline"))
	if linkNode.Kind != "link" || linkNode.Href != "https://example.com/docs" {
		t.Fatalf("expected link node with href, got %#v", linkNode)
	}
	if len(linkNode.Children) != 1 || linkNode.Children[0].Plain != "docs" {
		t.Fatalf("expected inline link content, got %#v", linkNode.Children)
	}

	commandNode := convertTextable(api.NewLinkCommand("stack/get").
		WithArgs("stack-42").
		WithFlag("events", "1").
		WithTarget(api.LinkTargetDialog).
		WithAutoRun(true).
		Append("stack-42", "text-blue-600 underline"))
	if commandNode.Kind != "link-command" || commandNode.Command != "stack/get" {
		t.Fatalf("expected link-command node, got %#v", commandNode)
	}
	if len(commandNode.Args) != 1 || commandNode.Args[0] != "stack-42" {
		t.Fatalf("expected args to be preserved, got %#v", commandNode.Args)
	}
	if commandNode.Flags["events"] != "1" || !commandNode.AutoRun {
		t.Fatalf("expected command flags/autRun to be preserved, got %#v", commandNode)
	}

	headingNode := convertTextable(api.Heading{Level: 8, Content: api.Text{Content: "Report"}})
	if headingNode.Kind != "heading" || headingNode.Level != 6 {
		t.Fatalf("expected clamped heading node, got %#v", headingNode)
	}
	if headingNode.Content == nil || headingNode.Content.Plain != "Report" {
		t.Fatalf("expected heading content to be preserved, got %#v", headingNode.Content)
	}

	blockquoteNode := convertTextable(api.Blockquote{Content: api.Text{Content: "quoted text"}})
	if blockquoteNode.Kind != "blockquote" || blockquoteNode.Content == nil || blockquoteNode.Content.Plain != "quoted text" {
		t.Fatalf("expected blockquote content to be preserved, got %#v", blockquoteNode)
	}

	admonitionNode := convertTextable(api.Admonition{
		Severity: api.SeverityWarning,
		Title:    api.Text{Content: "Review"},
		Body:     api.Text{Content: "Check the disclosure."},
	})
	if admonitionNode.Kind != "admonition" || admonitionNode.Severity != "warning" {
		t.Fatalf("expected warning admonition node, got %#v", admonitionNode)
	}
	if admonitionNode.Label == nil || admonitionNode.Label.Plain != "Review" {
		t.Fatalf("expected admonition title label, got %#v", admonitionNode.Label)
	}
	if admonitionNode.Content == nil || admonitionNode.Content.Plain != "Check the disclosure." {
		t.Fatalf("expected admonition body content, got %#v", admonitionNode.Content)
	}

	refNode := convertTextable(api.FootnoteRef{ID: "cash"})
	if refNode.Kind != "footnote-ref" || refNode.ID != "cash" || refNode.Plain != "[^cash]" {
		t.Fatalf("expected footnote ref node, got %#v", refNode)
	}

	footnoteNode := convertTextable(api.Footnote{ID: "cash", Content: api.Text{Content: "Cash equivalents."}})
	if footnoteNode.Kind != "footnote" || footnoteNode.ID != "cash" {
		t.Fatalf("expected footnote node, got %#v", footnoteNode)
	}
	if footnoteNode.Content == nil || footnoteNode.Content.Plain != "Cash equivalents." {
		t.Fatalf("expected footnote content, got %#v", footnoteNode.Content)
	}

	footnotesNode := convertTextable(api.Footnotes{Items: []api.Footnote{
		{ID: "cash", Content: api.Text{Content: "Cash equivalents."}},
		{ID: "", Content: api.Text{Content: "skip"}},
	}})
	if footnotesNode.Kind != "footnotes" || len(footnotesNode.Items) != 1 {
		t.Fatalf("expected one valid footnote item, got %#v", footnotesNode)
	}
	if footnotesNode.Items[0].Kind != "footnote" || footnotesNode.Items[0].ID != "cash" {
		t.Fatalf("expected footnote item to be preserved, got %#v", footnotesNode.Items)
	}
}

func TestHTMLReactConvertStackTrace(t *testing.T) {
	trace := api.NewStackTrace()
	trace.ExceptionClass = "java.lang.RuntimeException"
	trace.Message = "boom"
	trace.Language = "java"
	trace.Frames = []api.StackFrame{
		{
			Class:           "com.example.pas.AddressScreen",
			Method:          "load",
			File:            "AddressScreen.java",
			Line:            42,
			SourceLines:     []string{"class AddressScreen {", "  void load() { throw boom; }"},
			SourceStartLine: 41,
			SourceLanguage:  "java",
		},
	}

	node := convertTextable(trace)
	if node.Kind != "stacktrace" {
		t.Fatalf("expected stacktrace node, got %s", node.Kind)
	}
	if node.ExceptionClass != "java.lang.RuntimeException" || node.Message != "boom" {
		t.Fatalf("expected exception metadata, got %#v", node)
	}
	if len(node.Frames) != 1 {
		t.Fatalf("expected one frame, got %d", len(node.Frames))
	}
	if node.Frames[0].FunctionName != "com.example.pas.AddressScreen.load" {
		t.Fatalf("expected function name to be preserved, got %q", node.Frames[0].FunctionName)
	}
	if len(node.Frames[0].SourceLines) != 2 || node.Frames[0].SourceStartLine != 41 {
		t.Fatalf("expected source context to be preserved, got %#v", node.Frames[0])
	}
}

func TestHTMLReactBuildHTML(t *testing.T) {
	html := buildReactHTML(`{"version":1,"node":{"kind":"text","text":"hello"}}`)

	checks := []string{
		"clicky-data",
		`id="root"`,
		"@flanksource/clicky-ui",
		"StackTrace",
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

// entityLinkLike mirrors models.EntityLink: it implements BOTH Pretty (which
// returns a Link) and Textable. When it is a struct field it must serialize as
// a single link node, not a {kind,guid,name} field map.
type entityLinkLike struct {
	Kind string `json:"kind"`
	GUID string `json:"guid"`
	Name string `json:"name"`
}

func (l entityLinkLike) Pretty() api.Text {
	return api.Text{}.Add(api.Link{Href: "/entity/" + l.Kind + "/" + l.GUID, Content: api.Text{Content: l.Name}})
}
func (l entityLinkLike) String() string   { return l.Name }
func (l entityLinkLike) ANSI() string     { return l.Name }
func (l entityLinkLike) HTML() string     { return l.Pretty().HTML() }
func (l entityLinkLike) Markdown() string { return l.Pretty().Markdown() }

// TestClickyJSONEntityLinkFieldRendersAsLink pins the EntityLink fix: a struct
// field whose type implements both Pretty and Textable serializes as a "link"
// node (honoring Pretty), not a nested "map" of its struct fields.
func TestClickyJSONEntityLinkFieldRendersAsLink(t *testing.T) {
	type detail struct {
		Name             string         `json:"name"`
		ClientEntityLink entityLinkLike `json:"clientEntityLink"`
	}
	d := detail{
		Name:             "Scheme",
		ClientEntityLink: entityLinkLike{Kind: "client", GUID: "abc-123", Name: "GL Scheme G0796016"},
	}

	out, err := (&ClickyJSONFormatter{}).Format(d, FormatOptions{})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.Contains(out, `"link"`) {
		t.Errorf("expected a link node, got:\n%s", out)
	}
	if !strings.Contains(out, "/entity/client/abc-123") {
		t.Errorf("expected clientEntityLink href in output, got:\n%s", out)
	}
	if !strings.Contains(out, "GL Scheme G0796016") {
		t.Errorf("expected link label in output, got:\n%s", out)
	}
	// The field must NOT degrade to a map exposing its kind/guid/name fields.
	if strings.Contains(out, `"guid"`) {
		t.Errorf("clientEntityLink leaked struct fields (rendered as a map), got:\n%s", out)
	}
}

// shortLinkLike implements PrettyShort (a compact self-link) AND Pretty (a
// fuller block). The `short` tag must select PrettyShort.
type shortLinkLike struct {
	Kind string `json:"kind"`
	GUID string `json:"guid"`
	Name string `json:"name"`
}

func (l shortLinkLike) PrettyShort() api.Textable {
	return api.Text{}.Add(api.Link{Href: "/entity/" + l.Kind + "/" + l.GUID, Content: api.Text{Content: l.Name}})
}
func (l shortLinkLike) Pretty() api.Text {
	return api.Text{Content: "FULL-DETAIL-BLOCK-" + l.Name}
}

// TestClickyJSONShortTagRendersPrettyShort pins the `short` pretty-tag: a field
// tagged `pretty:",short"` renders via PrettyShort() (a link node), while the
// same value without the tag renders via Pretty() (the full block).
func TestClickyJSONShortTagRendersPrettyShort(t *testing.T) {
	type shortDetail struct {
		Plan shortLinkLike `json:"plan" pretty:",short"`
	}
	out, err := (&ClickyJSONFormatter{}).Format(
		shortDetail{Plan: shortLinkLike{Kind: "plan", GUID: "p-1", Name: "Life Plan"}},
		FormatOptions{})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.Contains(out, `"link"`) || !strings.Contains(out, "/entity/plan/p-1") {
		t.Errorf("short tag: expected a link node to the plan, got:\n%s", out)
	}
	if strings.Contains(out, "FULL-DETAIL-BLOCK") {
		t.Errorf("short tag: expected PrettyShort, not Pretty's full block, got:\n%s", out)
	}

	type plainDetail struct {
		Plan shortLinkLike `json:"plan"`
	}
	plain, err := (&ClickyJSONFormatter{}).Format(
		plainDetail{Plan: shortLinkLike{Kind: "plan", GUID: "p-1", Name: "Life Plan"}},
		FormatOptions{})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.Contains(plain, "FULL-DETAIL-BLOCK-Life Plan") {
		t.Errorf("without short tag: expected Pretty's full block, got:\n%s", plain)
	}
}
