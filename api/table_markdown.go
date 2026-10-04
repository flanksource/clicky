//go:build !wasm

package api

func (t TextTable) MarkdownWithOptions(options MarkdownOptions) string {
	return t.plainMarkdown(options)
}
