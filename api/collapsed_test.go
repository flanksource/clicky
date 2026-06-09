package api

import (
	"strings"
	"testing"
)

// TestCollapsedANSI_ExpandsByDefault: a plain Collapsed renders its content
// inline in ANSI (terminals can't toggle), so the body is fully visible.
func TestCollapsedANSI_ExpandsByDefault(t *testing.T) {
	c := Collapsed{Label: "stderr", Content: Text{}.Append("boom\non two lines")}

	got := c.ANSI()
	if !strings.Contains(got, "boom") || !strings.Contains(got, "on two lines") {
		t.Fatalf("default Collapsed must expand content in ANSI, got %q", got)
	}
	if strings.Contains(got, "▶") {
		t.Fatalf("default Collapsed must not show a collapsed header in ANSI, got %q", got)
	}
}

// TestCollapsedANSI_CollapsedHidesContent: with CollapseANSI set, ANSI shows only
// a "▶ <label>" header and hides the bulky content — used to keep a raw-JSON dump
// out of a terminal failure trace while leaving it expandable in HTML.
func TestCollapsedANSI_CollapsedHidesContent(t *testing.T) {
	c := Collapsed{
		Label:        "activity-trace.json",
		Content:      Text{}.Append(`{"kind":"activity"}`),
		CollapseANSI: true,
	}

	got := c.ANSI()
	if got != "▶ activity-trace.json" {
		t.Fatalf("CollapseANSI must render a label-only header, got %q", got)
	}
	if strings.Contains(got, "activity") && strings.Contains(got, "{") {
		t.Fatalf("CollapseANSI must hide the JSON content in ANSI, got %q", got)
	}
}

// TestCollapsedHTML_StaysInteractiveWhenCollapseANSI: CollapseANSI is ANSI-only;
// HTML keeps the toggle and embeds the content regardless of the flag.
func TestCollapsedHTML_StaysInteractiveWhenCollapseANSI(t *testing.T) {
	c := Collapsed{
		Label:        "activity-trace.json",
		Content:      Text{}.Append(`{"kind":"activity"}`),
		CollapseANSI: true,
	}

	html := c.HTML()
	if !strings.Contains(html, "activity-trace.json") {
		t.Fatalf("HTML must still render the label, got %q", html)
	}
	if !strings.Contains(html, `x-if="open"`) {
		t.Fatalf("HTML must keep the interactive toggle, got %q", html)
	}
}
