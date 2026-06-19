package api

type Textable interface {
	String() string
	ANSI() string
	HTML() string
	Markdown() string
}

type Text struct {
	Content  string
	Style    string
	Children []Textable
	Tooltip  Textable
}

func (t Text) String() string   { return t.Content }
func (t Text) ANSI() string     { return t.Content }
func (t Text) HTML() string     { return t.Content }
func (t Text) Markdown() string { return t.Content }
func (t Text) MarkdownSlack() string {
	return t.Content
}

func (t Text) Append(text any, styles ...string) Text  { return t }
func (t Text) Appendf(format string, args ...any) Text { return t }
func (t Text) Add(child Textable) Text                 { return t }
func (t Text) Space() Text                             { return t }
func (t Text) Styles(classes ...string) Text           { return t }

type TextBuilder struct{ text Text }

func NewText(content string) *TextBuilder           { return &TextBuilder{text: Text{Content: content}} }
func (tb *TextBuilder) Bold() *TextBuilder          { return tb }
func (tb *TextBuilder) Color(c string) *TextBuilder { return tb }
func (tb *TextBuilder) Build() Text                 { return tb.text }

func SuccessText(content string) Text { return NewText(content).Build() }
func ErrorText(content string) Text   { return NewText(content).Build() }

type TextList []Textable

func (tl TextList) String() string       { return "" }
func (tl TextList) ANSI() string         { return "" }
func (tl TextList) HTML() string         { return "" }
func (tl TextList) Markdown() string     { return "" }
func (tl TextList) AsANSI() []string     { return nil }
func (tl TextList) AsHTML() []string     { return nil }
func (tl TextList) AsMarkdown() []string { return nil }

type List struct {
	Items []Textable
}

func (l List) String() string   { return "" }
func (l List) ANSI() string     { return "" }
func (l List) HTML() string     { return "" }
func (l List) Markdown() string { return "" }

type TextTable struct {
	Headers TextList
}

func (tt TextTable) String() string      { return "" }
func (tt TextTable) ANSI() string        { return "" }
func (tt TextTable) HTML() string        { return "" }
func (tt TextTable) Markdown() string    { return "" }
func (tt TextTable) CompactHTML() string { return "" }
func (tt TextTable) StaticHTML() string  { return "" }

type TextTree struct {
	Node Textable
}

func (tt TextTree) String() string   { return "" }
func (tt TextTree) ANSI() string     { return "" }
func (tt TextTree) HTML() string     { return "" }
func (tt TextTree) Markdown() string { return "" }

type Code struct {
	Content  string
	Language string
}

func (c Code) String() string   { return "" }
func (c Code) ANSI() string     { return "" }
func (c Code) HTML() string     { return "" }
func (c Code) Markdown() string { return "" }

type Button struct {
	Label string
	Href  string
}

func (b Button) String() string   { return "" }
func (b Button) ANSI() string     { return "" }
func (b Button) HTML() string     { return "" }
func (b Button) Markdown() string { return "" }

type ButtonGroup struct {
	Buttons []Button
}

func (b ButtonGroup) String() string   { return "" }
func (b ButtonGroup) ANSI() string     { return "" }
func (b ButtonGroup) HTML() string     { return "" }
func (b ButtonGroup) Markdown() string { return "" }

type KeyValuePair struct {
	Key   string
	Value any
}

func (kv KeyValuePair) String() string   { return "" }
func (kv KeyValuePair) ANSI() string     { return "" }
func (kv KeyValuePair) HTML() string     { return "" }
func (kv KeyValuePair) Markdown() string { return "" }

type DescriptionList struct {
	Items []KeyValuePair
}

func (dl DescriptionList) String() string   { return "" }
func (dl DescriptionList) ANSI() string     { return "" }
func (dl DescriptionList) HTML() string     { return "" }
func (dl DescriptionList) Markdown() string { return "" }

type LabelBadge struct {
	Label string
	Value string
}

func (b LabelBadge) String() string   { return "" }
func (b LabelBadge) ANSI() string     { return "" }
func (b LabelBadge) HTML() string     { return "" }
func (b LabelBadge) Markdown() string { return "" }

type Heading struct {
	Level   int
	Content Textable
}

func (h Heading) String() string   { return "" }
func (h Heading) ANSI() string     { return "" }
func (h Heading) HTML() string     { return "" }
func (h Heading) Markdown() string { return "" }

type Blockquote struct {
	Content Textable
}

func (b Blockquote) String() string   { return "" }
func (b Blockquote) ANSI() string     { return "" }
func (b Blockquote) HTML() string     { return "" }
func (b Blockquote) Markdown() string { return "" }

type Severity int

const SeverityWarning Severity = 3

type Admonition struct {
	Severity Severity
	Body     Textable
}

func (a Admonition) String() string   { return "" }
func (a Admonition) ANSI() string     { return "" }
func (a Admonition) HTML() string     { return "" }
func (a Admonition) Markdown() string { return "" }

type FootnoteRef struct {
	ID string
}

func (r FootnoteRef) String() string   { return "" }
func (r FootnoteRef) ANSI() string     { return "" }
func (r FootnoteRef) HTML() string     { return "" }
func (r FootnoteRef) Markdown() string { return "" }

type Footnote struct {
	ID      string
	Content Textable
}

func (f Footnote) String() string   { return "" }
func (f Footnote) ANSI() string     { return "" }
func (f Footnote) HTML() string     { return "" }
func (f Footnote) Markdown() string { return "" }

type Footnotes struct {
	Items []Footnote
}

func (f Footnotes) String() string   { return "" }
func (f Footnotes) ANSI() string     { return "" }
func (f Footnotes) HTML() string     { return "" }
func (f Footnotes) Markdown() string { return "" }

type Collapsed struct {
	Label   string
	Content Textable
}

func (c Collapsed) String() string   { return "" }
func (c Collapsed) ANSI() string     { return "" }
func (c Collapsed) HTML() string     { return "" }
func (c Collapsed) Markdown() string { return "" }

type Diff struct {
	Before string
	After  string
}

func (d Diff) String() string   { return "" }
func (d Diff) ANSI() string     { return "" }
func (d Diff) HTML() string     { return "" }
func (d Diff) Markdown() string { return "" }

type StackTrace struct {
	Raw string
}

func (s StackTrace) String() string   { return "" }
func (s StackTrace) ANSI() string     { return "" }
func (s StackTrace) HTML() string     { return "" }
func (s StackTrace) Markdown() string { return "" }

type HtmlElement struct {
	Tag     string
	Content string
}

func (e HtmlElement) String() string   { return "" }
func (e HtmlElement) ANSI() string     { return "" }
func (e HtmlElement) HTML() string     { return "" }
func (e HtmlElement) Markdown() string { return "" }

type Comment string

func (c Comment) String() string   { return "" }
func (c Comment) ANSI() string     { return "" }
func (c Comment) HTML() string     { return "" }
func (c Comment) Markdown() string { return "" }
