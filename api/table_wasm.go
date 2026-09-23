//go:build wasm

package api

// String renders the table as borderless aligned columns so the wasm build
// does not link lipgloss. Cell text matches the native renderer; the native
// border box and terminal-width column fitting are not reproduced.
func (t TextTable) String() string {
	return t.plainTable(false)
}

func (t TextTable) ANSI() string {
	return "\n" + t.plainTable(true)
}
