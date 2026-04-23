package gumchoose

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSingleSelectCurrentRowDoesNotDuplicateCursorOrPrefix(t *testing.T) {
	model := newModel([]string{"Scale canary up", "Abort release"}, Options{
		Header: "Choose action",
		Height: 5,
		Limit:  1,
	})

	view := ansi.Strip(model.View())
	if strings.Contains(view, ">  > ") {
		t.Fatalf("expected no duplicated cursor, got %q", view)
	}
	if !strings.Contains(view, "> Scale canary up") {
		t.Fatalf("expected single-select row to render without extra prefix, got %q", view)
	}
}

func TestMultiSelectUsesCirclePrefixes(t *testing.T) {
	model := newModel([]string{"Check pods", "Inspect logs"}, Options{
		Header: "Choose checks",
		Height: 5,
		Limit:  2,
	})

	view := ansi.Strip(model.View())
	if !strings.Contains(view, "> ○ Check pods") {
		t.Fatalf("expected current multi-select row to include unselected circle, got %q", view)
	}
}
