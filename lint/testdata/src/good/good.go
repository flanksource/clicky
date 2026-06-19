package good

import (
	"fmt"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
)

// Rule 1 exception: empty literal for chaining
var chained = api.Text{}.Append("hello", "font-bold")

// Rule 1 exception: empty literal as zero value
var empty = api.Text{}

// Rule 2 exception: Pretty method
type MyType struct{}

func (m MyType) Pretty() api.Text {
	return api.Text{}.Append("hello")
}

// Rule 2 exception: PrettyFull method
func (m MyType) PrettyFull() api.Textable {
	return api.Text{}.Append("hello")
}

// Rule 2 exception: PrettyRow method
func (m MyType) PrettyRow(opts interface{}) map[string]api.Text {
	return nil
}

// Rule 2 exception: PrettyShort method
func (m MyType) PrettyShort() api.Textable {
	return api.Text{}.Append("hello")
}

// Using builder pattern - fine
var built = api.NewText("hello").Bold().Build()

// Using helper functions - fine
var success = api.SuccessText("done")

// Fluent chaining - fine
var fluent = api.Text{}.Append("label", "font-bold").Space().Append("value")

var helperText = clicky.Text("hello", "font-bold")
var helperList = clicky.List(clicky.Text("one"))
var helperTextList = clicky.TextList(clicky.Text("one"), clicky.Text("two"))
var helperTable = clicky.Table("Name", "Status")
var helperTree = clicky.Tree(clicky.Text("root"))
var helperCode = clicky.CodeBlock("go", "package main")
var helperButton = clicky.Button("Open", "/orders")
var helperButtonGroup = clicky.ButtonGroup(helperButton)
var helperKeyValue = clicky.KeyValue("status", "active")
var helperMap = clicky.Map(map[string]string{"env": "prod"})
var helperLabelBadge = clicky.LabelBadge("Status", "active")
var helperHeading = clicky.Heading(2, clicky.Text("Section"))
var helperHeader = clicky.Header(3, clicky.Text("Alias"))
var helperBlockquote = clicky.Blockquote(clicky.Text("quoted"))
var helperCollapsed = clicky.Collapsed("Details", clicky.Text("body"))
var helperAdmonition = clicky.Admonition(api.SeverityWarning, nil, clicky.Text("body"))
var helperFootnoteRef = clicky.FootnoteRef("cash")
var helperFootnote = clicky.Footnote("cash", clicky.Text("Cash equivalents"))
var helperFootnotes = clicky.Footnotes(helperFootnote)
var helperDiff = clicky.Diff("old", "new", "old", "new")
var helperStackTrace = clicky.StackTrace("panic")
var helperHTMLElement = clicky.HTMLElement("span", "raw")

type domainID string

func (d domainID) String() string {
	return string(d)
}

func BuildDetail(id domainID) api.Textable {
	return api.Text{}.Append(id.String()).Space().Append("ready", "text-green-600")
}

func BuildRows(name string) map[string]api.Text {
	return map[string]api.Text{
		"name": api.Text{}.Append(fmt.Sprintf("%s", name)),
	}
}
