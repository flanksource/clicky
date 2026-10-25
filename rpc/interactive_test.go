package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/flags"
	"github.com/flanksource/clicky/prompt"
)

type deployRun struct {
	args     []string
	env      string
	replicas int
	tags     []string
}

// newDeployTree builds `app deploy <name> --env --replicas --tag` and records
// what the command's RunE receives.
func newDeployTree(got *deployRun) *cobra.Command {
	root := &cobra.Command{Use: "app"}
	deploy := &cobra.Command{
		Use:   "deploy <name>",
		Short: "Deploy a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			got.args = args
			got.env, _ = cmd.Flags().GetString("env")
			got.replicas, _ = cmd.Flags().GetInt("replicas")
			got.tags, _ = cmd.Flags().GetStringSlice("tag")
			return nil
		},
	}
	deploy.Flags().String("env", "", "Target environment")
	flags.SetAllowedValues(deploy.Flags().Lookup("env"), "dev", "prod")
	deploy.Flags().Int("replicas", 1, "Replica count")
	_ = deploy.MarkFlagRequired("replicas")
	deploy.Flags().StringSlice("tag", nil, "Release tags")
	root.AddCommand(deploy)
	return root
}

func TestOperationInputSchemaMirrorsTheOpenAPIParameters(t *testing.T) {
	root := newDeployTree(&deployRun{})
	op, err := NewConverter(DefaultConfig()).ConvertCommand(root.Commands()[0])
	require.NoError(t, err)

	schema, err := OperationInputSchema(*op, OperationSchemaOptions{})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"type": "object",
		"title": "deploy",
		"x-order": ["name", "env", "replicas", "tag"],
		"required": ["name", "replicas"],
		"properties": {
			"name": {"type": "string", "description": "Positional argument from command"},
			"env": {"type": "string", "description": "Target environment", "enum": ["dev", "prod"]},
			"replicas": {"type": "integer", "format": "int32", "description": "Replica count", "default": 1},
			"tag": {"type": "array", "description": "Release tags", "items": {"type": "string"}}
		}
	}`, string(schema))
}

// Every query param of a lookup-supporting list op has the filter role, but
// only the filters the lookup serves can be resolved, so only they get a
// lookup — the same keys the web FilterBar reads from `?__lookup=filters`.
func TestOperationInputSchemaLooksUpOnlyServedFilters(t *testing.T) {
	op := RPCOperation{
		Name:       "stack list",
		Parameters: []RPCParameter{{Name: "team", Type: "string", In: "query"}, {Name: "region", Type: "string", In: "query"}},
		Clicky:     &ClickyOperationMeta{Verb: "list", SupportsLookup: true},
		LookupFunc: func(map[string]string, []string) (any, error) {
			return entity.LookupResponse{Filters: map[string]entity.LookupFilter{"team": {}}}, nil
		},
	}
	served, err := OperationLookupFilters(context.Background(), op)
	require.NoError(t, err)
	require.Equal(t, []string{"team"}, served)

	raw, err := OperationInputSchema(op, OperationSchemaOptions{LookupFilters: served})
	require.NoError(t, err)
	var schema struct {
		Properties map[string]map[string]any `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))
	assert.Contains(t, schema.Properties["team"], "x-clicky-lookup")
	assert.NotContains(t, schema.Properties["region"], "x-clicky-lookup")
}

