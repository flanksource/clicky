package clicky

import (
	"errors"
	"fmt"
	"testing"

	"github.com/flanksource/clicky/api"
)

// prettyErr is a test error that carries a clicky rendering interface —
// the shape renderableError must detect so the command runner renders it
// through the format pipeline instead of collapsing it to Error().
type prettyErr struct{ msg string }

func (e *prettyErr) Error() string    { return e.msg }
func (e *prettyErr) Pretty() api.Text { return api.Text{Content: "rendered: " + e.msg} }

func TestRenderableError_DetectsErrorImplementingPretty(t *testing.T) {
	rich, ok := renderableError(&prettyErr{msg: "boom"})
	if !ok {
		t.Fatalf("renderableError must detect an error implementing Pretty()")
	}
	if api.TryTypedValue(rich) == nil {
		t.Fatalf("the returned value must be renderable by the format pipeline")
	}
}

func TestRenderableError_WalksWrappedChain(t *testing.T) {
	// A fmt-wrapped rich error must still be detected — clicky walks the
	// Unwrap chain so a renderable cause survives an outer fmt.Errorf.
	wrapped := fmt.Errorf("dispatching: %w", &prettyErr{msg: "boom"})
	rich, ok := renderableError(wrapped)
	if !ok {
		t.Fatalf("renderableError must walk the Unwrap chain to find a renderable cause")
	}
	if _, isPretty := rich.(*prettyErr); !isPretty {
		t.Fatalf("renderableError must return the matched *prettyErr, got %T", rich)
	}
}

func TestRenderableError_PlainErrorIsNotRenderable(t *testing.T) {
	// A plain error carries no rendering interface — the command runner
	// must fall back to logging Error(), not the render pipeline.
	if _, ok := renderableError(errors.New("plain failure")); ok {
		t.Fatalf("a plain error must not be reported as renderable")
	}
}
