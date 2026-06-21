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
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List invoices",
		RunE:  func(*cobra.Command, []string) error { return nil },
	}
	cmd.Annotations = map[string]string{
		"clicky/entity-name":    "invoice",
		"clicky/operation-verb": "list",
		"clicky/tool-group":     "billing",
	}

	op, err := NewConverter(DefaultConfig()).ConvertCommand(cmd)
	assert.NoError(t, err)
	assert.Equal(t, "billing", op.Group, "RPCOperation.Group")
	if assert.NotNil(t, op.Clicky) {
		assert.Equal(t, "billing", op.Clicky.Group, "ClickyOperationMeta.Group")
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
