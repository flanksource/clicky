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

	wantSections := []string{
		"Logging flags:",
		"Tasks flags:",
		"Format flags:",
	}
	for _, s := range wantSections {
		if !strings.Contains(out, s) {
			t.Errorf("expected section %q in help output, got:\n%s", s, out)
		}
	}

	cases := []struct {
		section, flag string
	}{
		{"Logging flags:", "--log-level"},
		{"Logging flags:", "--json-logs"},
		{"Tasks flags:", "--max-concurrent"},
		{"Tasks flags:", "--no-progress"},
		{"Format flags:", "--format"},
		{"Format flags:", "--json"},
	}
	for _, tc := range cases {
		section := flagSection(out, tc.section)
		if !strings.Contains(section, tc.flag) {
			t.Errorf("expected %q under %q section; section was:\n%s", tc.flag, tc.section, section)
		}
	}
}

// flagSection returns the lines belonging to heading. It stops at the next
// flags heading instead of relying on blank-line separators, so the assertions
// fail if flags slide into a later group.
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
		if strings.HasSuffix(trimmed, "flags:") {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
