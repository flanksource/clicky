//go:build wasm

package api

// String renders the tree without lipgloss for the wasm build; the output
// matches the native renderer's connectors and label normalization.
func (tt TextTree) String() string {
	return tt.plainTree(false)
}

func (tt TextTree) ANSI() string {
	return tt.plainTree(true)
}
