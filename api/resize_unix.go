//go:build !windows

package api

import (
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

// watchTerminalResize drops the cached terminal size whenever the window
// changes, so the next render re-measures instead of laying out for the size
// the process started with.
func watchTerminalResize() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, unix.SIGWINCH)
	go func() {
		for range ch {
			InvalidateTerminalSize()
		}
	}()
}
