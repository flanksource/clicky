package api

import (
	"strings"
	"testing"
)

// A pty created without a window size makes term.GetSize return (0, 0, nil).
// Before the fix that zero was cached and returned, and every width-aware
// renderer collapsed to a single column.
func TestTerminalSizeNeverNonPositive(t *testing.T) {
	t.Cleanup(InvalidateTerminalSize)

	for _, pinned := range []int{0, -1, -80} {
		SetTerminalWidth(pinned)
		if got := GetTerminalWidth(); got <= 0 {
			t.Errorf("GetTerminalWidth() = %d after pinning %d, want a positive width", got, pinned)
		}
		SetTerminalLines(pinned)
		if got := GetTerminalLines(); got <= 0 {
			t.Errorf("GetTerminalLines() = %d after pinning %d, want a positive height", got, pinned)
		}
	}
}

func TestSetTerminalWidthIsUsedAndInvalidated(t *testing.T) {
	t.Cleanup(InvalidateTerminalSize)

	SetTerminalWidth(97)
	if got := GetTerminalWidth(); got != 97 {
		t.Errorf("GetTerminalWidth() = %d, want the pinned 97", got)
	}

	InvalidateTerminalSize()
	if got := GetTerminalWidth(); got == 97 {
		t.Error("GetTerminalWidth() still returned the pinned width after InvalidateTerminalSize")
	}
}

// A tree label must survive an unmeasurable terminal. With a zero width the
// label budget went negative, clamped to one column, and rendered as "…".
func TestTreeLabelSurvivesUnmeasurableWidth(t *testing.T) {
	t.Cleanup(InvalidateTerminalSize)
	SetTerminalWidth(0)

	const label = "View full results: https://github.com/acme/widgets/actions/runs/31387133937"
	tree := TextTree{
		Node:     Text{Content: "gavel"},
		Children: []TextTree{{Node: Text{Content: label}}},
	}

	rendered := tree.String()
	if !strings.Contains(rendered, label) {
		t.Errorf("tree label was truncated at width 0:\n%q", rendered)
	}

	// Negative control: the renderer really is width-sensitive, so the check
	// above passes because the width fell back, not because width is ignored.
	SetTerminalWidth(20)
	if narrow := tree.String(); strings.Contains(narrow, label) {
		t.Errorf("tree label was not truncated at width 20, so the width-0 case proves nothing:\n%q", narrow)
	}
}
