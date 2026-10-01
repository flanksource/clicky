//go:build js || wasip1

package shutdown

import (
	"os"
	"syscall"
)

// WebAssembly has no controlling terminal to hang up, so there is no SIGHUP.
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
