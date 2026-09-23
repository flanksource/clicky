//go:build !unix

package api

// watchTerminalResize is a no-op where there is no SIGWINCH — Windows, and the
// js/wasm and wasip1 targets a browser-side build of a clicky consumer uses. The
// size is measured once and callers can re-measure via InvalidateTerminalSize.
func watchTerminalResize() {}
