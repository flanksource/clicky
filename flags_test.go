package clicky

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestBindAllFlagsToCommand_GroupsHelpOutput(t *testing.T) {
	cmd := &cobra.Command{Use: "demo", Run: func(*cobra.Command, []string) {}}
	BindAllFlagsToCommand(cmd, "tasks", "format")

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute --help: %v", err)
	}

	out := buf.String()

	wantSections := []string{"Formatting:", "Logging:", "Tasks:"}
	for _, s := range wantSections {
		if !strings.Contains(out, s) {
			t.Errorf("expected section %q in help output, got:\n%s", s, out)
		}
	}

	cases := []struct {
		section, flag string
	}{
		{"Logging:", "--log-level"},
		{"Logging:", "--json-logs"},
		{"Tasks:", "--max-concurrent"},
		{"Tasks:", "--no-progress"},
		{"Formatting:", "--format"},
		{"Formatting:", "--json"},
	}
	for _, tc := range cases {
		section := flagSection(out, tc.section)
		if !strings.Contains(section, tc.flag) {
			t.Errorf("expected %q under %q section; section was:\n%s", tc.flag, tc.section, section)
		}
	}
}

// flagSection stops at the next unindented heading.
func flagSection(help, heading string) string {
	lines := strings.Split(help, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == heading {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}

	out := []string{lines[start]}
	for _, line := range lines[start+1:] {
		trimmed := strings.TrimSpace(line)
		if line == trimmed && strings.HasSuffix(trimmed, ":") {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// go test pipes stdout, so stdout formatting resolves uncoloured; the logger
// writes to stderr and must keep its colour.
func TestUseFlags_RedirectedStdoutKeepsLogColor(t *testing.T) {
	for _, env := range []string{"NO_COLOR", "COLOR", "CLICKY_FORCE_INTERACTIVE"} {
		t.Setenv(env, "")
	}
	t.Setenv("TERM", "xterm-256color")
	flags := AllFlags{}

	flags.UseFlags()

	if !flags.NoColor {
		t.Errorf("NoColor = false, want true: stdout is not a terminal")
	}
	if !flags.Color {
		t.Errorf("Color = false, want true: the logger writes to stderr, which a redirected stdout does not decolour")
	}
}
