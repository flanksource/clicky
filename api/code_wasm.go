//go:build wasm

package api

// The wasm build does not link chroma or xmlfmt: code renders unhighlighted
// (plainCodeANSI / plainCodeHTML) and XML is not re-indented, so Code.HTML
// shows XML content exactly as given.

// ANSI returns the source code without syntax highlighting.
func (c Code) ANSI() string { return plainCodeANSI(c) }

// HTML returns the source code as an escaped <pre><code class="language-x">
// block for a client-side highlighter.
func (c Code) HTML() string { return plainCodeHTML(c) }

// highlightCodeLineHTML returns one escaped stack-trace source line; there is
// no highlighter on wasm, so no wrapper needs stripping.
func highlightCodeLineHTML(line, _ string) string { return htmlEscapeString(line) }

// GetChromaCSS returns "" on wasm: Code.HTML emits no chroma classes.
func GetChromaCSS() string { return "" }
