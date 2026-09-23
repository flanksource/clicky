//go:build wasm

package api

// MarkdownWithOptions renders the table as an unpadded GFM pipe table so the
// wasm build does not link tablewriter. Cell and header text match the native
// renderer; only column padding and alignment markers differ.
func (t TextTable) MarkdownWithOptions(options MarkdownOptions) string {
	return t.plainMarkdown(options)
}
