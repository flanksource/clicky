package clicky

import md "github.com/flanksource/clicky/markdown"

// ParseMarkdown parses markdown source into a structured Clicky document.
func ParseMarkdown(source string, opts ...md.Option) (*md.Document, error) {
	return md.ParseString(source, opts...)
}

// MustParseMarkdown parses markdown source and panics on failure.
func MustParseMarkdown(source string, opts ...md.Option) *md.Document {
	doc, err := ParseMarkdown(source, opts...)
	if err != nil {
		panic(err)
	}
	return doc
}
