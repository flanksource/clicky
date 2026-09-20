package rpc

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/flanksource/clicky/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// restPathForAnnotated is restPathFor with annotations applied to the child, so
// a test can exercise what a registration declared rather than what the command
// tree implies.
func restPathForAnnotated(parentUse, childUse string, annotations map[string]string) string {
	root := &cobra.Command{Use: "app"}
	parent := &cobra.Command{Use: parentUse}
	child := &cobra.Command{Use: childUse, Annotations: annotations,
		RunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(parent)
	parent.AddCommand(child)

	c := NewConverter(DefaultConfig())
	return c.generateRESTPath(child, getCommandPath(child))
}

// An operation may have to live at a path the naming algorithm would never
// produce — a URL a frontend already calls, or one an older release published.
// Declaring it is what lets such a route become an entity operation without
// breaking its callers.
func TestDeclaredOperationPathWins(t *testing.T) {
	got := restPathForAnnotated("cycle-run", "cancel <id>", map[string]string{
		"clicky/operation-path": "/api/v1/cycle-runs/{id}/levels/{level}/cancel",
	})
	assert.Equal(t, "/api/v1/cycle-runs/{id}/levels/{level}/cancel", got)
}

// A declared path without a leading slash is taken as relative to the prefix,
// so a caller does not have to repeat it.
func TestDeclaredRelativePathJoinsThePrefix(t *testing.T) {
	got := restPathForAnnotated("monitor", "stop <id>", map[string]string{
		"clicky/operation-path": "monitors/{id}/stop",
	})
	assert.Equal(t, "/api/v1/monitors/{id}/stop", got)
}

// Declaring nothing must leave every existing route exactly where it was.
func TestUndeclaredPathIsStillDerived(t *testing.T) {
	assert.Equal(t, restPathFor("policy", "recalculate <id>"),
		restPathForAnnotated("policy", "recalculate <id>", nil))
}

// Served-only is only useful if the route survives: the whole point is to keep
// an operation on the API while taking it off the CLI.
func TestServedOnlyOperationStillBecomesARoute(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	group := &cobra.Command{Use: "monitors"}
	list := &cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(group)
	group.AddCommand(list)
	entity.MarkServedOnly(group)
	entity.RefuseServedOnlyCommands(root)

	service, err := NewConverter(DefaultConfig()).ConvertCommandTree(root)
	require.NoError(t, err)

	var found bool
	for _, op := range service.Operations {
		if op.Path == "/api/v1/monitors" {
			found = true
		}
	}
	assert.True(t, found, "a served-only operation must still be routed")
}

// Local-only remains the opposite: off the API, on the CLI.
func TestLocalOnlyOperationIsNotRouted(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	group := &cobra.Command{Use: "migrate"}
	run := &cobra.Command{Use: "run", RunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(group)
	group.AddCommand(run)
	entity.MarkLocalOnly(group)

	service, err := NewConverter(DefaultConfig()).ConvertCommandTree(root)
	require.NoError(t, err)

	for _, op := range service.Operations {
		assert.NotEqual(t, "/api/v1/migrate/run", op.Path, "a local-only operation must not be routed")
	}
}
