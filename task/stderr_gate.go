package task

import (
	"io"
	"os"
)

// stderrGate forwards writes to os.Stderr unless the interactive task
// renderer currently owns the TTY, in which case the write is silently
// dropped. Used by callers that emit log-style content to stderr (e.g.
// clicky.Infof's logWriter, ai/cache debug prints) so their output
// cannot corrupt the renderer's in-place frame.
//
// Drops are silent by design: the user opted in by starting the
// interactive renderer. If you need writes preserved across the render
// window, route through commons/logger (already serialized via
// logSerializingWriter) or use StartCapturingOutput.
type stderrGate struct{}

func (stderrGate) Write(p []byte) (int, error) {
	if IsInteractiveRenderActive() {
		return len(p), nil
	}
	return os.Stderr.Write(p)
}

// GatedStderr returns a writer that wraps os.Stderr but drops writes
// while the interactive task renderer owns the TTY. The writer is
// stateless; each Write rechecks ownership, so a writer captured before
// the renderer started still gates correctly.
func GatedStderr() io.Writer {
	return stderrGate{}
}
