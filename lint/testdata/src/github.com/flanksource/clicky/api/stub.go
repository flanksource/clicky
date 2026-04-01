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
