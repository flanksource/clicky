package markdown_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	clicky "github.com/flanksource/clicky"
	"github.com/flanksource/clicky/markdown"
)

func TestParseCanonicalFrontmatterAndSourceLines(t *testing.T) {
	source := "---\ntitle: Annual Report\n---\n# Report\n\nIntro **bold** and [site](https://example.com).\n\n## Notes\n"

	doc, err := markdown.ParseString(source)
	if err != nil {
		t.Fatalf("ParseString returned error: %v", err)
	}
	canonical := assertCanonicalMarkdownStable(t, source)
	if !strings.HasPrefix(canonical, "---\ntitle: Annual Report\n---\n\n# Report\n") {
		t.Fatalf("canonical markdown did not preserve frontmatter and heading:\n%s", canonical)
	}
	if got := doc.Metadata["title"]; got != "Annual Report" {
		t.Fatalf("frontmatter title = %v, want Annual Report", got)
	}
	if len(doc.Root.Children) < 3 {
		t.Fatalf("expected parsed children, got %#v", doc.Root.Children)
	}

	heading := doc.Root.Children[0]
	if heading.Kind != "heading" || heading.Level != 1 || heading.String() != "Report" {
		t.Fatalf("unexpected first heading: %#v", heading)
	}
	if heading.LineStart != 4 || heading.LineEnd != 4 {
		t.Fatalf("heading lines = %d-%d, want 4-4", heading.LineStart, heading.LineEnd)
	}

	paragraph := doc.Root.Children[1]
	if !containsKind(paragraph.Children, "strong") || !containsKind(paragraph.Children, "link") {
		t.Fatalf("paragraph children missing rich inline nodes: %#v", paragraph.Children)
	}
}

func TestParseGFMTaskListAndTable(t *testing.T) {
	source := "- [x] Done\n- [ ] Todo\n\n| Name | Qty |\n| :--- | ---: |\n| Cash | 10 |\n"

	doc, err := markdown.ParseString(source)
	if err != nil {
		t.Fatalf("ParseString returned error: %v", err)
	}
	if len(doc.Root.Children) != 2 {
		t.Fatalf("children = %d, want 2: %#v", len(doc.Root.Children), doc.Root.Children)
	}

	list := doc.Root.Children[0]
	if list.Kind != "list" || len(list.Items) != 2 {
		t.Fatalf("unexpected list: %#v", list)
	}
	if list.Items[0].Checked == nil || !*list.Items[0].Checked {
		t.Fatalf("first task checked = %#v, want true", list.Items[0].Checked)
	}
	if list.Items[1].Checked == nil || *list.Items[1].Checked {
		t.Fatalf("second task checked = %#v, want false", list.Items[1].Checked)
	}

	table := doc.Root.Children[1]
	clickyNode := table.ClickyNode()
	if table.Kind != "table" || len(clickyNode.Columns) != 2 || len(clickyNode.Rows) != 1 {
		t.Fatalf("unexpected table clicky node: %#v", clickyNode)
	}
	if clickyNode.Columns[0].Name != "name" || clickyNode.Columns[1].Align != "right" {
		t.Fatalf("unexpected columns: %#v", clickyNode.Columns)
	}
	if got := clickyNode.Rows[0].Cells["qty"].Plain; got != "10" {
		t.Fatalf("qty cell = %q, want 10", got)
	}
}

func TestParseAdmonitionAndDetailsHTML(t *testing.T) {
	source := "!!! warning Review\n    Check this before export.\n\n<details>\n<summary>More</summary>\n<p>Hidden</p>\n</details>\n"

	doc, err := markdown.ParseString(source)
	if err != nil {
		t.Fatalf("ParseString returned error: %v", err)
	}
	if len(doc.Root.Children) != 2 {
		t.Fatalf("children = %d, want 2: %#v", len(doc.Root.Children), doc.Root.Children)
	}

	admonition := doc.Root.Children[0]
	if admonition.Kind != "admonition" || admonition.Severity != "warning" || admonition.Title != "Review" {
		t.Fatalf("unexpected admonition: %#v", admonition)
	}
	if !strings.Contains(admonition.String(), "Check this before export.") {
		t.Fatalf("admonition body missing: %q", admonition.String())
	}

	collapsed := doc.Root.Children[1]
	if collapsed.Kind != "collapsed" || collapsed.Title != "More" {
		t.Fatalf("unexpected collapsed details: %#v", collapsed)
	}
}

