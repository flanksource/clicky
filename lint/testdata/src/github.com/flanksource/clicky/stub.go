package clicky

import (
	"github.com/flanksource/clicky/api"
	"github.com/spf13/cobra"
)

// --- entity registration surface (subset used by the entityreg lint tests) ---

type EntityItem interface {
	GetID() string
	GetName() string
}

type EntityBuilder[T EntityItem, ListOpts any, R any] struct{}

func NewEntity[T EntityItem, ListOpts any, R any](name string) *EntityBuilder[T, ListOpts, R] {
	return &EntityBuilder[T, ListOpts, R]{}
}

func (b *EntityBuilder[T, ListOpts, R]) List(fn func(ListOpts) ([]T, error)) *EntityBuilder[T, ListOpts, R] {
	return b
}

func (b *EntityBuilder[T, ListOpts, R]) Register() {}

func GenerateCLI(parent *cobra.Command) {}

func Text(content string, styles ...string) api.Text { return api.Text{} }

func Heading(level int, content api.Textable) api.Heading { return api.Heading{} }

func Header(level int, content api.Textable) api.Heading { return api.Heading{} }

func Blockquote(content api.Textable) api.Blockquote { return api.Blockquote{} }

func FootnoteRef(id string) api.FootnoteRef { return api.FootnoteRef{} }

func Footnote(id string, content api.Textable) api.Footnote { return api.Footnote{} }

func Footnotes(notes ...api.Footnote) api.Footnotes { return api.Footnotes{} }

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
