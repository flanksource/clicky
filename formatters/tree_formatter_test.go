package formatters

import (
	"regexp"
	"strings"
	"testing"

	"github.com/flanksource/clicky/api"
)

func TestNormalizeTreeLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"single line", "hello", "hello"},
		{"already clean multiline", "a\nb\nc", "a\nb\nc"},
		{"leading blank", "\nhello", "hello"},
		{"trailing blank", "hello\n", "hello"},
		{"leading and trailing blank", "\n\nhello\n\n", "hello"},
		{"doubled internal blank dropped", "a\n\n\nb", "a\nb"},
		{"single internal blank dropped", "a\n\nb", "a\nb"},
		{"trailing spaces trimmed", "a   \nb\t", "a\nb"},
		{"line of only spaces dropped", "a\n   \nb", "a\nb"},
		{"leading line of only spaces dropped", "   \na", "a"},
		{"mixed: trailing space + blanks all dropped", "root  \n\n\nchild line\n  \n\ntail  ", "root\nchild line\ntail"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTreeLabel(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeTreeLabel(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeTreeLabelIdempotent(t *testing.T) {
	in := "root\n\nchild line\n\ntail"
	once := normalizeTreeLabel(in)
	twice := normalizeTreeLabel(once)
	if once != twice {
		t.Errorf("normalizeTreeLabel not idempotent: once=%q twice=%q", once, twice)
	}
	if strings.Contains(once, "\n\n") {
		t.Errorf("normalizeTreeLabel left an internal blank line: %q", once)
	}
}

// blankGutterLine matches a rendered tree line carrying only tree-connector
// glyphs (│ ├ ╰ └ ─) and whitespace — a line where the node label contributed
// no visible content. lipgloss right-pads lines, so trailing space is ignored.
var blankGutterLine = regexp.MustCompile(`^[\s│├╰└─]*$`)

// TestFormatTreeDropsBlankGutterLines renders a tree whose first (non-last)
// child carries a multi-line label with leading, trailing, and internal blank
// lines plus trailing whitespace. A non-last child uses the "│" continuation
// gutter, so any surviving blank physical line shows up as a "│"-only line.
// Policy: every blank line in a node label is dropped, so the rendered tree has
// no blank gutter lines at all; content lines stay in order.
func TestFormatTreeDropsBlankGutterLines(t *testing.T) {
	child := &api.SimpleTreeNode{
		Label: "\nroot  \n\n\nchild line\n  \n\ntail  \n",
	}
	sibling := &api.SimpleTreeNode{Label: "sibling"}
	parent := &api.SimpleTreeNode{
		Label:    "parent",
		Children: []api.TreeNode{child, sibling},
	}

	f := NewTreeFormatter(api.DefaultTheme(), true, nil)
	out := f.FormatTreeFromRoot(parent)
	lines := strings.Split(out, "\n")

	for i, line := range lines {
		if blankGutterLine.MatchString(line) {
			t.Errorf("blank gutter line at index %d (should be dropped):\n%s", i, out)
		}
	}

	// The first child's label must sit on the branch line — leading blank gone.
	if rootLine := lines[1]; !strings.Contains(rootLine, "root") {
		t.Errorf("expected first child line to carry 'root', got %q in:\n%s", rootLine, out)
	}

	for _, want := range []string{"parent", "root", "child line", "tail", "sibling"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	rootIdx := strings.Index(out, "root")
	childIdx := strings.Index(out, "child line")
	tailIdx := strings.Index(out, "tail")
	if !(rootIdx < childIdx && childIdx < tailIdx) {
		t.Errorf("content lines out of order: root=%d child=%d tail=%d in:\n%s", rootIdx, childIdx, tailIdx, out)
	}
}
