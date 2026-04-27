package api

import (
	"strings"
	"testing"
)

func TestLabelBadge_String(t *testing.T) {
	cases := []struct {
		name string
		b    LabelBadge
		want string
	}{
		{"label and value", LabelBadge{Label: "env", Value: "prod"}, "env: prod"},
		{"only value", LabelBadge{Value: "prod"}, "prod"},
		{"only label", LabelBadge{Label: "env"}, "env"},
		{"empty", LabelBadge{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.b.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLabelBadge_Markdown(t *testing.T) {
	b := LabelBadge{Label: "env", Value: "prod"}
	if got, want := b.Markdown(), "**env**: prod"; got != want {
		t.Errorf("Markdown() = %q, want %q", got, want)
	}
	if got, want := (LabelBadge{Value: "prod"}).Markdown(), "prod"; got != want {
		t.Errorf("Markdown(value-only) = %q, want %q", got, want)
	}
}

func TestLabelBadge_HTML_EscapesUserInput(t *testing.T) {
	b := LabelBadge{Label: "<script>", Value: `"bad"`}
	got := b.HTML()
	if strings.Contains(got, "<script>") {
		t.Errorf("HTML() did not escape label: %q", got)
	}
	if strings.Contains(got, `"bad"`) {
		t.Errorf("HTML() did not escape value: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("HTML() missing escaped label: %q", got)
	}
}

func TestLabelBadge_HTML_UsesColorClasses(t *testing.T) {
	b := LabelBadge{Label: "k", Value: "v", Color: "bg-blue-100", TextColor: "text-slate-700"}
	got := b.HTML()
	for _, want := range []string{"bg-blue-100", "text-slate-700"} {
		if !strings.Contains(got, want) {
			t.Errorf("HTML() missing class %q in %q", want, got)
		}
	}
}

func TestLabelBadge_ImplementsTextable(t *testing.T) {
	var _ Textable = LabelBadge{}
}