// The interactive run goes through the prompt sink (a scoped context prefers
// it), which is the dashboard path: the form arrives pre-filled with the flags
// given on the command line, and the answer drives flags, args and the
// required-flag check.
func TestInteractiveFillsFlagsAndArgsBeforeRunning(t *testing.T) {
	m := prompt.NewManager(prompt.NewMemory(prompt.MemoryConfig{}))
	prompt.SetDefault(m)
	defer prompt.SetDefault(nil)

	got := &deployRun{}
	root := newDeployTree(got)
	InstallInteractive(root)
	root.SetArgs([]string{"deploy", "-i", "--env", "dev"})
	ctx := prompt.WithScope(context.Background(), prompt.Scope{Owner: "interactive-test"})
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(ctx) }()

	snap := waitForForm(t, m, "interactive-test")
	// Schema defaults are not part of the pre-fill: JsonSchemaForm (like the
	// terminal form) applies them itself.
	assert.Equal(t, map[string]any{"env": "dev"}, snap.Value, "the form must start from the flags given")
	var form struct {
		Order []string `json:"x-order"`
	}
	require.NoError(t, json.Unmarshal(snap.Schema, &form))
	assert.Equal(t, []string{"name", "env", "replicas", "tag"}, form.Order,
		"cobra's own --help (added at execution) is not an operation argument")
	require.NoError(t, m.Resolve(snap.ID, prompt.Answer{Values: map[string]any{
		"name":     "checkout",
		"env":      "prod",
		"replicas": float64(3),
		"tag":      []any{"v2", "canary"},
	}}))

	require.NoError(t, <-done)
	assert.Equal(t, &deployRun{args: []string{"checkout"}, env: "prod", replicas: 3, tags: []string{"v2", "canary"}}, got)
}

func TestInteractiveCancelledFormFailsTheCommand(t *testing.T) {
	m := prompt.NewManager(prompt.NewMemory(prompt.MemoryConfig{}))
	prompt.SetDefault(m)
	defer prompt.SetDefault(nil)

	got := &deployRun{}
	root := newDeployTree(got)
	InstallInteractive(root)
	root.SilenceErrors, root.SilenceUsage = true, true
	root.SetArgs([]string{"deploy", "--interactive"})
	ctx := prompt.WithScope(context.Background(), prompt.Scope{Owner: "interactive-cancel"})
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(ctx) }()

	snap := waitForForm(t, m, "interactive-cancel")
	require.NoError(t, m.Resolve(snap.ID, prompt.Answer{Cancelled: true}))
	assert.ErrorContains(t, <-done, "prompt cancelled")
	assert.Nil(t, got.args, "a cancelled form must not run the command")
}

func TestInstallInteractiveLeavesACommandsOwnInteractiveFlag(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	var own bool
	child := &cobra.Command{Use: "init", RunE: func(cmd *cobra.Command, _ []string) error {
		own, _ = cmd.Flags().GetBool("interactive")
		return nil
	}}
	child.Flags().BoolP("interactive", "i", false, "Ask setup questions")
	root.AddCommand(child)

	InstallInteractive(root)
	assert.Equal(t, "i", root.PersistentFlags().Lookup("interactive").Shorthand,
		"init's own --interactive shadows the global one, so -i stays available everywhere")
	root.SetArgs([]string{"init", "-i"})
	require.NoError(t, root.Execute())
	assert.True(t, own, "init must see its own --interactive, not the form")
}

func TestInstallInteractiveDropsTheShorthandWhenADifferentFlagOwnsIt(t *testing.T) {
	root := &cobra.Command{Use: "app"}
	child := &cobra.Command{Use: "convert", RunE: func(*cobra.Command, []string) error { return nil }}
	child.Flags().StringP("input", "i", "", "Input file")
	root.AddCommand(child)

	InstallInteractive(root)
	assert.Empty(t, root.PersistentFlags().Lookup("interactive").Shorthand,
		"-i means --input on convert, so the global flag is long-form only")
	root.SetArgs([]string{"convert", "-i", "in.json"})
	require.NoError(t, root.Execute(), "convert's -i must still parse as --input")
}

func waitForForm(t *testing.T, m *prompt.Manager, owner string) prompt.PromptSnapshot {
	t.Helper()
	for i := 0; i < 500; i++ {
		if ps := m.List(prompt.Filter{Owner: owner, Kind: "form", State: "pending"}); len(ps) == 1 {
			return ps[0]
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("no pending form prompt for %s", owner)
	return prompt.PromptSnapshot{}
}
