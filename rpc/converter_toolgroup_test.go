package rpc

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// TestConvertCommandPropagatesToolGroup verifies the converter copies the
// clicky/tool-group annotation onto RPCOperation.Group and Clicky.Group, and
// that it does NOT leak into the auto-derived OpenAPI Tags.
func TestConvertCommandPropagatesToolGroup(t *testing.T) {
	strict := "true"
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List invoices",
		RunE:  func(*cobra.Command, []string) error { return nil },
	}
	cmd.Annotations = map[string]string{
		"clicky/entity-name":             "invoice",
		"clicky/operation-verb":          "list",
		"clicky/tool-group":              "billing",
		"clicky/tool-parent":             "accounting",
		"clicky/tool-icon":               "receipt",
		"clicky/tool-read-only-hint":     "true",
		"clicky/tool-destructive-hint":   "false",
		"clicky/tool-default-permission": "ask",
		"clicky/tool-strict":             strict,
	}

	op, err := NewConverter(DefaultConfig()).ConvertCommand(cmd)
	assert.NoError(t, err)
	assert.Equal(t, "billing", op.Group, "RPCOperation.Group")
	assert.Equal(t, "billing", op.ToolHints.Group, "RPCOperation.ToolHints.Group")
	assert.Equal(t, "accounting", op.ToolHints.Parent, "RPCOperation.ToolHints.Parent")
	assert.Equal(t, "receipt", op.ToolHints.Icon, "RPCOperation.ToolHints.Icon")
	assert.Equal(t, "ask", string(op.ToolHints.DefaultPermission), "RPCOperation.ToolHints.DefaultPermission")
	if assert.NotNil(t, op.ToolHints.ReadOnlyHint) {
		assert.True(t, *op.ToolHints.ReadOnlyHint)
	}
	if assert.NotNil(t, op.ToolHints.DestructiveHint) {
		assert.False(t, *op.ToolHints.DestructiveHint)
	}
	if assert.NotNil(t, op.ToolHints.Strict) {
		assert.True(t, *op.ToolHints.Strict)
	}
	if assert.NotNil(t, op.Clicky) {
		assert.Equal(t, "billing", op.Clicky.Group, "ClickyOperationMeta.Group")
		assert.Equal(t, op.ToolHints, op.Clicky.ToolHints, "ClickyOperationMeta.ToolHints")
		assert.Equal(t, &op.ToolHints, op.Clicky.OpenAPIToolHints, "ClickyOperationMeta.OpenAPIToolHints")
	}
	assert.NotContains(t, op.Tags, "billing", "tool-group must not leak into OpenAPI Tags")
}

// TestConvertCommandNoToolGroup verifies an operation without the annotation has
// an empty Group.
func TestConvertCommandNoToolGroup(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "list",
		RunE: func(*cobra.Command, []string) error { return nil },
	}
	cmd.Annotations = map[string]string{
		"clicky/entity-name":    "invoice",
		"clicky/operation-verb": "list",
	}

	op, err := NewConverter(DefaultConfig()).ConvertCommand(cmd)
	assert.NoError(t, err)
	assert.Equal(t, "", op.Group)
}
