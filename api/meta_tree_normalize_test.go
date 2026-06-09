package api

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestNormalizeTreeLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single line", "hello", "hello"},
		{"clean multiline", "a\nb\nc", "a\nb\nc"},
		{"leading blank", "\nhello", "hello"},
		{"trailing blank", "hello\n", "hello"},
		{"internal blank dropped", "a\n\nb", "a\nb"},
		{"doubled internal blank dropped", "a\n\n\nb", "a\nb"},
		{"trailing spaces trimmed", "a   \nb\t", "a\nb"},
		{"line of only spaces dropped", "a\n   \nb", "a\nb"},
		{"ansi-only line dropped", "a\n\x1b[1;38;2;8;145;178m\x1b[0m\nb", "a\nb"},
		{"ansi content kept", "\x1b[1mTitle\x1b[0m\nbody", "\x1b[1mTitle\x1b[0m\nbody"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeTreeLabel(tt.input); got != tt.expected {
				t.Errorf("normalizeTreeLabel(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// treeNodeStub returns a Pretty() whose first node is short but whose children
// carry multi-line content with a deliberate blank-line separator — mirroring a
// gavel Test node wrapping an activity-trace detail.
type treeNodeStub struct{}

func (treeNodeStub) Pretty() Text {
	return Text{Content: "✗ trace"}.
		NewLine().Add(Text{Content: "Scheme: GL Scheme"}).
		NewLine().NewLine().Add(Text{Content: "Activity 123 Foo"})
}
func (treeNodeStub) GetChildren() []TreeNode { return nil }

var gutterOnly = regexp.MustCompile(`^[\s│├╰└─]*$`)

// TestTextTreeRender_NoBlankGutterLines is the regression guard for stray
// "│"-gutter lines: a multi-line node label with an internal blank separator
// must not produce a bare gutter line in either the plain or ANSI tree render.
func TestTextTreeRender_NoBlankGutterLines(t *testing.T) {
	root := &SimpleTreeNode{
		Label: "suite",
		Children: []TreeNode{
			treeNodeStub{},
			&SimpleTreeNode{Label: "sibling"},
		},
	}
	tree := NewTree[TreeNode](root)

	for _, tc := range []struct {
		name string
		out  string
	}{
		{"String", tree.String()},
		{"ANSI", tree.ANSI()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i, line := range strings.Split(tc.out, "\n") {
				if gutterOnly.MatchString(ansi.Strip(line)) {
					t.Errorf("blank gutter line at index %d:\n%s", i, tc.out)
				}
			}
			for _, want := range []string{"trace", "Scheme: GL Scheme", "Activity 123 Foo", "sibling"} {
				if !strings.Contains(tc.out, want) {
					t.Errorf("expected %q in:\n%s", want, tc.out)
				}
			}
		})
	}
}
