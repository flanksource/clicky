package formatters

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/flanksource/clicky/api"
)

// richDoc builds a Text carrying a labelled YAML code block wrapped in a
// collapsible — the shape the apply suite attaches as a test's source view.
func richDoc() api.Text {
	body := api.Text{Content: "source"}.
		NewLine().
		Add(api.CodeBlock("yaml", "kind: TestPlan\nproduct: Acme\n"))
	return api.Text{Content: "Document"}.
		Add(api.Collapsed{Label: "details", Content: body})
}

func TestNewClickyDocumentEncodesCodeAndCollapsed(t *testing.T) {
	doc := NewClickyDocument(richDoc())

	if doc.Version != 1 {
		t.Fatalf("version = %d, want 1", doc.Version)
	}
	if doc.Node.Kind != "text" {
		t.Fatalf("root kind = %q, want text", doc.Node.Kind)
	}

	collapsed := findNode(doc.Node, "collapsed")
	if collapsed == nil {
		t.Fatal("no collapsed node in document")
	}
	if collapsed.Content == nil {
		t.Fatal("collapsed node has no content")
	}

	code := findNode(doc.Node, "code")
	if code == nil {
		t.Fatal("no code node in document")
	}
	if code.Language != "yaml" {
		t.Fatalf("code language = %q, want yaml", code.Language)
	}
	if code.Source != "kind: TestPlan\nproduct: Acme\n" {
		t.Fatalf("code source = %q, want the yaml body", code.Source)
	}
}

func TestClickyDocumentRoundTrips(t *testing.T) {
	want := NewClickyDocument(richDoc())

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ClickyDocument
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round-trip mismatch:\n want %#v\n got  %#v", want, got)
	}
}

func TestClickyTextMarshalsAsDocument(t *testing.T) {
	type holder struct {
		Detail ClickyText `json:"detail"`
	}

	raw, err := json.Marshal(holder{Detail: ClickyText{Textable: richDoc()}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		Detail ClickyDocument `json:"detail"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Detail.Version != 1 {
		t.Fatalf("wrapped version = %d, want 1", decoded.Detail.Version)
	}
	if findNode(decoded.Detail.Node, "code") == nil {
		t.Fatal("wrapped document lost its code block")
	}
}

// findNode returns the first node of the given kind in a depth-first walk over
// the node tree (children, items, content, label).
func findNode(node ClickyNode, kind string) *ClickyNode {
	if node.Kind == kind {
		return &node
	}
	candidates := append([]ClickyNode{}, node.Children...)
	candidates = append(candidates, node.Items...)
	if node.Content != nil {
		candidates = append(candidates, *node.Content)
	}
	if node.Label != nil {
		candidates = append(candidates, *node.Label)
	}
	for _, c := range candidates {
		if found := findNode(c, kind); found != nil {
			return found
		}
	}
	return nil
}
