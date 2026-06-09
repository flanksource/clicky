package rpc

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestPositionalOperandCount(t *testing.T) {
	cases := []struct {
		use  string
		want int
	}{
		{"diff", 0},
		{"recalculate <id>", 1},
		{"get [policyNumber]", 1},
		{"diff <a> <b>", 2},
		{"transfer <from> <to>", 2},
		{"bulk-suspend <id> [id...]", 1}, // variadic [id...] is not an operand
		{"set key=value", 0},             // body-style token, not an operand
		{"records <id> [flags]", 1},      // [flags] placeholder is not an operand
		{"diff <a> <b> [flags]", 2},      // flags placeholder excluded, two operands remain
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, positionalOperandCount(c.use),
			"positionalOperandCount(%q)", c.use)
	}
}

// restPathFor builds a parent>child tree and returns the generated REST path
// for the child, mirroring how AddNamedCommand-registered actions nest under an
// entity command beneath root.
func restPathFor(parentUse, childUse string) string {
	root := &cobra.Command{Use: "app"}
	parent := &cobra.Command{Use: parentUse}
	child := &cobra.Command{Use: childUse, RunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(parent)
	parent.AddCommand(child)

	c := NewConverter(DefaultConfig())
	return c.generateRESTPath(child, getCommandPath(child))
}

// TestGenerateRESTPath_SingleIdActionLiftsToIDSegment locks in the existing
// behaviour: a one-operand action restructures to /entity/{id}/action.
func TestGenerateRESTPath_SingleIdActionLiftsToIDSegment(t *testing.T) {
	assert.Equal(t, "/api/v1/policy/{id}/recalculate",
		restPathFor("policy", "recalculate <id>"))
	// The entity-action builder renders Use as "<verb> <id> [flags]" when the
	// action carries flags; [flags] must not be counted as a second operand.
	assert.Equal(t, "/api/v1/intake/{id}/records",
		restPathFor("intake", "records <id> [flags]"))
}

// TestGenerateRESTPath_TwoOperandActionStaysFlat is the regression guard for the
// scheme/policy diff bug: a two-operand `diff <a> <b>` must stay flat at
// /entity/diff and not lift the first operand into /entity/{a}/diff.
func TestGenerateRESTPath_TwoOperandActionStaysFlat(t *testing.T) {
	assert.Equal(t, "/api/v1/scheme/diff", restPathFor("scheme", "diff <a> <b>"))
	assert.Equal(t, "/api/v1/policy/diff", restPathFor("policy", "diff <a> <b>"))
}