func TestParseFootnoteLabels(t *testing.T) {
	doc, err := markdown.ParseString("Cash[^cash].\n\n[^cash]: Bank deposits.\n")
	if err != nil {
		t.Fatalf("ParseString returned error: %v", err)
	}
	if len(doc.Root.Children) != 2 {
		t.Fatalf("children = %d, want 2: %#v", len(doc.Root.Children), doc.Root.Children)
	}
	paragraph := doc.Root.Children[0]
	if len(paragraph.Children) < 2 || paragraph.Children[1].Kind != "footnote_ref" || paragraph.Children[1].ID != "cash" {
		t.Fatalf("paragraph footnote ref not preserved: %#v", paragraph.Children)
	}
	footnotes := doc.Root.Children[1]
	if footnotes.Kind != "footnotes" || len(footnotes.Items) != 1 || footnotes.Items[0].ID != "cash" {
		t.Fatalf("footnote definition not preserved: %#v", footnotes)
	}
}

func TestParseKitchenSinkRoundTripAndClickyJSON(t *testing.T) {
	source := readKitchenSink(t)

	doc, err := markdown.ParseString(source)
	if err != nil {
		t.Fatalf("ParseString returned error: %v", err)
	}
	canonical := assertCanonicalMarkdownStable(t, source)
	if !strings.HasPrefix(canonical, "---\ntitle: Kitchen Sink\nversion: 1\n---\n\n# Kitchen Sink\n") {
		t.Fatalf("canonical markdown did not preserve frontmatter and title:\n%s", canonical)
	}
	if got := doc.Metadata["title"]; got != "Kitchen Sink" {
		t.Fatalf("frontmatter title = %v, want Kitchen Sink", got)
	}

	for _, kind := range []string{
		"heading",
		"paragraph",
		"blockquote",
		"list",
		"table",
		"code_block",
		"admonition",
		"collapsed",
		"raw-html",
		"thematic_break",
		"footnote_ref",
		"footnotes",
	} {
		if countKind(doc.Root, kind) == 0 {
			t.Fatalf("kitchen sink did not produce %q node: %#v", kind, doc.Root)
		}
	}

	taskList := findNode(doc.Root, func(n markdown.Node) bool {
		if n.Kind != "list" {
			return false
		}
		for _, item := range n.Items {
			if item.Checked != nil {
				return true
			}
		}
		return false
	})
	if taskList.Kind == "" || len(taskList.Items) < 2 || taskList.Items[0].Checked == nil || !*taskList.Items[0].Checked || taskList.Items[1].Checked == nil || *taskList.Items[1].Checked {
		t.Fatalf("task list state not preserved: %#v", taskList)
	}

	table := findNode(doc.Root, func(n markdown.Node) bool { return n.Kind == "table" })
	clickyTable := table.ClickyNode()
	if len(clickyTable.Columns) != 3 || len(clickyTable.Rows) != 2 {
		t.Fatalf("table did not convert to clicky columns/rows: %#v", clickyTable)
	}
	if clickyTable.Columns[1].Align != "right" || clickyTable.Columns[2].Align != "center" {
		t.Fatalf("table alignment not preserved: %#v", clickyTable.Columns)
	}

	out, err := clicky.Format(doc, clicky.FormatOptions{Format: "clicky-json"})
	if err != nil {
		t.Fatalf("Format clicky-json returned error: %v", err)
	}
	var payload struct {
		Version int `json:"version"`
		Node    struct {
			Kind string `json:"kind"`
		} `json:"node"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal kitchen sink clicky-json: %v\n%s", err, out)
	}
	if payload.Version != 1 || payload.Node.Kind != "document" {
		t.Fatalf("unexpected kitchen sink payload: %#v", payload)
	}
	assertNoParserProvenance(t, out)
}

func readKitchenSink(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate markdown test file")
	}
	sourcePath := filepath.Join(filepath.Dir(currentFile), "..", "examples", "kitchen-sink.md")
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read kitchen sink example: %v", err)
	}
	return string(data)
}

func TestClickyJSONUsesParsedMarkdownProvider(t *testing.T) {
	doc := clicky.MustParseMarkdown("# Report\n\n- [x] Done\n")

	out, err := clicky.Format(doc, clicky.FormatOptions{Format: "clicky-json"})
	if err != nil {
		t.Fatalf("Format clicky-json returned error: %v", err)
	}

	var payload struct {
		Version int `json:"version"`
		Node    struct {
			Kind     string `json:"kind"`
			Children []struct {
				Kind  string `json:"kind"`
				Level int    `json:"level,omitempty"`
			} `json:"children"`
		} `json:"node"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal clicky-json: %v\n%s", err, out)
	}
	if payload.Version != 1 || payload.Node.Kind != "document" {
		t.Fatalf("unexpected payload envelope: %#v", payload)
	}
	if len(payload.Node.Children) != 2 || payload.Node.Children[0].Kind != "heading" || payload.Node.Children[0].Level != 1 {
		t.Fatalf("unexpected parsed payload children: %#v", payload.Node.Children)
	}
	assertNoParserProvenance(t, out)

	plain, err := clicky.Format("**not parsed**", clicky.FormatOptions{Format: "markdown"})
	if err != nil {
		t.Fatalf("Format markdown string returned error: %v", err)
	}
	if strings.TrimSpace(plain) != "**not parsed**" {
		t.Fatalf("plain string markdown changed: %q", plain)
	}
}

