//go:build windows

package api

// watchTerminalResize is a no-op on Windows, which has no SIGWINCH. The size is
// measured once and callers can re-measure via InvalidateTerminalSize.
func watchTerminalResize() {}
