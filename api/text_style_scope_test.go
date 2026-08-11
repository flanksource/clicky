package api

import (
	"strings"
	"testing"
)

const sgrReset = "\x1b[0m"

// A node's style must cover its own content only. Folding children into the
// styled content opened the node's SGR before them and closed it after the
// last one, so a `font-bold` section header rendered its entire subtree bold —
// and the first child emitting its own reset silently cancelled the header's
// attribute for everything that followed.
func TestANSIStyleDoesNotLeakOntoChildren(t *testing.T) {
	header := Text{
		Content:  "Workflows:",
		Style:    "font-bold",
		Children: []Textable{Text{Content: "\n  child", Style: "text-gray-500"}},
	}

	got := header.ANSI()

	styled, _, ok := strings.Cut(got, sgrReset)
	if !ok {
		t.Fatalf("ANSI() emitted no reset sequence: %q", got)
	}
	if !strings.Contains(styled, "Workflows:") {
		t.Errorf("header content is not inside the first styled span: %q", got)
	}
	if strings.Contains(styled, "child") {
		t.Errorf("child content is inside the header's styled span, so the header style leaks: %q", got)
	}
}

// The child keeps its own styling; scoping the parent must not strip it.
func TestANSIChildKeepsItsOwnStyle(t *testing.T) {
	parent := Text{
		Content:  "head",
		Style:    "font-bold",
		Children: []Textable{Text{Content: "tail", Style: "text-gray-500"}},
	}

	got := parent.ANSI()
	want := formatANSI("head", styleOf("font-bold")) + formatANSI("tail", styleOf("text-gray-500"))
	if got != want {
		t.Errorf("ANSI() = %q, want each node styled independently: %q", got, want)
	}
}

func styleOf(class string) TailwindStyle {
	_, style := ApplyTailwindStyle("", class)
	return style
}
