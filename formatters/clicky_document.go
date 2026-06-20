package formatters

import (
	"encoding/json"

	"github.com/flanksource/clicky/api"
)

// NewClickyDocument builds the {version:1, node:{…}} document directly from a
// Textable, reusing the same node converter the clicky-json / html-react
// formatters use. Unlike those formatters it does not route through PrettyData,
// so a hand-built rich Text (code blocks, collapsibles, tables, …) is encoded
// verbatim. The result is the shape consumed by @flanksource/clicky-ui's
// <Clicky data={…} /> component and round-trips through encoding/json.
func NewClickyDocument(t api.Textable) ClickyDocument {
	if t == nil {
		return ClickyDocument{Version: 1, Node: clickyTextNode("")}
	}
	if provider, ok := t.(ClickyDocumentProvider); ok {
		return provider.ClickyDocument()
	}
	return ClickyDocument{Version: 1, Node: convertTextable(t)}
}

// ClickyText wraps a Textable so json.Marshal emits the structured clicky
// document (NewClickyDocument) instead of the flattened plain string that
// api.Text.MarshalJSON produces. Drop it into any struct field whose JSON should
// be rendered by <Clicky/>:
//
//	type Result struct {
//	    Detail ClickyText `json:"detail"`
//	}
//
// The transported/persisted form is the concrete ClickyDocument, which is what
// unmarshals — ClickyText is producer-side only.
type ClickyText struct {
	api.Textable
}

// MarshalJSON renders the wrapped Textable as a ClickyDocument.
func (c ClickyText) MarshalJSON() ([]byte, error) {
	return json.Marshal(NewClickyDocument(c.Textable))
}
