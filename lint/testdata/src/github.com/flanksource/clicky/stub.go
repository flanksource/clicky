package clicky

import "github.com/flanksource/clicky/api"

func Text(content string, styles ...string) api.Text { return api.Text{} }

func List(items ...api.Textable) api.List { return api.List{} }

func TextList(items ...api.Textable) api.TextList { return api.TextList(items) }

func Table(headers ...string) api.TextTable { return api.TextTable{} }

func Tree(node api.Textable, children ...api.TextTree) api.TextTree { return api.TextTree{} }

func KeyValue(key string, value any, styles ...string) api.KeyValuePair {
	return api.KeyValuePair{}
}

func Map[T any](m map[string]T, styles ...string) api.DescriptionList {
	return api.DescriptionList{}
}

func CodeBlock(language, content string, styles ...string) api.Code { return api.Code{} }

func Button(label, href string, options ...func(*api.Button)) api.Button { return api.Button{} }

func ButtonGroup(buttons ...api.Button) api.ButtonGroup { return api.ButtonGroup{} }

func LabelBadge(label, value string, options ...func(*api.LabelBadge)) api.LabelBadge {
	return api.LabelBadge{}
}

func Collapsed(label string, content api.Textable, styles ...string) api.Collapsed {
	return api.Collapsed{}
}

func Admonition(severity api.Severity, title, body api.Textable) api.Admonition {
	return api.Admonition{}
}

func Diff(before, after, fromLabel, toLabel string) api.Diff { return api.Diff{} }

func StackTrace(input string) api.StackTrace { return api.StackTrace{} }

func Comment(text string) api.Comment { return api.Comment(text) }

func HTMLElement(tag, content string, attrs ...map[string]string) api.HtmlElement {
	return api.HtmlElement{}
}
