package bad

import (
	"fmt"

	"github.com/flanksource/clicky/api"
)

// Rule 1: direct struct literal with fields
var directLiteral = api.Text{Content: "hello", Style: "font-bold"} // want `avoid direct api\.Text struct literal`

// Rule 1: single field still counts
var singleField = api.Text{Content: "hello"} // want `avoid direct api\.Text struct literal`

// Rule 2: non-Pretty function returning api.Text
func MakeLabel() api.Text { // want `MakeLabel returns api\.Text`
	return api.Text{}
}

// Rule 2: method not named Pretty
type Foo struct{}

func (f Foo) Render() api.Text { // want `Render returns api\.Text`
	return api.Text{}
}

// Rule 3: string concatenation in Content (also triggers Rule 1)
var concatContent = api.Text{Content: "hello" + " world"} // want `avoid direct api\.Text struct literal` `avoid string concatenation in Content field`

// Rule 3: fmt.Sprintf in Content (also triggers Rule 1)
var sprintfContent = api.Text{Content: fmt.Sprintf("hello %s", "world")} // want `avoid direct api\.Text struct literal` `avoid fmt\.Sprintf in Content field`

// Rule 4: Children slice literal with api.Text elements
var childrenLiteral = api.Text{ // want `avoid direct api\.Text struct literal`
	Children: []api.Textable{api.Text{Content: "a"}, api.Text{Content: "b"}}, // want `avoid Children slice literal` `avoid direct api\.Text struct literal` `avoid direct api\.Text struct literal`
}

var directList = api.List{Items: []api.Textable{api.Text{}.Append("one")}} // want `avoid direct api\.List struct literal`
var directTextList = api.TextList{api.Text{}.Append("one")}                // want `avoid direct api\.TextList struct literal`
var directTable = api.TextTable{Headers: api.TextList{}}                   // want `avoid direct api\.TextTable struct literal` `avoid direct api\.TextList struct literal`
var directTree = api.TextTree{Node: api.Text{}.Append("root")}             // want `avoid direct api\.TextTree struct literal`
var directCode = api.Code{Content: "package main"}                         // want `avoid direct api\.Code struct literal`
var directButton = api.Button{Label: "Open", Href: "/orders"}              // want `avoid direct api\.Button struct literal`
var directButtonGroup = api.ButtonGroup{Buttons: nil}                      // want `avoid direct api\.ButtonGroup struct literal`
var directKeyValue = api.KeyValuePair{Key: "status", Value: "active"}      // want `avoid direct api\.KeyValuePair struct literal`
var directDescriptionList = api.DescriptionList{Items: nil}                // want `avoid direct api\.DescriptionList struct literal`
var directLabelBadge = api.LabelBadge{Label: "Status", Value: "active"}    // want `avoid direct api\.LabelBadge struct literal`
var directAdmonition = api.Admonition{Severity: api.SeverityWarning}       // want `avoid direct api\.Admonition struct literal`
var directCollapsed = api.Collapsed{Label: "Details"}                      // want `avoid direct api\.Collapsed struct literal`
var directDiff = api.Diff{Before: "old", After: "new"}                     // want `avoid direct api\.Diff struct literal`
var directStackTrace = api.StackTrace{Raw: "panic"}                        // want `avoid direct api\.StackTrace struct literal`
var directHTMLElement = api.HtmlElement{Tag: "span", Content: "raw"}       // want `avoid direct api\.HtmlElement struct literal`

type RenderBuilder struct{}

func (r RenderBuilder) Pretty() api.Text {
	text := api.Text{}.Append("hello")
	return api.Text{}.Append(text.ANSI()) // want `avoid \.ANSI\(\) inside clicky render builders`
}

func (r RenderBuilder) PrettyFull() api.Textable {
	text := api.Text{}.Append("hello")
	return api.Text{}.Append(text.HTML()) // want `avoid \.HTML\(\) inside clicky render builders`
}

func (r RenderBuilder) PrettyShort() api.Textable {
	text := api.Text{}.Append("hello")
	return api.Text{}.Append(text.String()) // want `avoid \.String\(\) inside clicky render builders`
}

func (r RenderBuilder) PrettyRow(_ interface{}) map[string]api.Text {
	text := api.Text{}.Append("hello")
	return map[string]api.Text{
		"name": api.Text{}.Append(text.Markdown()), // want `avoid \.Markdown\(\) inside clicky render builders`
	}
}

func BuildTextable() api.Textable {
	text := api.Text{}.Append("hello")
	return api.Text{}.Append(text.MarkdownSlack()) // want `avoid \.MarkdownSlack\(\) inside clicky render builders`
}

func BuildTable() api.TextTable {
	table := api.TextTable{} // want `avoid direct api\.TextTable struct literal`
	_ = table.CompactHTML()  // want `avoid \.CompactHTML\(\) inside clicky render builders`
	return table
}

func BuildList() api.TextList {
	list := api.TextList{api.Text{}.Append("hello")} // want `avoid direct api\.TextList struct literal`
	_ = list.AsANSI()                                // want `avoid \.AsANSI\(\) inside clicky render builders`
	return list
}
