// Package good exercises the allowed cases of the direct-stdout rule.
package good

import (
	"bytes"
	"fmt"
	"os"
)

// Non-stdout/stderr targets are fine — the rule only catches writes to
// os.Stdout / os.Stderr, not to caller-supplied buffers.
func FprintToBuffer() {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "to buf")
}

// os.Stdin is not flagged (prompts are legitimate).
func ReadFromStdin() {
	var s string
	_, _ = fmt.Fscanln(os.Stdin, &s)
}

// Non-writer methods on os.Stdout are fine — the rule targets writes, not
// inspection of the stream (Fd(), Name()).
func InspectStdout() uintptr {
	return os.Stdout.Fd()
}
