package rpc

import (
	"testing"

	"github.com/flanksource/clicky"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertCommand_OmitsInheritedAndHiddenFlags pins that a plain (non-entity)
// command's tool schema carries ONLY its own operation flags — never the
// inherited persistent globals that cobra folds into Flags() (format/log-level/
// config/company/entity/limit/...), and never a flag explicitly marked hidden
// (the mechanism xero-cli uses to keep db-url off tool/OpenAPI schemas). A
// regression here reintroduces the "wall of global-flag chips" on every tool.
func TestConvertCommand_OmitsInheritedAndHiddenFlags(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	// Mirror a real CLI root: format/logging globals plus a couple of app
	// globals live on the root's persistent flag set, inherited by every child.
	clicky.BindAllFlags(root.PersistentFlags(), "format")
	root.PersistentFlags().String("config", "", "Config file path")
	root.PersistentFlags().String("company", "", "Select company")
	root.PersistentFlags().String("entity", "", "Select entity")
	root.PersistentFlags().Int("limit", 0, "Max rows")

	child := &cobra.Command{
		Use:   "do-thing",
		Short: "Do a thing",
		RunE:  func(*cobra.Command, []string) error { return nil },
	}
	// Operation-specific flags: one visible, one hidden (like db-url).
	child.Flags().String("name", "", "Name to act on")
	child.Flags().Int("count", 0, "How many")
	child.Flags().String("db-url", "", "Infra DSN")
	require.NoError(t, child.Flags().MarkHidden("db-url"))
	root.AddCommand(child)

	op, err := NewConverter(DefaultConfig()).ConvertCommand(child)
	require.NoError(t, err)

	params := map[string]bool{}
	for _, p := range op.Parameters {
		params[p.Name] = true
	}

	assert.True(t, params["name"], "operation-specific flag `name` must be a parameter")
	assert.True(t, params["count"], "operation-specific flag `count` must be a parameter")

	for _, omitted := range []string{
		"format", "no-color", "json", "yaml", "csv", "html", "pdf", "markdown",
		"pretty", "tree", "table", "filter", "dump-schema", "loglevel",
		"log-level", "json-logs", "config", "company", "entity", "limit",
	} {
		assert.False(t, params[omitted],
			"inherited global flag %q must NOT leak into the tool schema", omitted)
		assert.NotContains(t, op.Schema.Properties, omitted,
			"inherited global flag %q must NOT leak into the schema properties", omitted)
	}

	assert.False(t, params["db-url"], "hidden flag db-url must NOT appear as a parameter")
	assert.NotContains(t, op.Schema.Properties, "db-url",
		"hidden flag db-url must NOT appear in schema properties")
}
