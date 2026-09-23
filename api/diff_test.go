package api

import (
	"strings"
	"testing"
)

func TestDiffIsEmpty(t *testing.T) {
	d := NewDiff("same", "same", "a", "b")
	if !d.IsEmpty() {
		t.Fatalf("identical inputs should be empty")
	}
	if d.Unified() != "" {
		t.Fatalf("Unified() must be empty for identical inputs, got %q", d.Unified())
	}
	if d.String() != "" || d.ANSI() != "" || d.HTML() != "" || d.Markdown() != "" {
		t.Fatalf("all renderers must be empty for identical inputs")
	}
}

func TestDiffUnifiedHasHeadersAndChanges(t *testing.T) {
	before := "alpha\nbravo\ncharlie\n"
	after := "alpha\nBRAVO\ncharlie\n"
	d := NewDiff(before, after, "before.txt", "after.txt")

	if d.IsEmpty() {
		t.Fatalf("differing inputs must not be empty")
	}

	got := d.Unified()
	for _, want := range []string{"--- before.txt", "+++ after.txt", "-bravo", "+BRAVO", "@@"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Unified() missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestColorizeUnifiedDiffStylesEachLineKind(t *testing.T) {
	raw := "--- x\n+++ y\n@@ -1 +1 @@\n-a\n+b\n same\n"
	want := "\x1b[1m--- x\x1b[0m\n\x1b[1m+++ y\x1b[0m\n\x1b[36m@@ -1 +1 @@\x1b[0m\n" +
		"\x1b[31m-a\x1b[0m\n\x1b[32m+b\x1b[0m\n same\n"
	if got := colorizeUnifiedDiff(raw); got != want {
		t.Fatalf("colorizeUnifiedDiff() = %q, want %q", got, want)
	}
}

func TestDiffANSIHonorsNoColorEnv(t *testing.T) {
	for name, env := range map[string][2]string{
		"NO_COLOR set": {"NO_COLOR", "1"},
		"TERM=dumb":    {"TERM", "dumb"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(env[0], env[1])
			if ansiColorEnabled() {
				t.Fatalf("ansiColorEnabled() must be false with %s=%s", env[0], env[1])
			}
			d := NewDiff("a\n", "b\n", "x", "y")
			if got := d.ANSI(); got != d.Unified() {
				t.Fatalf("ANSI() must equal Unified() with %s=%s, got %q", env[0], env[1], got)
			}
		})
	}
}

func TestDiffHTMLWrapsWithSpans(t *testing.T) {
	d := NewDiff("alpha\nbravo\n", "alpha\nBRAVO\n", "before", "after")
	got := d.HTML()
	for _, want := range []string{
		`<pre class="diff">`,
		`class="diff-meta"`,
		`class="diff-hunk"`,
		`class="diff-add"`,
		`class="diff-remove"`,
		`-bravo`,
		`+BRAVO`,
		`</pre>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("HTML() missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestDiffHTMLEscapesContent(t *testing.T) {
	d := NewDiff("<a>\n", "<b>\n", "from", "to")
	got := d.HTML()
	if strings.Contains(got, "<a>") || strings.Contains(got, "<b>") {
		t.Fatalf("HTML() must escape angle brackets in content, got %s", got)
	}
	if !strings.Contains(got, "&lt;a&gt;") || !strings.Contains(got, "&lt;b&gt;") {
		t.Fatalf("HTML() must contain escaped entities, got %s", got)
	}
}

func TestDiffMarkdownIsFencedDiffBlock(t *testing.T) {
	d := NewDiff("alpha\n", "beta\n", "from", "to")
	got := d.Markdown()
	if !strings.HasPrefix(got, "```diff\n") {
		t.Fatalf("Markdown() must start with ```diff fence, got %q", got)
	}
	if !strings.HasSuffix(got, "\n```") {
		t.Fatalf("Markdown() must end with closing fence, got %q", got)
	}
	if !strings.Contains(got, "-alpha") || !strings.Contains(got, "+beta") {
		t.Fatalf("Markdown() missing diff lines, got %s", got)
	}
}

func TestDiffContextDefaults(t *testing.T) {
	d := Diff{Before: "a\n", After: "b\n"} // Context: 0
	if d.Unified() == "" {
		t.Fatalf("zero Context should still produce a diff (defaulting to 3)")
	}
}

// Regression: Diff must satisfy Textable so it can be embedded in api.Text.
func TestDiffSatisfiesTextable(t *testing.T) {
	var _ Textable = Diff{}
}
