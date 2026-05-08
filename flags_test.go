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
		section := sectionAfter(out, tc.section)
		if !strings.Contains(section, tc.flag) {
			t.Errorf("expected %q under %q section; section was:\n%s", tc.flag, tc.section, section)
		}
	}
}

// sectionAfter returns the substring of help output starting at heading and
// ending at the next blank line, used to assert flags land in the right group.
func sectionAfter(help, heading string) string {
	idx := strings.Index(help, heading)
	if idx < 0 {
		return ""
	}
	rest := help[idx:]
	end := strings.Index(rest, "\n\n")
	if end < 0 {
		return rest
	}
	return rest[:end]
}
