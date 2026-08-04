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

// pflag stringifies an empty slice default as "[]". Publishing that as a schema
// default pre-fills generated forms with the literal characters "[]", which a
// client then posts back as a one-element slice — turning "no values" into one
// bogus value. A repeatable flag with no default must advertise none.
func TestConvertCommand_OmitsEmptySliceDefaults(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	child := &cobra.Command{
		Use:   "act",
		Short: "Act",
		RunE:  func(*cobra.Command, []string) error { return nil },
	}
	child.Flags().StringSlice("param", nil, "Repeatable key=value")
	child.Flags().StringArray("header", nil, "Repeatable header")
	child.Flags().StringSlice("tag", []string{"a", "b"}, "Repeatable tag with defaults")
	child.Flags().String("name", "fallback", "Scalar with a default")
	root.AddCommand(child)

	op, err := NewConverter(DefaultConfig()).ConvertCommand(child)
	require.NoError(t, err)

	defaults := map[string]any{}
	for _, p := range op.Parameters {
		defaults[p.Name] = p.Default
	}

	assert.Nil(t, defaults["param"], "an empty slice flag must advertise no default")
	assert.Nil(t, defaults["header"], "an empty array flag must advertise no default")
	assert.Nil(t, op.Schema.Properties["param"].Default,
		"the schema property must not carry the empty-slice default either")

	// A slice flag with real defaults, and every scalar flag, keep theirs.
	assert.Equal(t, "[a,b]", defaults["tag"])
	assert.Equal(t, "fallback", defaults["name"])
}