func TestParsedMarkdownHeadingMatchesBuilderClickyNode(t *testing.T) {
	doc := clicky.MustParseMarkdown("# Report\n")
	parsedOut, err := clicky.Format(doc, clicky.FormatOptions{Format: "clicky-json"})
	if err != nil {
		t.Fatalf("Format parsed markdown: %v", err)
	}
	builderOut, err := clicky.Format(clicky.Heading(1, clicky.Text("Report")), clicky.FormatOptions{Format: "clicky-json"})
	if err != nil {
		t.Fatalf("Format builder heading: %v", err)
	}

	var parsed struct {
		Node struct {
			Children []map[string]any `json:"children"`
		} `json:"node"`
	}
	if err := json.Unmarshal([]byte(parsedOut), &parsed); err != nil {
		t.Fatalf("unmarshal parsed clicky-json: %v\n%s", err, parsedOut)
	}
	var builder struct {
		Node map[string]any `json:"node"`
	}
	if err := json.Unmarshal([]byte(builderOut), &builder); err != nil {
		t.Fatalf("unmarshal builder clicky-json: %v\n%s", err, builderOut)
	}
	if len(parsed.Node.Children) != 1 {
		t.Fatalf("parsed markdown children = %d, want 1: %s", len(parsed.Node.Children), parsedOut)
	}
	if !reflect.DeepEqual(parsed.Node.Children[0], builder.Node) {
		t.Fatalf("parsed markdown heading differs from builder heading\nparsed: %#v\nbuilder: %#v", parsed.Node.Children[0], builder.Node)
	}
}

func assertCanonicalMarkdownStable(t *testing.T, source string) string {
	t.Helper()
	doc, err := markdown.ParseString(source)
	if err != nil {
		t.Fatalf("parse original markdown: %v", err)
	}
	canonical := doc.Markdown()
	roundTripped, err := markdown.ParseString(canonical)
	if err != nil {
		t.Fatalf("parse canonical markdown: %v\n%s", err, canonical)
	}
	if got := roundTripped.Markdown(); got != canonical {
		t.Fatalf("canonical markdown is not stable\nfirst:\n%s\nsecond:\n%s", canonical, got)
	}
	return canonical
}

func assertNoParserProvenance(t *testing.T, out string) {
	t.Helper()
	for _, field := range []string{"sourceMarkdown", "lineStart", "lineEnd"} {
		if strings.Contains(out, field) {
			t.Fatalf("clicky-json leaked parser provenance field %q:\n%s", field, out)
		}
	}
}

func containsKind(nodes []markdown.Node, kind string) bool {
	for _, node := range nodes {
		if node.Kind == kind {
			return true
		}
	}
	return false
}

func countKind(node markdown.Node, kind string) int {
	count := 0
	if node.Kind == kind {
		count++
	}
	for _, child := range node.Children {
		count += countKind(child, kind)
	}
	for _, item := range node.Items {
		count += countKind(item, kind)
	}
	return count
}

func findNode(node markdown.Node, match func(markdown.Node) bool) markdown.Node {
	if match(node) {
		return node
	}
	for _, child := range node.Children {
		if found := findNode(child, match); found.Kind != "" {
			return found
		}
	}
	for _, item := range node.Items {
		if found := findNode(item, match); found.Kind != "" {
			return found
		}
	}
	return markdown.Node{}
}
